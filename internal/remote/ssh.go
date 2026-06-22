package remote

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Sp0nge-bob/backupscript/internal/config"
)

type PathStatus struct {
	Path   string
	Exists bool
	IsDir  bool
}

const sshTimeout = 30 * time.Second

type SSHClient struct {
	ssh  *ssh.Client
	sftp *sftp.Client
}

func Connect(node config.NodeConfig) (*SSHClient, error) {
	keyData, err := os.ReadFile(node.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", node.KeyFile, err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse key %s: %w", node.KeyFile, err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            node.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshTimeout,
	}

	addr := net.JoinHostPort(node.Host, fmt.Sprintf("%d", node.SSHPort()))
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp client: %w", err)
	}

	return &SSHClient{ssh: sshClient, sftp: sftpClient}, nil
}

func (c *SSHClient) Close() error {
	if c.sftp != nil {
		_ = c.sftp.Close()
	}
	if c.ssh != nil {
		return c.ssh.Close()
	}
	return nil
}

func (c *SSHClient) InspectPaths(paths []string) []PathStatus {
	statuses := make([]PathStatus, 0, len(paths))
	for _, p := range paths {
		info, err := c.sftp.Stat(p)
		statuses = append(statuses, PathStatus{
			Path:   p,
			Exists: err == nil,
			IsDir:  err == nil && info.IsDir(),
		})
	}
	return statuses
}

func (c *SSHClient) AddToZip(zw *zip.Writer, paths, exclude []string, prefix, tmpDir string) (warnings []string, count int, err error) {
	for _, sourcePath := range paths {
		warn, added, addErr := c.addSource(zw, sourcePath, exclude, prefix, tmpDir)
		if addErr != nil {
			return warnings, count, addErr
		}
		if warn != "" {
			warnings = append(warnings, warn)
		}
		count += added
	}
	return warnings, count, nil
}

func (c *SSHClient) addSource(zw *zip.Writer, sourcePath string, exclude []string, prefix, tmpDir string) (warning string, count int, err error) {
	info, statErr := c.sftp.Stat(sourcePath)
	if statErr != nil {
		return fmt.Sprintf("missing: %s", sourcePath), 0, nil
	}

	if info.IsDir() {
		return c.addDirectory(zw, sourcePath, exclude, prefix, tmpDir)
	}

	if isSQLite(sourcePath) {
		return c.addSQLiteFile(zw, sourcePath, prefix, tmpDir)
	}

	entry := zipEntryWithPrefix(prefix, sourcePath)
	if err := c.addRemoteFile(zw, sourcePath, entry); err != nil {
		return "", 0, err
	}
	return "", 1, nil
}

func (c *SSHClient) addDirectory(zw *zip.Writer, root string, exclude []string, prefix, tmpDir string) (warning string, count int, err error) {
	walker := c.sftp.Walk(root)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return "", count, err
		}
		path := walker.Path()
		info := walker.Stat()
		if info.IsDir() {
			continue
		}
		if shouldExclude(filepath.Base(path), exclude) {
			continue
		}
		if isSQLite(path) {
			warn, added, err := c.addSQLiteFile(zw, path, prefix, tmpDir)
			if err != nil {
				return warning, count, err
			}
			if warn != "" {
				warning = warn
			}
			count += added
			continue
		}
		entry := zipEntryWithPrefix(prefix, path)
		if err := c.addRemoteFile(zw, path, entry); err != nil {
			return warning, count, err
		}
		count++
	}
	return warning, count, nil
}

func (c *SSHClient) addSQLiteFile(zw *zip.Writer, sourcePath, prefix, tmpDir string) (warning string, count int, err error) {
	base := filepath.Base(sourcePath)
	tmpCopy := filepath.Join(tmpDir, fmt.Sprintf("sqlite-remote-%d-%s", time.Now().UnixNano(), base))

	remote, err := c.sftp.Open(sourcePath)
	if err != nil {
		return fmt.Sprintf("cannot read sqlite %s: %v", sourcePath, err), 0, nil
	}
	defer remote.Close()

	local, err := os.OpenFile(tmpCopy, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create sqlite copy: %w", err)
	}
	if _, err := io.Copy(local, remote); err != nil {
		local.Close()
		os.Remove(tmpCopy)
		return "", 0, fmt.Errorf("copy sqlite %s: %w", sourcePath, err)
	}
	if err := local.Close(); err != nil {
		os.Remove(tmpCopy)
		return "", 0, err
	}
	defer os.Remove(tmpCopy)

	entry := zipEntryWithPrefix(prefix, sourcePath)
	if err := addLocalFile(zw, tmpCopy, entry); err != nil {
		return "", 0, err
	}
	return "", 1, nil
}

func (c *SSHClient) addRemoteFile(zw *zip.Writer, remotePath, entry string) error {
	remote, err := c.sftp.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open remote %s: %w", remotePath, err)
	}
	defer remote.Close()

	info, err := remote.Stat()
	if err != nil {
		return fmt.Errorf("stat remote %s: %w", remotePath, err)
	}

	header := &zip.FileHeader{
		Name:   entry,
		Method: zip.Deflate,
	}
	header.SetModTime(info.ModTime())
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, remote)
	return err
}

func zipEntryWithPrefix(prefix, absPath string) string {
	entry := strings.TrimPrefix(filepath.ToSlash(absPath), "/")
	if prefix != "" {
		entry = prefix + "/" + entry
	}
	return entry
}

func isSQLite(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".db" || ext == ".sqlite" || ext == ".sqlite3"
}

func shouldExclude(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func addLocalFile(zw *zip.Writer, sourcePath, entryName string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", sourcePath, err)
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", sourcePath, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("zip header %s: %w", sourcePath, err)
	}
	header.Name = entryName
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("zip entry %s: %w", entryName, err)
	}

	if _, err := io.Copy(writer, src); err != nil {
		return fmt.Errorf("write zip entry %s: %w", entryName, err)
	}
	return nil
}

func Ping(node config.NodeConfig) error {
	client, err := Connect(node)
	if err != nil {
		return err
	}
	return client.Close()
}