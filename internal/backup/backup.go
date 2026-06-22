package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxTelegramFileSize = 50 * 1024 * 1024

type Config struct {
	Name    string
	Paths   []string
	Exclude []string
	TmpDir  string
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

	for _, sourcePath := range cfg.Paths {
		warn, count, err := addSource(zw, sourcePath, cfg)
		if err != nil {
			zw.Close()
			zipFile.Close()
			os.Remove(archivePath)
			return nil, err
		}
		if warn != "" {
			result.Warnings = append(result.Warnings, warn)
		}
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
		return nil, fmt.Errorf("no files added to archive; check backup.paths in config")
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
			"archive size %s exceeds Telegram limit of 50 MB; reduce backup.paths or split backup",
			formatSize(result.Size),
		)
	}

	return result, nil
}

func addSource(zw *zip.Writer, sourcePath string, cfg Config) (warning string, count int, err error) {
	info, statErr := os.Stat(sourcePath)
	if statErr != nil {
		return fmt.Sprintf("missing: %s", sourcePath), 0, nil
	}

	if info.IsDir() {
		count, err = addDirectory(zw, sourcePath, cfg)
		return "", count, err
	}

	if isSQLite(sourcePath) {
		return addSQLiteFile(zw, sourcePath, cfg.TmpDir)
	}

	return "", 0, addFile(zw, sourcePath, zipEntryName(sourcePath))
}

func addDirectory(zw *zip.Writer, root string, cfg Config) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if shouldExclude(info.Name(), cfg.Exclude) {
			return nil
		}
		if isSQLite(path) {
			_, _, err := addSQLiteFile(zw, path, cfg.TmpDir)
			if err != nil {
				return err
			}
			count++
			return nil
		}
		if err := addFile(zw, path, zipEntryName(path)); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func addSQLiteFile(zw *zip.Writer, sourcePath, tmpDir string) (warning string, count int, err error) {
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

	if err := addFile(zw, tmpCopy, zipEntryName(sourcePath)); err != nil {
		return "", 0, err
	}
	return "", 1, nil
}

func addFile(zw *zip.Writer, sourcePath, entryName string) error {
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

func zipEntryName(absPath string) string {
	return strings.TrimPrefix(filepath.ToSlash(absPath), "/")
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