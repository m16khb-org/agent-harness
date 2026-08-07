package searchrouting

import (
	"path/filepath"
	"strings"
)

func isShellTool(tool string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "bash", "sh", "zsh", "shell", "exec", "run_command", "shell_command", "unified_exec", "exec_command":
		return true
	default:
		return false
	}
}

func searchTokenName(token string) string {
	cleaned := strings.Trim(token, `"'`)
	return filepath.Base(cleaned)
}
