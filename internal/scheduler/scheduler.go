package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"

	"github.com/Sp0nge-bob/backupscript/internal/config"
	"github.com/Sp0nge-bob/backupscript/internal/interval"
)

type BackupRunner interface {
	SendBackupTo(chatID int64) error
	API() *tgbotapi.BotAPI
}

type Manager struct {
	mu     sync.Mutex
	runner BackupRunner
	cfg    *config.Config
	stopCh chan struct{}
	cron   *cron.Cron
}

func New(runner BackupRunner) *Manager {
	return &Manager{runner: runner}
}

func (m *Manager) Start(cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopLocked()
	m.cfg = cfg

	if !cfg.Schedule.Enabled {
		log.Printf("scheduler disabled")
		return nil
	}

	if cfg.Schedule.UsesInterval() {
		return m.startIntervalLocked(cfg)
	}

	if cfg.Schedule.Cron != "" {
		return m.startCronLocked(cfg)
	}

	log.Printf("scheduler enabled but interval/cron not set")
	return nil
}

func (m *Manager) Reload(cfg *config.Config) error {
	return m.Start(cfg)
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	if m.cron != nil {
		ctx := m.cron.Stop()
		<-ctx.Done()
		m.cron = nil
	}
}

func (m *Manager) startIntervalLocked(cfg *config.Config) error {
	d, err := interval.Parse(cfg.Schedule.Interval)
	if err != nil {
		return fmt.Errorf("parse interval %q: %w", cfg.Schedule.Interval, err)
	}

	chatID := cfg.DefaultNotifyChatID()
	stopCh := make(chan struct{})
	m.stopCh = stopCh

	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()

		log.Printf("scheduler started: every %s", cfg.Schedule.Interval)
		for {
			select {
			case <-ticker.C:
				m.runBackup(chatID, "scheduled")
			case <-stopCh:
				return
			}
		}
	}()

	return nil
}

func (m *Manager) startCronLocked(cfg *config.Config) error {
	c := cron.New()
	chatID := cfg.DefaultNotifyChatID()

	_, err := c.AddFunc(cfg.Schedule.Cron, func() {
		m.runBackup(chatID, "cron")
	})
	if err != nil {
		return fmt.Errorf("parse cron %q: %w", cfg.Schedule.Cron, err)
	}

	c.Start()
	m.cron = c
	log.Printf("scheduler started: cron %s", cfg.Schedule.Cron)
	return nil
}

func (m *Manager) runBackup(chatID int64, kind string) {
	log.Printf("%s backup started for chat %d", kind, chatID)
	if err := m.runner.SendBackupTo(chatID); err != nil {
		log.Printf("%s backup failed: %v", kind, err)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Плановый бекап не удался:\n%s", err.Error()))
		if _, sendErr := m.runner.API().Send(msg); sendErr != nil {
			log.Printf("send scheduler error notification: %v", sendErr)
		}
		return
	}
	log.Printf("%s backup completed", kind)
}

func Start(cfg *config.Config, runner BackupRunner) (*Manager, error) {
	m := New(runner)
	if err := m.Start(cfg); err != nil {
		return nil, err
	}
	return m, nil
}