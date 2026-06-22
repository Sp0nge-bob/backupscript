package scheduler

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"

	"github.com/Sp0nge-bob/backupscript/internal/config"
)

type BackupRunner interface {
	SendBackupTo(chatID int64) error
	API() *tgbotapi.BotAPI
}

func Start(cfg *config.Config, runner BackupRunner) (*cron.Cron, error) {
	if !cfg.Schedule.Enabled {
		log.Printf("scheduler disabled in config")
		return nil, nil
	}

	c := cron.New()
	chatID := cfg.DefaultNotifyChatID()

	_, err := c.AddFunc(cfg.Schedule.Cron, func() {
		log.Printf("scheduled backup started for chat %d", chatID)
		if err := runner.SendBackupTo(chatID); err != nil {
			log.Printf("scheduled backup failed: %v", err)
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Плановый бекап не удался:\n%s", err.Error()))
			if _, sendErr := runner.API().Send(msg); sendErr != nil {
				log.Printf("send scheduler error notification: %v", sendErr)
			}
			return
		}
		log.Printf("scheduled backup completed")
	})
	if err != nil {
		return nil, fmt.Errorf("parse cron %q: %w", cfg.Schedule.Cron, err)
	}

	c.Start()
	log.Printf("scheduler started: %s", cfg.Schedule.Cron)
	return c, nil
}