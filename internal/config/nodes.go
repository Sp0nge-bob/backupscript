package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	NodeModeSSH   = "ssh"
	NodeModeAgent = "agent"
)

type AgentConfig struct {
	Listen        string `yaml:"listen"`
	MaxStagingAge string `yaml:"max_staging_age"`
}

type NodeConfig struct {
	Name    string   `yaml:"name"`
	Mode    string   `yaml:"mode"`
	Host    string   `yaml:"host,omitempty"`
	Port    int      `yaml:"port,omitempty"`
	User    string   `yaml:"user,omitempty"`
	KeyFile string   `yaml:"key_file,omitempty"`
	Token   string   `yaml:"token,omitempty"`
	Paths   []string `yaml:"paths"`
}

func (n NodeConfig) NormalizedMode() string {
	return strings.ToLower(strings.TrimSpace(n.Mode))
}

func (n NodeConfig) SSHPort() int {
	if n.Port == 0 {
		return 22
	}
	return n.Port
}

func (n NodeConfig) Validate() error {
	name := strings.TrimSpace(n.Name)
	if name == "" {
		return fmt.Errorf("node name is required")
	}

	switch n.NormalizedMode() {
	case NodeModeSSH:
		if strings.TrimSpace(n.Host) == "" {
			return fmt.Errorf("node %s: host is required for ssh mode", name)
		}
		if strings.TrimSpace(n.User) == "" {
			return fmt.Errorf("node %s: user is required for ssh mode", name)
		}
		if strings.TrimSpace(n.KeyFile) == "" {
			return fmt.Errorf("node %s: key_file is required for ssh mode", name)
		}
	case NodeModeAgent:
		if strings.TrimSpace(n.Token) == "" {
			return fmt.Errorf("node %s: token is required for agent mode", name)
		}
	default:
		return fmt.Errorf("node %s: unknown mode %q (use ssh or agent)", name, n.Mode)
	}
	return nil
}

func (a AgentConfig) MaxStagingAgeDuration() (time.Duration, error) {
	raw := strings.TrimSpace(a.MaxStagingAge)
	if raw == "" {
		return 2 * time.Hour, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid agent.max_staging_age %q: %w", raw, err)
	}
	return d, nil
}

func (c *Config) StagingDir(nodeName string) string {
	return filepath.Join(c.TmpDir, "staging", nodeName)
}

func (c *Config) FindNode(name string) (NodeConfig, int, error) {
	name = strings.TrimSpace(name)
	for i, node := range c.Nodes {
		if node.Name == name {
			return node, i, nil
		}
	}
	return NodeConfig{}, -1, fmt.Errorf("нода не найдена: %s", name)
}

func (c *Config) SaveNodes(nodes []NodeConfig) error {
	return c.saveYAML(func(yamlCfg *YAMLConfig) error {
		yamlCfg.Nodes = nodes
		return nil
	})
}

func (c *Config) AddNodePath(nodeName, path string) error {
	normalized, err := NormalizeBackupPath(path)
	if err != nil {
		return err
	}

	node, idx, err := c.FindNode(nodeName)
	if err != nil {
		return err
	}

	for _, existing := range node.Paths {
		if filepath.Clean(existing) == normalized {
			return fmt.Errorf("путь уже есть на ноде %s: %s", nodeName, normalized)
		}
	}

	node.Paths = append(append([]string{}, node.Paths...), normalized)
	nodes := append([]NodeConfig{}, c.Nodes...)
	nodes[idx] = node
	return c.SaveNodes(nodes)
}

func (c *Config) RemoveNodePath(nodeName, path string) error {
	normalized, err := NormalizeBackupPath(path)
	if err != nil {
		return err
	}

	node, idx, err := c.FindNode(nodeName)
	if err != nil {
		return err
	}

	var paths []string
	found := false
	for _, existing := range node.Paths {
		if filepath.Clean(existing) == normalized {
			found = true
			continue
		}
		paths = append(paths, existing)
	}
	if !found {
		return fmt.Errorf("путь не найден на ноде %s: %s", nodeName, normalized)
	}

	node.Paths = paths
	nodes := append([]NodeConfig{}, c.Nodes...)
	nodes[idx] = node
	return c.SaveNodes(nodes)
}

func validateNodes(nodes []NodeConfig) error {
	seen := make(map[string]struct{})
	for _, node := range nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if _, ok := seen[node.Name]; ok {
			return fmt.Errorf("duplicate node name: %s", node.Name)
		}
		seen[node.Name] = struct{}{}
	}
	return nil
}

func hasAnyBackupPaths(backup BackupConfig, nodes []NodeConfig) bool {
	if len(backup.Paths) > 0 {
		return true
	}
	for _, node := range nodes {
		if len(node.Paths) > 0 {
			return true
		}
	}
	return false
}