package config

import (
	"fmt"
	"os"
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
	Cron         string `yaml:"cron"`
	NotifyChatID *int64 `yaml:"notify_chat_id"`
}

type YAMLConfig struct {
	Backup   BackupConfig   `yaml:"backup"`
	Schedule ScheduleConfig `yaml:"schedule"`
}

type Config struct {
	Token          string
	AllowedUserIDs map[int64]bool
	AllowedOrder   []int64
	ConfigPath     string
	TmpDir         string
	Backup         BackupConfig
	Schedule       ScheduleConfig
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
	if len(yamlCfg.Backup.Paths) == 0 {
		return nil, fmt.Errorf("backup.paths is empty in %s", configPath)
	}

	return &Config{
		Token:          token,
		AllowedUserIDs: allowed,
		AllowedOrder:   allowedOrder,
		ConfigPath:     configPath,
		TmpDir:         tmpDir,
		Backup:         yamlCfg.Backup,
		Schedule:       yamlCfg.Schedule,
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