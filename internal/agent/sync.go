package agent

import (
	"fmt"
	"strings"

	"github.com/Sp0nge-bob/backupscript/internal/config"
)

func SyncAgentNodes(cfg *config.Config, registry *Registry) ([]string, error) {
	timeout, err := cfg.Agent.SyncTimeoutDuration()
	if err != nil {
		return nil, err
	}

	var agentNodes []config.NodeConfig
	for _, node := range cfg.Nodes {
		if node.NormalizedMode() == config.NodeModeAgent {
			agentNodes = append(agentNodes, node)
		}
	}
	if len(agentNodes) == 0 {
		return nil, nil
	}

	var warnings []string
	for _, node := range agentNodes {
		since := registry.RequestSync(node.Name)
		if err := registry.WaitForFreshUpload(node.Name, since, timeout); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", node.Name, err))
		}
	}

	if len(warnings) == len(agentNodes) {
		return warnings, fmt.Errorf("ни одна agent-нода не ответила: %s", strings.Join(warnings, "; "))
	}
	return warnings, nil
}