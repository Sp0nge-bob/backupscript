package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Sp0nge-bob/backupscript/internal/bot"
	"github.com/Sp0nge-bob/backupscript/internal/config"
	"github.com/Sp0nge-bob/backupscript/internal/scheduler"
)

func main() {
	log.SetFlags(log.LstdFlags)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	svc := bot.New(api, cfg)

	if _, err := scheduler.Start(cfg, svc); err != nil {
		log.Fatalf("scheduler: %v", err)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutting down")
		os.Exit(0)
	}()

	svc.Run()
}