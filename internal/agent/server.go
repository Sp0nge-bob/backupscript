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
	mux.HandleFunc("/api/agent/heartbeat", s.handleHeartbeat)
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

type heartbeatRequest struct {
	Node    string       `json:"node"`
	Token   string       `json:"token"`
	Version string       `json:"version"`
	Paths   []PathReport `json:"paths"`
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req heartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	node, err := s.authenticate(req.Node, req.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	s.registry.RecordHeartbeat(node.Name, req.Version, req.Paths)
	syncRequired := s.registry.SyncRequired(node.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if syncRequired {
		_, _ = w.Write([]byte(`{"ok":true,"sync_required":true}`))
		return
	}
	_, _ = w.Write([]byte(`{"ok":true,"sync_required":false}`))
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