package agent

import (
	"fmt"
	"strings"
	"time"

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
		if !registry.HasWaiter(node.Name) {
			warnings = append(warnings, fmt.Sprintf("%s: агент не подключён (нет активного канала)", node.Name))
			continue
		}
		since := registry.RequestSync(node.Name)
		if err := registry.WaitForFreshUpload(node.Name, since, timeout); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", node.Name, err))
		}
	}

	if len(warnings) == len(agentNodes) {
		return warnings, fmt.Errorf("ни одна agent-нода не синхронизировалась: %s", strings.Join(warnings, "; "))
	}
	return warnings, nil
}

func PingAgentNode(cfg *config.Config, registry *Registry, nodeName string) error {
	node, _, err := cfg.FindNode(nodeName)
	if err != nil {
		return err
	}
	if node.NormalizedMode() != config.NodeModeAgent {
		return fmt.Errorf("нода %s не в режиме agent", nodeName)
	}

	stateBefore, _ := registry.Get(node.Name)
	since := stateBefore.LastSeen

	if !registry.HasWaiter(node.Name) {
		return fmt.Errorf("агент %s не подключён к master", nodeName)
	}

	registry.WakeNode(node.Name)
	return registry.WaitForContact(node.Name, since, 30*time.Second)
}