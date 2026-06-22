package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Sp0nge-bob/backupscript/internal/agent"
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

	registry := agent.NewRegistry()
	agentServer := agent.NewServer(cfg, registry)
	if err := agentServer.Start(); err != nil {
		log.Fatalf("agent api: %v", err)
	}

	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	svc := bot.New(api, cfg)
	svc.SetAgentRegistry(registry)

	sched, err := scheduler.Start(cfg, svc)
	if err != nil {
		log.Fatalf("scheduler: %v", err)
	}
	svc.SetScheduler(sched)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutting down")
		os.Exit(0)
	}()

	svc.Run()
}