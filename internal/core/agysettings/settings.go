package agysettings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/repopath"
)

type file struct {
	Model string `json:"model"`
}

func ResolvePath(path string) string {
	if strings.TrimSpace(path) != "" {
		return repopath.ExpandLeadingTilde(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
}

func ReadConfiguredModel(settingsPath string) (string, error) {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("read agy settings %s: %w", settingsPath, err)
	}
	var settings file
	if err := json.Unmarshal(b, &settings); err != nil {
		return "", fmt.Errorf("parse agy settings %s: %w", settingsPath, err)
	}
	if strings.TrimSpace(settings.Model) == "" {
		return "", fmt.Errorf("agy settings %s has no model key", settingsPath)
	}
	return settings.Model, nil
}
