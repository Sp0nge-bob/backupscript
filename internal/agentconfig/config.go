package agentconfig

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node      string   `yaml:"node"`
	MasterURL string   `yaml:"master_url"`
	Token     string   `yaml:"token"`
	Paths     []string `yaml:"paths"`
	Interval  string   `yaml:"interval"`
	TmpDir    string   `yaml:"tmp_dir"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.Node = strings.TrimSpace(cfg.Node)
	cfg.MasterURL = strings.TrimSpace(cfg.MasterURL)
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.Interval = strings.TrimSpace(cfg.Interval)

	if cfg.Node == "" {
		return nil, fmt.Errorf("node is required")
	}
	if cfg.MasterURL == "" {
		return nil, fmt.Errorf("master_url is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if len(cfg.Paths) == 0 {
		return nil, fmt.Errorf("paths is required")
	}
	if cfg.Interval == "" {
		cfg.Interval = "1h"
	}
	if cfg.TmpDir == "" {
		cfg.TmpDir = "/tmp/backup-agent"
	}

	return &cfg, nil
}