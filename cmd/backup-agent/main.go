package main

import (
	"log"
	"os"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/agent"
	"github.com/Sp0nge-bob/backupscript/internal/agentconfig"
	"github.com/Sp0nge-bob/backupscript/internal/interval"
)

const version = "1.0.0"

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

	d, err := interval.Parse(cfg.Interval)
	if err != nil {
		log.Fatalf("interval: %v", err)
	}

	clientCfg := agent.ClientConfig{
		Node:      cfg.Node,
		MasterURL: cfg.MasterURL,
		Token:     cfg.Token,
		Paths:     cfg.Paths,
		Version:   version,
	}

	log.Printf("backup-agent %s for node %s", version, cfg.Node)
	runOnce(clientCfg, cfg.TmpDir)

	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for range ticker.C {
		runOnce(clientCfg, cfg.TmpDir)
	}
}

func runOnce(cfg agent.ClientConfig, tmpDir string) {
	if err := agent.Heartbeat(cfg); err != nil {
		log.Printf("heartbeat failed: %v", err)
	} else {
		log.Printf("heartbeat ok")
	}

	if err := agent.Upload(cfg, tmpDir); err != nil {
		log.Printf("upload failed: %v", err)
		return
	}
	log.Printf("upload ok")
}