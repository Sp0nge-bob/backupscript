package main

import (
	"log"
	"os"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/agent"
	"github.com/Sp0nge-bob/backupscript/internal/agentconfig"
	"github.com/Sp0nge-bob/backupscript/internal/interval"
)

const version = "1.2.0"

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

	listenTimeout, err := interval.Parse(cfg.ListenTimeout)
	if err != nil {
		log.Fatalf("listen_timeout: %v", err)
	}

	clientCfg := agent.ClientConfig{
		Node:          cfg.Node,
		MasterURL:     cfg.MasterURL,
		Token:         cfg.Token,
		Paths:         cfg.Paths,
		Version:       version,
		ListenTimeout: listenTimeout,
	}

	log.Printf("backup-agent %s for node %s (listen %s)", version, cfg.Node, cfg.ListenTimeout)

	for {
		runListen(clientCfg, cfg.TmpDir)
		time.Sleep(5 * time.Second)
	}
}

func runListen(cfg agent.ClientConfig, tmpDir string) {
	resp, err := agent.WaitForSync(cfg)
	if err != nil {
		log.Printf("listen failed: %v", err)
		return
	}

	if resp.SyncRequired {
		log.Printf("backup requested, uploading fresh data")
		if err := agent.Upload(cfg, tmpDir); err != nil {
			log.Printf("upload failed: %v", err)
			return
		}
		log.Printf("upload ok")
	}
}