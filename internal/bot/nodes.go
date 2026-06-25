package bot

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sp0nge-bob/backupscript/internal/agent"
	"github.com/Sp0nge-bob/backupscript/internal/backup"
	"github.com/Sp0nge-bob/backupscript/internal/config"
	"github.com/Sp0nge-bob/backupscript/internal/remote"
)

const agentOnlineWindow = 6 * time.Hour

func (s *Service) handleNodes(chatID int64, args string) {
	args = strings.TrimSpace(args)
	if args == "" {
		s.sendNodesHelp(chatID)
		return
	}

	parts := strings.SplitN(args, " ", 3)
	action := strings.ToLower(parts[0])

	switch action {
	case "list":
		s.sendNodesList(chatID)
	case "status":
		if len(parts) < 2 {
			s.sendText(chatID, "Укажите ноду: /nodes status nl2")
			return
		}
		s.sendNodeStatus(chatID, parts[1])
	case "ping":
		if len(parts) < 2 {
			s.sendText(chatID, "Укажите ноду или all:\n/nodes ping nl3\n/nodes ping all")
			return
		}
		target := strings.ToLower(strings.TrimSpace(parts[1]))
		if target == "all" || target == "*" {
			s.pingAllNodes(chatID)
			return
		}
		s.pingNode(chatID, parts[1])
	case "paths":
		if len(parts) < 3 {
			s.sendText(chatID, "Использование: /nodes paths list nl2")
			return
		}
		s.handleNodePaths(chatID, strings.ToLower(parts[1]), strings.TrimSpace(parts[2]))
	default:
		s.sendNodesHelp(chatID)
	}
}

func (s *Service) handleNodePaths(chatID int64, action, rest string) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		s.sendText(chatID, "Использование:\n/nodes paths list nl2\n/nodes paths add nl2 /etc/foo\n/nodes paths remove nl2 /etc/foo")
		return
	}

	sub := strings.SplitN(rest, " ", 2)
	nodeName := sub[0]
	pathArg := ""
	if len(sub) > 1 {
		pathArg = strings.TrimSpace(sub[1])
	}

	switch action {
	case "list":
		s.sendNodePathsList(chatID, nodeName)
	case "add":
		if pathArg == "" {
			s.sendText(chatID, "Укажите путь: /nodes paths add "+nodeName+" /etc/foo")
			return
		}
		if err := s.cfg.AddNodePath(nodeName, pathArg); err != nil {
			s.sendText(chatID, "Ошибка: "+err.Error())
			return
		}
		s.sendText(chatID, fmt.Sprintf("Добавлено на %s: %s", nodeName, pathArg))
		s.sendNodePathsList(chatID, nodeName)
	case "remove", "rm", "del":
		if pathArg == "" {
			s.sendText(chatID, "Укажите путь: /nodes paths remove "+nodeName+" /etc/foo")
			return
		}
		if err := s.cfg.RemoveNodePath(nodeName, pathArg); err != nil {
			s.sendText(chatID, "Ошибка: "+err.Error())
			return
		}
		s.sendText(chatID, fmt.Sprintf("Удалено с %s: %s", nodeName, pathArg))
		s.sendNodePathsList(chatID, nodeName)
	default:
		s.sendText(chatID, "Неизвестное действие. /nodes paths list "+nodeName)
	}
}

func (s *Service) sendNodesHelp(chatID int64) {
	s.sendText(chatID, "Ноды (добавление ноды — в config.yaml):\n\n/nodes list\n/nodes status nl2\n/nodes ping nl3 — одна нода\n/nodes ping all — все ноды\n/nodes paths list nl2\n/nodes paths add nl2 /etc/foo\n/nodes paths remove nl2 /etc/foo")
}

func (s *Service) pingAllNodes(chatID int64) {
	if len(s.cfg.Nodes) == 0 {
		s.sendText(chatID, "Ноды не настроены")
		return
	}

	var b strings.Builder
	b.WriteString("Проверка нод:\n\n")
	for _, node := range s.cfg.Nodes {
		b.WriteString(s.pingNodeLine(node))
		b.WriteString("\n")
	}
	s.sendText(chatID, b.String())
}

func (s *Service) pingNodeLine(node config.NodeConfig) string {
	switch node.NormalizedMode() {
	case config.NodeModeSSH:
		if err := remote.Ping(node); err != nil {
			return fmt.Sprintf("SSH %s: offline (%v)", node.Name, err)
		}
		return fmt.Sprintf("SSH %s: online", node.Name)
	case config.NodeModeAgent:
		if s.agentReg == nil {
			return fmt.Sprintf("Agent %s: проверка недоступна", node.Name)
		}
		if err := agent.PingAgentNode(s.cfg, s.agentReg, node.Name); err != nil {
			return fmt.Sprintf("Agent %s: offline (%v)", node.Name, err)
		}
		return fmt.Sprintf("Agent %s: online", node.Name)
	default:
		return fmt.Sprintf("%s: неизвестный режим", node.Name)
	}
}

func (s *Service) pingNode(chatID int64, nodeName string) {
	node, _, err := s.cfg.FindNode(nodeName)
	if err != nil {
		s.sendText(chatID, "Ошибка: "+err.Error())
		return
	}

	if node.NormalizedMode() == config.NodeModeAgent && s.agentReg == nil {
		s.sendText(chatID, "Проверка agent недоступна")
		return
	}

	s.sendText(chatID, s.pingNodeLine(node))
}

func (s *Service) sendNodesList(chatID int64) {
	if len(s.cfg.Nodes) == 0 {
		s.sendText(chatID, "Ноды не настроены. Добавьте секцию nodes в config.yaml")
		return
	}

	var b strings.Builder
	b.WriteString("Ноды:\n\n")
	for _, node := range s.cfg.Nodes {
		b.WriteString(s.formatNodeSummary(node))
		b.WriteString("\n")
	}
	s.sendText(chatID, b.String())
}

func (s *Service) sendNodeStatus(chatID int64, nodeName string) {
	node, _, err := s.cfg.FindNode(nodeName)
	if err != nil {
		s.sendText(chatID, "Ошибка: "+err.Error())
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Нода: %s\n", node.Name))
	b.WriteString(fmt.Sprintf("Режим: %s\n", node.Mode))
	b.WriteString(s.formatNodeSummary(node))
	b.WriteString("\nПути:\n")

	switch node.NormalizedMode() {
	case config.NodeModeSSH:
		client, err := remote.Connect(node)
		if err != nil {
			b.WriteString(fmt.Sprintf("SSH: недоступна (%v)\n", err))
		} else {
			b.WriteString("SSH: ok\n")
			for _, st := range client.InspectPaths(node.Paths) {
				b.WriteString(formatRemotePathStatus(st) + "\n")
			}
			_ = client.Close()
		}
	case config.NodeModeAgent:
		if s.agentReg != nil {
			if state, ok := s.agentReg.Get(node.Name); ok && !state.LastSeen.IsZero() {
				b.WriteString(fmt.Sprintf("Последний контакт: %s\n", state.LastSeen.Format("2006-01-02 15:04:05")))
				if !state.LastUpload.IsZero() {
					b.WriteString(fmt.Sprintf("Последняя загрузка: %s\n", state.LastUpload.Format("2006-01-02 15:04:05")))
				}
				if state.Version != "" {
					b.WriteString(fmt.Sprintf("Версия агента: %s\n", state.Version))
				}
				if state.LastError != "" {
					b.WriteString(fmt.Sprintf("Ошибка: %s\n", state.LastError))
				}
			}
		}
		for _, st := range backup.InspectPaths(node.Paths) {
			b.WriteString(formatPathStatus(st) + " (локально на master staging)\n")
		}
		staging := s.cfg.StagingDir(node.Name)
		if _, err := os.Stat(staging); err == nil {
			b.WriteString(fmt.Sprintf("Staging: %s\n", staging))
		} else {
			b.WriteString("Staging: пусто (агент ещё не загружал)\n")
		}
	}

	s.sendText(chatID, b.String())
}

func (s *Service) sendNodePathsList(chatID int64, nodeName string) {
	node, _, err := s.cfg.FindNode(nodeName)
	if err != nil {
		s.sendText(chatID, "Ошибка: "+err.Error())
		return
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Пути ноды %s:\n\n", node.Name))
	if len(node.Paths) == 0 {
		b.WriteString("(пусто)\n")
		s.sendText(chatID, b.String())
		return
	}

	switch node.NormalizedMode() {
	case config.NodeModeSSH:
		client, err := remote.Connect(node)
		if err != nil {
			for _, p := range node.Paths {
				b.WriteString(fmt.Sprintf("? — %s (ssh недоступен)\n", p))
			}
		} else {
			for _, st := range client.InspectPaths(node.Paths) {
				b.WriteString(formatRemotePathStatus(st) + "\n")
			}
			_ = client.Close()
		}
	default:
		for _, p := range node.Paths {
			b.WriteString(fmt.Sprintf("cfg — %s\n", p))
		}
	}

	s.sendText(chatID, b.String())
}

func (s *Service) formatNodeSummary(node config.NodeConfig) string {
	summary := fmt.Sprintf("%s [%s] — %d путей", node.Name, node.Mode, len(node.Paths))
	switch node.NormalizedMode() {
	case config.NodeModeSSH:
		if err := remote.Ping(node); err != nil {
			return summary + ", ssh: offline"
		}
		return summary + ", ssh: online"
	case config.NodeModeAgent:
		if s.agentReg != nil && s.agentReg.IsOnline(node.Name, agentOnlineWindow) {
			if s.agentReg.HasWaiter(node.Name) {
				return summary + ", agent: online (подключён)"
			}
			return summary + ", agent: online"
		}
		return summary + ", agent: offline (/nodes ping)"
	}
	return summary
}

func formatPathStatus(st backup.PathStatus) string {
	return formatPathState(st.Exists, st.IsDir, st.Path)
}

func formatRemotePathStatus(st remote.PathStatus) string {
	return formatPathState(st.Exists, st.IsDir, st.Path)
}

func formatPathState(exists, isDir bool, path string) string {
	state := "ok"
	if !exists {
		state = "missing"
	} else if isDir {
		state = "dir"
	}
	return fmt.Sprintf("%s — %s", state, path)
}