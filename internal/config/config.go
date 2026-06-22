package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type BackupConfig struct {
	Name    string   `yaml:"name"`
	Paths   []string `yaml:"paths"`
	Exclude []string `yaml:"exclude"`
}

type ScheduleConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Interval     string `yaml:"interval"`
	Cron         string `yaml:"cron"`
	NotifyChatID *int64 `yaml:"notify_chat_id"`
}

type YAMLConfig struct {
	Backup   BackupConfig     `yaml:"backup"`
	Schedule ScheduleConfig   `yaml:"schedule"`
	Agent    AgentConfig      `yaml:"agent"`
	Nodes    []NodeConfig     `yaml:"nodes"`
}

type Config struct {
	Token          string
	AllowedUserIDs map[int64]bool
	AllowedOrder   []int64
	ConfigPath     string
	TmpDir         string
	Backup         BackupConfig
	Schedule       ScheduleConfig
	Agent          AgentConfig
	Nodes          []NodeConfig
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	allowedRaw := os.Getenv("ALLOWED_USER_IDS")
	if allowedRaw == "" {
		return nil, fmt.Errorf("ALLOWED_USER_IDS is not set")
	}

	allowed := make(map[int64]bool)
	var allowedOrder []int64
	for _, part := range strings.Split(allowedRaw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ALLOWED_USER_IDS entry %q: %w", part, err)
		}
		if !allowed[id] {
			allowedOrder = append(allowedOrder, id)
		}
		allowed[id] = true
	}
	if len(allowed) == 0 {
		return nil, fmt.Errorf("ALLOWED_USER_IDS is empty")
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	tmpDir := os.Getenv("BACKUP_TMP_DIR")
	if tmpDir == "" {
		tmpDir = "/tmp/backup-bot"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if yamlCfg.Backup.Name == "" {
		yamlCfg.Backup.Name = "server-backup"
	}
	if yamlCfg.Agent.Listen == "" {
		yamlCfg.Agent.Listen = "0.0.0.0:9876"
	}
	if yamlCfg.Agent.MaxStagingAge == "" {
		yamlCfg.Agent.MaxStagingAge = "2h"
	}
	if yamlCfg.Nodes == nil {
		yamlCfg.Nodes = []NodeConfig{}
	}
	if err := validateNodes(yamlCfg.Nodes); err != nil {
		return nil, err
	}
	if !hasAnyBackupPaths(yamlCfg.Backup, yamlCfg.Nodes) {
		return nil, fmt.Errorf("no backup paths configured (backup.paths or nodes[].paths)")
	}

	return &Config{
		Token:          token,
		AllowedUserIDs: allowed,
		AllowedOrder:   allowedOrder,
		ConfigPath:     configPath,
		TmpDir:         tmpDir,
		Backup:         yamlCfg.Backup,
		Schedule:       yamlCfg.Schedule,
		Agent:          yamlCfg.Agent,
		Nodes:          yamlCfg.Nodes,
	}, nil
}

func (c *Config) DefaultNotifyChatID() int64 {
	if c.Schedule.NotifyChatID != nil {
		return *c.Schedule.NotifyChatID
	}
	return c.AllowedOrder[0]
}

func (c *Config) IsAllowed(userID int64) bool {
	return c.AllowedUserIDs[userID]
}

func (s ScheduleConfig) UsesInterval() bool {
	return strings.TrimSpace(s.Interval) != ""
}

func (s ScheduleConfig) ScheduleDescription() string {
	if !s.Enabled {
		return "выключено"
	}
	if s.UsesInterval() {
		return "каждые " + strings.TrimSpace(s.Interval)
	}
	if strings.TrimSpace(s.Cron) != "" {
		return "cron: " + strings.TrimSpace(s.Cron)
	}
	return "не настроено"
}

func NormalizeBackupPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("путь пустой")
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("путь должен быть абсолютным, например /etc/nginx/nginx.conf")
	}
	cleaned := filepath.Clean(p)
	if cleaned == "/" {
		return "", fmt.Errorf("нельзя добавить корень /")
	}
	return cleaned, nil
}

func (c *Config) saveYAML(mutate func(*YAMLConfig) error) error {
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		return fmt.Errorf("read config %s: %w", c.ConfigPath, err)
	}

	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(data, &yamlCfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := mutate(&yamlCfg); err != nil {
		return err
	}

	out, err := yaml.Marshal(&yamlCfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(c.ConfigPath, out, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", c.ConfigPath, err)
	}

	c.Backup = yamlCfg.Backup
	c.Schedule = yamlCfg.Schedule
	c.Agent = yamlCfg.Agent
	c.Nodes = yamlCfg.Nodes
	return nil
}

func (c *Config) SaveSchedule(schedule ScheduleConfig) error {
	return c.saveYAML(func(yamlCfg *YAMLConfig) error {
		yamlCfg.Schedule = schedule
		return nil
	})
}

func (c *Config) SaveBackup(backupCfg BackupConfig) error {
	if !hasAnyBackupPaths(backupCfg, c.Nodes) {
		return fmt.Errorf("должен остаться хотя бы один путь (master или ноды)")
	}
	return c.saveYAML(func(yamlCfg *YAMLConfig) error {
		if backupCfg.Name == "" {
			backupCfg.Name = yamlCfg.Backup.Name
		}
		if backupCfg.Name == "" {
			backupCfg.Name = "server-backup"
		}
		if backupCfg.Exclude == nil {
			backupCfg.Exclude = yamlCfg.Backup.Exclude
		}
		yamlCfg.Backup = backupCfg
		return nil
	})
}

func (c *Config) AddBackupPath(path string) error {
	normalized, err := NormalizeBackupPath(path)
	if err != nil {
		return err
	}

	for _, existing := range c.Backup.Paths {
		if filepath.Clean(existing) == normalized {
			return fmt.Errorf("путь уже есть: %s", normalized)
		}
	}

	paths := append(append([]string{}, c.Backup.Paths...), normalized)
	backupCfg := c.Backup
	backupCfg.Paths = paths
	return c.SaveBackup(backupCfg)
}

func (c *Config) RemoveBackupPath(path string) error {
	normalized, err := NormalizeBackupPath(path)
	if err != nil {
		return err
	}

	var paths []string
	found := false
	for _, existing := range c.Backup.Paths {
		if filepath.Clean(existing) == normalized {
			found = true
			continue
		}
		paths = append(paths, existing)
	}
	if !found {
		return fmt.Errorf("путь не найден: %s", normalized)
	}

	backupCfg := c.Backup
	backupCfg.Paths = paths
	if !hasAnyBackupPaths(backupCfg, c.Nodes) {
		return fmt.Errorf("нельзя удалить последний путь")
	}
	return c.SaveBackup(backupCfg)
}