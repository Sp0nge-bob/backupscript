package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/config"
	"github.com/Sp0nge-bob/backupscript/internal/remote"
)

const MaxTelegramFileSize = 50 * 1024 * 1024

const LocalPrefix = "local"

type Config struct {
	Name              string
	Paths             []string
	Exclude           []string
	TmpDir            string
	Nodes             []config.NodeConfig
	StagingDir        func(string) string
	MaxStagingAge     time.Duration
	StagingStaleWarn  func(string) string
}

type PathStatus struct {
	Path   string
	Exists bool
	IsDir  bool
}

type Result struct {
	Path      string
	Size      int64
	CreatedAt time.Time
	Warnings  []string
}

func InspectPaths(paths []string) []PathStatus {
	statuses := make([]PathStatus, 0, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p)
		statuses = append(statuses, PathStatus{
			Path:   p,
			Exists: err == nil,
			IsDir:  err == nil && info.IsDir(),
		})
	}
	return statuses
}

func Create(cfg Config) (*Result, error) {
	if err := os.MkdirAll(cfg.TmpDir, 0o700); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	archiveName := fmt.Sprintf("%s_%s.zip", cfg.Name, timestamp)
	archivePath := filepath.Join(cfg.TmpDir, archiveName)

	result := &Result{
		Path:      archivePath,
		CreatedAt: time.Now(),
	}

	zipFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("create archive: %w", err)
	}

	zw := zip.NewWriter(zipFile)
	added := 0
	localCfg := cfg
	localCfg.Paths = cfg.Paths

	for _, sourcePath := range cfg.Paths {
		warn, count, err := addLocalSource(zw, sourcePath, localCfg, LocalPrefix)
		if err != nil {
			zw.Close()
			zipFile.Close()
			os.Remove(archivePath)
			return nil, err
		}
		if warn != "" {
			result.Warnings = append(result.Warnings, prefixWarning(LocalPrefix, warn))
		}
		added += count
	}

	for _, node := range cfg.Nodes {
		warns, count, err := addNodeSources(zw, node, cfg)
		if err != nil {
			result.Warnings = append(result.Warnings, prefixWarning(node.Name, err.Error()))
			continue
		}
		result.Warnings = append(result.Warnings, warns...)
		added += count
	}

	if err := zw.Close(); err != nil {
		zipFile.Close()
		os.Remove(archivePath)
		return nil, fmt.Errorf("finalize archive: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		os.Remove(archivePath)
		return nil, fmt.Errorf("close archive: %w", err)
	}

	if added == 0 {
		os.Remove(archivePath)
		return nil, fmt.Errorf("no files added to archive; check backup.paths and nodes")
	}

	info, err := os.Stat(archivePath)
	if err != nil {
		os.Remove(archivePath)
		return nil, fmt.Errorf("stat archive: %w", err)
	}
	result.Size = info.Size()

	if result.Size > MaxTelegramFileSize {
		os.Remove(archivePath)
		return nil, fmt.Errorf(
			"archive size %s exceeds Telegram limit of 50 MB; reduce paths or nodes",
			formatSize(result.Size),
		)
	}

	return result, nil
}

func addNodeSources(zw *zip.Writer, node config.NodeConfig, cfg Config) (warnings []string, count int, err error) {
	switch node.NormalizedMode() {
	case config.NodeModeSSH:
		client, connErr := remote.Connect(node)
		if connErr != nil {
			return nil, 0, fmt.Errorf("ssh connect failed: %w", connErr)
		}
		defer client.Close()
		warns, added, addErr := client.AddToZip(zw, node.Paths, cfg.Exclude, node.Name, cfg.TmpDir)
		for _, w := range warns {
			warnings = append(warnings, prefixWarning(node.Name, w))
		}
		return warnings, added, addErr

	case config.NodeModeAgent:
		if cfg.StagingStaleWarn != nil {
			if warn := cfg.StagingStaleWarn(node.Name); warn != "" {
				warnings = append(warnings, prefixWarning(node.Name, warn))
			}
		}
		stagingDir := ""
		if cfg.StagingDir != nil {
			stagingDir = cfg.StagingDir(node.Name)
		}
		if stagingDir == "" {
			return []string{prefixWarning(node.Name, "staging dir not configured")}, 0, nil
		}
		if _, statErr := os.Stat(stagingDir); statErr != nil {
			return []string{prefixWarning(node.Name, "no agent upload yet")}, 0, nil
		}
		localCfg := cfg
		localCfg.Paths = []string{stagingDir}
		warn, added, walkErr := addLocalDirectory(zw, stagingDir, localCfg, node.Name, true)
		if warn != "" {
			warnings = append(warnings, prefixWarning(node.Name, warn))
		}
		return warnings, added, walkErr

	default:
		return nil, 0, fmt.Errorf("unknown node mode %q", node.Mode)
	}
}

func addLocalSource(zw *zip.Writer, sourcePath string, cfg Config, prefix string) (warning string, count int, err error) {
	info, statErr := os.Stat(sourcePath)
	if statErr != nil {
		return fmt.Sprintf("missing: %s", sourcePath), 0, nil
	}

	if info.IsDir() {
		return addLocalDirectory(zw, sourcePath, cfg, prefix, false)
	}

	if isSQLite(sourcePath) {
		return addSQLiteFile(zw, sourcePath, cfg.TmpDir, prefix)
	}

	entry := zipEntryWithPrefix(prefix, sourcePath)
	return "", 0, AddLocalFile(zw, sourcePath, entry)
}

func addLocalDirectory(zw *zip.Writer, root string, cfg Config, prefix string, stripRoot bool) (warning string, count int, err error) {
	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if shouldExclude(info.Name(), cfg.Exclude) {
			return nil
		}

		entryPath := path
		if stripRoot {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			entryPath = rel
			entry := prefix + "/" + filepath.ToSlash(rel)
			if isSQLite(path) {
				_, _, err := addSQLiteFileWithEntry(zw, path, cfg.TmpDir, entry)
				if err != nil {
					return err
				}
				count++
				return nil
			}
			return AddLocalFile(zw, path, entry)
		}

		if isSQLite(path) {
			_, _, err := addSQLiteFile(zw, path, cfg.TmpDir, prefix)
			if err != nil {
				return err
			}
			count++
			return nil
		}
		entry := zipEntryWithPrefix(prefix, entryPath)
		if err := AddLocalFile(zw, path, entry); err != nil {
			return err
		}
		count++
		return nil
	})
	return warning, count, err
}

func addSQLiteFile(zw *zip.Writer, sourcePath, tmpDir, prefix string) (warning string, count int, err error) {
	entry := zipEntryWithPrefix(prefix, sourcePath)
	return addSQLiteFileWithEntry(zw, sourcePath, tmpDir, entry)
}

func addSQLiteFileWithEntry(zw *zip.Writer, sourcePath, tmpDir, entry string) (warning string, count int, err error) {
	base := filepath.Base(sourcePath)
	tmpCopy := filepath.Join(tmpDir, fmt.Sprintf("sqlite-copy-%d-%s", time.Now().UnixNano(), base))

	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Sprintf("cannot read sqlite %s: %v", sourcePath, err), 0, nil
	}
	defer src.Close()

	dst, err := os.OpenFile(tmpCopy, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", 0, fmt.Errorf("create sqlite copy: %w", err)
	}

	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmpCopy)
		return "", 0, fmt.Errorf("copy sqlite %s: %w", sourcePath, err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(tmpCopy)
		return "", 0, fmt.Errorf("close sqlite copy: %w", err)
	}
	defer os.Remove(tmpCopy)

	if err := AddLocalFile(zw, tmpCopy, entry); err != nil {
		return "", 0, err
	}
	return "", 1, nil
}

func AddLocalFile(zw *zip.Writer, sourcePath, entryName string) error {
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

func zipEntryWithPrefix(prefix, absPath string) string {
	entry := strings.TrimPrefix(filepath.ToSlash(absPath), "/")
	if prefix != "" {
		entry = prefix + "/" + entry
	}
	return entry
}

func prefixWarning(prefix, msg string) string {
	if prefix == "" {
		return msg
	}
	return prefix + ": " + msg
}

func FormatSize(size int64) string {
	return formatSize(size)
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}