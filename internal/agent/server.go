package agent

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/config"
)

type Server struct {
	cfg      *config.Config
	registry *Registry
}

func NewServer(cfg *config.Config, registry *Registry) *Server {
	return &Server{cfg: cfg, registry: registry}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/wait-sync", s.handleWaitSync)
	mux.HandleFunc("/api/agent/upload", s.handleUpload)

	server := &http.Server{
		Addr:              s.cfg.Agent.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("agent API listening on %s", s.cfg.Agent.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("agent API error: %v", err)
		}
	}()
	return nil
}

func (s *Server) handleWaitSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeName := r.URL.Query().Get("node")
	token := r.URL.Query().Get("token")
	timeoutRaw := r.URL.Query().Get("timeout")

	node, err := s.authenticate(nodeName, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	timeout, err := time.ParseDuration(timeoutRaw)
	if err != nil || timeout <= 0 {
		timeout = 6 * time.Hour
	}
	if timeout > 6*time.Hour {
		timeout = 6 * time.Hour
	}

	s.registry.RecordContact(node.Name)

	waiter := s.registry.RegisterWaiter(node.Name)
	defer s.registry.UnregisterWaiter(node.Name, waiter)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case syncRequired := <-waiter:
		writeWaitResponse(w, syncRequired)
	case <-timer.C:
		writeWaitResponse(w, s.registry.SyncRequired(node.Name))
	case <-r.Context().Done():
		return
	}
}

func writeWaitResponse(w http.ResponseWriter, syncRequired bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"ok":            true,
		"sync_required": syncRequired,
	})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeName := r.FormValue("node")
	token := r.FormValue("token")
	node, err := s.authenticate(nodeName, token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	file, header, err := r.FormFile("archive")
	if err != nil {
		http.Error(w, "archive field required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	stagingDir := s.cfg.StagingDir(node.Name)
	if err := os.RemoveAll(stagingDir); err != nil {
		http.Error(w, "reset staging failed", http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		http.Error(w, "create staging failed", http.StatusInternalServerError)
		return
	}

	if err := extractArchive(file, stagingDir, header.Filename); err != nil {
		s.registry.RecordError(node.Name, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.registry.RecordUpload(node.Name)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) authenticate(nodeName, token string) (config.NodeConfig, error) {
	nodeName = strings.TrimSpace(nodeName)
	token = strings.TrimSpace(token)
	if nodeName == "" || token == "" {
		return config.NodeConfig{}, fmt.Errorf("node and token required")
	}

	node, _, err := s.cfg.FindNode(nodeName)
	if err != nil {
		return config.NodeConfig{}, fmt.Errorf("unknown node")
	}
	if node.NormalizedMode() != config.NodeModeAgent {
		return config.NodeConfig{}, fmt.Errorf("node is not agent mode")
	}
	if node.Token != token {
		return config.NodeConfig{}, fmt.Errorf("invalid token")
	}
	return node, nil
}

func extractArchive(reader io.Reader, destDir, filename string) error {
	lower := strings.ToLower(filename)
	var input io.Reader = reader

	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tgz") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		input = gz
	}

	tr := tar.NewReader(input)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}

		name := strings.TrimPrefix(filepath.Clean(header.Name), "/")
		if name == "." || name == "" {
			continue
		}

		target := filepath.Join(destDir, name)
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != destDir {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}

		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}