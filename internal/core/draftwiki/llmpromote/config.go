package llmpromote

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/repopath"
)

type llmWikiConfigFile struct {
	HubPath      string `json:"hub_path"`
	ResolvedPath string `json:"resolved_path"`
}

type llmWikiRegistryFile struct {
	Default string                         `json:"default"`
	Wikis   map[string]llmWikiRegistryWiki `json:"wikis"`
}

type llmWikiRegistryWiki struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func resolveLLMWikiRoot(configPath, targetWiki string) (string, error) {
	hub, err := resolveLLMWikiHub(configPath)
	if err != nil {
		return "", err
	}
	registryPath := filepath.Join(hub, "wikis.json")
	b, err := os.ReadFile(registryPath)
	if err != nil {
		return "", fmt.Errorf("read llm-wiki registry %s: %w", registryPath, err)
	}
	var registry llmWikiRegistryFile
	if err := json.Unmarshal(b, &registry); err != nil {
		return "", fmt.Errorf("parse llm-wiki registry %s: %w", registryPath, err)
	}
	if strings.TrimSpace(targetWiki) == "" {
		targetWiki = registry.Default
	}
	if targetWiki == "" {
		return "", fmt.Errorf("target wiki is required")
	}
	entry, ok := registry.Wikis[targetWiki]
	if !ok {
		fallback := filepath.Join(hub, "topics", targetWiki)
		if _, err := os.Stat(fallback); err == nil {
			return fallback, nil
		}
		return "", fmt.Errorf("target wiki %q not found in %s", targetWiki, registryPath)
	}
	root := entry.Path
	switch {
	case root == "", root == "<HUB>":
		root = hub
	case strings.HasPrefix(root, "~/"):
		root = repopath.ExpandLeadingTilde(root)
	case filepath.IsAbs(root):
	default:
		root = filepath.Join(hub, filepath.FromSlash(root))
	}
	if _, err := os.Stat(root); err != nil {
		return "", fmt.Errorf("target wiki path %s: %w", root, err)
	}
	return root, nil
}

func resolveLLMWikiHub(configPath string) (string, error) {
	if strings.TrimSpace(configPath) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configPath = filepath.Join(home, ".config", "llm-wiki", "config.json")
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("read llm-wiki config %s: %w", configPath, err)
	}
	var cfg llmWikiConfigFile
	if err := json.Unmarshal(b, &cfg); err != nil {
		return "", fmt.Errorf("parse llm-wiki config %s: %w", configPath, err)
	}
	hub := strings.TrimSpace(cfg.HubPath)
	if hub == "" {
		hub = strings.TrimSpace(cfg.ResolvedPath)
	}
	if hub == "" {
		return "", fmt.Errorf("llm-wiki config %s has no hub_path or resolved_path", configPath)
	}
	hub = repopath.ExpandLeadingTilde(hub)
	if _, err := os.Stat(hub); err != nil {
		return "", fmt.Errorf("llm-wiki hub path %s: %w", hub, err)
	}
	return hub, nil
}
