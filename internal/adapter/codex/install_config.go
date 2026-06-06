package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func writeGlobalConfig(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_mcp_config"}
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
		if !req.DryRun {
			backup := path + ".harness.bak"
			if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
				if writeErr := os.WriteFile(backup, []byte(text), 0o600); writeErr != nil {
					return file, writeErr
				}
			}
		}
	} else if !os.IsNotExist(err) && !req.DryRun {
		return file, err
	}
	for _, section := range []string{"mcp_servers.agent_harness", "mcp_servers.agent_harness.env", "mcp_servers.agent-harness", "mcp_servers.agent-harness.env"} {
		text = removeTOMLSection(text, section)
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n\n") {
		text += "\n"
	}
	text += codexGlobalBlock(req)
	existing, _ := os.ReadFile(path)
	if string(existing) == text {
		return file, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func codexGlobalBlock(req port.NativeInstallRequest) string {
	return fmt.Sprintf(`[mcp_servers.agent_harness]
command = %s
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = %s
`, installutil.TOMLString(req.BinPath), installutil.TOMLString(req.Root))
}

func codexTemplate(req port.NativeInstallRequest) string {
	return `[mcp_servers.agent_harness]
command = "./bin/agent-harness"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "."
`
}

func removeTOMLSection(src, section string) string {
	marker := "[" + section + "]"
	for {
		pos := strings.Index(src, marker)
		if pos < 0 {
			return src
		}
		next := strings.Index(src[pos+len(marker):], "\n[")
		if next < 0 {
			src = strings.TrimRight(src[:pos], " \t\r\n") + "\n"
			continue
		}
		nextPos := pos + len(marker) + next + 1
		src = src[:pos] + src[nextPos:]
	}
}
