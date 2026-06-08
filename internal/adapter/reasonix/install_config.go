package reasonix

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

func writeReasonixMCPConfig(req port.NativeInstallRequest) (port.InstallFile, error) {
	configDir := reasonixConfigDir(req.Home)
	configPath := filepath.Join(configDir, "config.toml")
	file := port.InstallFile{Path: configPath, Kind: "reasonix_user_mcp_config"}

	if req.DryRun {
		file.WouldWrite = true
		return file, nil
	}

	text := ""
	if b, err := os.ReadFile(configPath); err == nil {
		text = string(b)
		backup := configPath + ".harness.bak"
		if _, statErr := os.Stat(backup); os.IsNotExist(statErr) {
			if writeErr := os.WriteFile(backup, []byte(text), 0o600); writeErr != nil {
				return file, writeErr
			}
		}
	} else if !os.IsNotExist(err) {
		return file, err
	}

	text = removeReasonixPluginSection(text)
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += reasonixPluginBlock(req)

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(configPath, []byte(text), 0o600); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func reasonixPluginBlock(req port.NativeInstallRequest) string {
	return fmt.Sprintf("[[plugins]]\nname = %s\ncommand = %s\nargs = [\"mcp\"]\n",
		installutil.TOMLString("agent_harness_project"),
		installutil.TOMLString(req.BinPath),
	)
}

func reasonixConfigDir(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "reasonix")
	}
	return filepath.Join(home, ".config", "reasonix")
}

func removeReasonixPluginSection(src string) string {
	marker := "[[plugins]]"
	for {
		pos := strings.Index(src, marker)
		if pos < 0 {
			return src
		}
		next := strings.Index(src[pos+len(marker):], "\n[[")
		if next < 0 {
			nextEnd := strings.Index(src[pos+len(marker):], "\n\n")
			if nextEnd < 0 {
				src = strings.TrimRight(src[:pos], " \t\r\n") + "\n"
				continue
			}
			src = src[:pos] + src[pos+len(marker)+nextEnd+2:]
			continue
		}
		nextPos := pos + len(marker) + next + 1
		candidate := strings.TrimSpace(src[pos : pos+len(marker)+next])
		if strings.Contains(candidate, "agent_harness") {
			src = src[:pos] + src[nextPos:]
		} else {
			return src
		}
	}
}
