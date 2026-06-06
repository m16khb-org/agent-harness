package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type agySettingsFile struct {
	Model string `json:"model"`
}

func resolveRepoFile(root, candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", fmt.Errorf("input path is required")
	}
	path := candidate
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("input path escapes repo root: %s", candidate)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("input path is a directory: %s", candidate)
	}
	return abs, nil
}

func resolveAgySettingsPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return expandLeadingTilde(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
}

func readAgyConfiguredModel(settingsPath string) (string, error) {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("read agy settings %s: %w", settingsPath, err)
	}
	var settings agySettingsFile
	if err := json.Unmarshal(b, &settings); err != nil {
		return "", fmt.Errorf("parse agy settings %s: %w", settingsPath, err)
	}
	if strings.TrimSpace(settings.Model) == "" {
		return "", fmt.Errorf("agy settings %s has no model key", settingsPath)
	}
	return settings.Model, nil
}
