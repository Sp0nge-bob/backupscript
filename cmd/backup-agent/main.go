package main

import (
	"log"
	"os"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/agent"
	"github.com/Sp0nge-bob/backupscript/internal/agentconfig"
	"github.com/Sp0nge-bob/backupscript/internal/interval"
)

const version = "1.1.0"

func main() {
	log.SetFlags(log.LstdFlags)

	configPath := os.Getenv("AGENT_CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/backup-agent/config.yaml"
	}

	cfg, err := agentconfig.Load(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pollInterval, err := interval.Parse(cfg.PollInterval)
	if err != nil {
		log.Fatalf("poll_interval: %v", err)
	}

	clientCfg := agent.ClientConfig{
		Node:      cfg.Node,
		MasterURL: cfg.MasterURL,
		Token:     cfg.Token,
		Paths:     cfg.Paths,
		Version:   version,
	}

	log.Printf("backup-agent %s for node %s (poll %s)", version, cfg.Node, cfg.PollInterval)
	runPoll(clientCfg, cfg.TmpDir)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for range ticker.C {
		runPoll(clientCfg, cfg.TmpDir)
	}
}

func runPoll(cfg agent.ClientConfig, tmpDir string) {
	resp, err := agent.Heartbeat(cfg)
	if err != nil {
		log.Printf("heartbeat failed: %v", err)
		return
	}

	if resp.SyncRequired {
		log.Printf("master requested fresh sync")
		if err := agent.Upload(cfg, tmpDir); err != nil {
			log.Printf("sync upload failed: %v", err)
			return
		}
		log.Printf("sync upload ok")
	}
}