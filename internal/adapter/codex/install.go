package codex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

type Installer struct{}

func NewInstaller() Installer { return Installer{} }

func (Installer) Name() string { return "codex" }

func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	result := port.HostInstallResult{Host: "codex", OK: true}
	var errs []error

	for _, skillName := range req.SkillNames {
		link, err := installutil.EnsureSymlink(filepath.Join(req.Root, "skills", skillName), filepath.Join(req.CodexHome, "skills", skillName))
		result.Links = append(result.Links, link)
		if err != nil {
			errs = append(errs, err)
		}
	}

	globalConfig := filepath.Join(req.CodexHome, "config.toml")
	file, err := writeGlobalConfig(globalConfig, req)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	templatePath := filepath.Join(req.Root, "configs", "codex", "mcp.config.toml")
	file, err = installutil.WriteText(templatePath, "codex_mcp_template", codexTemplate(req), 0o644)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}

	hookPath := filepath.Join(req.Root, "configs", "codex", "hooks", "session-start-llm-wiki.sh")
	file, err = installutil.WriteText(hookPath, "codex_session_start_hook_template", codexHookTemplate(), 0o755)
	result.Files = append(result.Files, file)
	if err != nil {
		errs = append(errs, err)
	}
	if chmodErr := os.Chmod(hookPath, 0o755); chmodErr != nil {
		errs = append(errs, chmodErr)
	}

	if len(errs) > 0 {
		result.OK = false
		return result, joinErrors(errs)
	}
	return result, nil
}

func writeGlobalConfig(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_mcp_config"}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
		backup := path + ".harness.bak"
		if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
			if writeErr := os.WriteFile(backup, []byte(text), 0o600); writeErr != nil {
				return file, writeErr
			}
		}
	} else if !os.IsNotExist(err) {
		return file, err
	}
	for _, section := range []string{"mcp_servers.agent_harness", "mcp_servers.agent_harness.env"} {
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
LLM_WIKI_ROOT = %s
`, installutil.TOMLString(req.BinPath), installutil.TOMLString(req.Root), installutil.TOMLString(req.LLMWikiRoot))
}

func codexTemplate(req port.NativeInstallRequest) string {
	portableRoot := req.PortableLLMWikiRoot
	if portableRoot == "" {
		portableRoot = "~/workspace/knowledge-base/llm-wiki"
	}
	return fmt.Sprintf(`[mcp_servers.agent_harness]
command = "./bin/harness"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "."
LLM_WIKI_ROOT = %s
`, installutil.TOMLString(portableRoot))
}

func codexHookTemplate() string {
	return `#!/usr/bin/env bash
# Generic plain-text session-start hook adapter for Codex-compatible hook runners.
# It intentionally emits plain context instead of Claude's hookSpecificOutput JSON.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
export HARNESS_ROOT="$ROOT"
export HARNESS_SESSION_CONTEXT_MODE=plain
exec "$ROOT/scripts/session-start-llm-wiki.sh"
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

func joinErrors(errs []error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
