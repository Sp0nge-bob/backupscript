package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ClientConfig struct {
	Node          string
	MasterURL     string
	Token         string
	Paths         []string
	Version       string
	ListenTimeout time.Duration
}

type WaitResponse struct {
	OK           bool `json:"ok"`
	SyncRequired bool `json:"sync_required"`
}

func WaitForSync(cfg ClientConfig) (WaitResponse, error) {
	timeout := cfg.ListenTimeout
	if timeout <= 0 {
		timeout = 6 * time.Hour
	}

	base := strings.TrimRight(cfg.MasterURL, "/")
	query := url.Values{}
	query.Set("node", cfg.Node)
	query.Set("token", cfg.Token)
	query.Set("timeout", timeout.String())
	endpoint := fmt.Sprintf("%s/api/agent/wait-sync?%s", base, query.Encode())

	client := &http.Client{Timeout: timeout + 30*time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return WaitResponse{}, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return WaitResponse{}, fmt.Errorf("wait-sync status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var result WaitResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return WaitResponse{}, fmt.Errorf("parse wait-sync response: %w", err)
	}
	return result, nil
}

func Upload(cfg ClientConfig, tmpDir string) error {
	archivePath, err := buildArchive(cfg.Paths, tmpDir)
	if err != nil {
		return err
	}
	defer os.Remove(archivePath)

	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("node", cfg.Node)
	_ = writer.WriteField("token", cfg.Token)

	part, err := writer.CreateFormFile("archive", filepath.Base(archivePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	url := strings.TrimRight(cfg.MasterURL, "/") + "/api/agent/upload"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func buildArchive(paths []string, tmpDir string) (string, error) {
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", err
	}

	archivePath := filepath.Join(tmpDir, fmt.Sprintf("agent-upload-%d.tar.gz", time.Now().UnixNano()))
	file, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)

	added := 0
	for _, source := range paths {
		info, err := os.Stat(source)
		if err != nil {
			continue
		}
		if info.IsDir() {
			count, err := addDirToTar(tw, source)
			if err != nil {
				tw.Close()
				gz.Close()
				file.Close()
				os.Remove(archivePath)
				return "", err
			}
			added += count
			continue
		}
		if err := addFileToTar(tw, source); err != nil {
			tw.Close()
			gz.Close()
			file.Close()
			os.Remove(archivePath)
			return "", err
		}
		added++
	}

	if added == 0 {
		tw.Close()
		gz.Close()
		file.Close()
		os.Remove(archivePath)
		return "", fmt.Errorf("no files to upload")
	}

	if err := tw.Close(); err != nil {
		gz.Close()
		file.Close()
		os.Remove(archivePath)
		return "", err
	}
	if err := gz.Close(); err != nil {
		file.Close()
		os.Remove(archivePath)
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(archivePath)
		return "", err
	}
	return archivePath, nil
}

func addDirToTar(tw *tar.Writer, root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if err := addFileToTar(tw, path); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func addFileToTar(tw *tar.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = strings.TrimPrefix(filepath.ToSlash(path), "/")

	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}