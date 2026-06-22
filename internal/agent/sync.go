package agent

import (
	"fmt"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/config"
)

func SyncAgentNodes(cfg *config.Config, registry *Registry) (failed []string, warnings []string, err error) {
	timeout, err := cfg.Agent.SyncTimeoutDuration()
	if err != nil {
		return nil, nil, err
	}

	var agentNodes []config.NodeConfig
	for _, node := range cfg.Nodes {
		if node.NormalizedMode() == config.NodeModeAgent {
			agentNodes = append(agentNodes, node)
		}
	}
	if len(agentNodes) == 0 {
		return nil, nil, nil
	}

	for _, node := range agentNodes {
		if !registry.HasWaiter(node.Name) {
			msg := fmt.Sprintf("%s: агент не подключён — нода пропущена", node.Name)
			warnings = append(warnings, msg)
			failed = append(failed, node.Name)
			continue
		}
		since := registry.RequestSync(node.Name)
		if err := registry.WaitForFreshUpload(node.Name, since, timeout); err != nil {
			msg := fmt.Sprintf("%s: %v — нода пропущена", node.Name, err)
			warnings = append(warnings, msg)
			failed = append(failed, node.Name)
		}
	}

	return failed, warnings, nil
}

func SyncAgentNode(cfg *config.Config, registry *Registry, nodeName string) error {
	node, _, err := cfg.FindNode(nodeName)
	if err != nil {
		return err
	}
	if node.NormalizedMode() != config.NodeModeAgent {
		return nil
	}

	timeout, err := cfg.Agent.SyncTimeoutDuration()
	if err != nil {
		return err
	}

	if !registry.HasWaiter(node.Name) {
		return fmt.Errorf("агент не подключён")
	}
	since := registry.RequestSync(node.Name)
	if err := registry.WaitForFreshUpload(node.Name, since, timeout); err != nil {
		return err
	}
	return nil
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