package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type promoteDraftWikiToLLMWikiRequest struct {
	RepoRoot          string
	Draft             DraftWikiDraft
	TargetWiki        string
	TargetType        string
	LLMWikiConfigPath string
}

type promoteDraftWikiToLLMWikiResult struct {
	WikiRoot string
	RawPath  string
	RawRel   string
	LogPath  string
}

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

func promoteDraftWikiToLLMWiki(req promoteDraftWikiToLLMWikiRequest) (promoteDraftWikiToLLMWikiResult, error) {
	wikiRoot, err := resolveLLMWikiRoot(req.LLMWikiConfigPath, req.TargetWiki)
	if err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	if !isLLMWikiRawType(targetType) {
		return promoteDraftWikiToLLMWikiResult{}, fmt.Errorf("unsupported llm-wiki raw type %q", targetType)
	}
	bodyBytes, err := os.ReadFile(req.Draft.Path)
	if err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	today := time.Now().Format(time.DateOnly)
	rawDir := filepath.Join(wikiRoot, "raw", targetType)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	rawName := draftWikiRawFileName(today, req.Draft.Path)
	rawPath := filepath.Join(rawDir, rawName)
	if _, err := os.Stat(rawPath); err == nil {
		return promoteDraftWikiToLLMWikiResult{}, fmt.Errorf("llm-wiki raw file already exists: %s", rawPath)
	} else if !os.IsNotExist(err) {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	rawRel, err := filepath.Rel(wikiRoot, rawPath)
	if err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	rawRel = filepath.ToSlash(rawRel)
	raw := llmWikiRawNoteContent(req.Draft, targetType, today, string(bodyBytes))
	if err := os.WriteFile(rawPath, []byte(raw), 0o644); err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	logPath := filepath.Join(wikiRoot, "log.md")
	if err := appendLLMWikiPromoteLog(logPath, today, req.Draft.Title, rawRel, req.Draft.RelPath); err != nil {
		return promoteDraftWikiToLLMWikiResult{}, err
	}
	return promoteDraftWikiToLLMWikiResult{
		WikiRoot: wikiRoot,
		RawPath:  rawPath,
		RawRel:   rawRel,
		LogPath:  logPath,
	}, nil
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
		root = expandLeadingTilde(root)
	case filepath.IsAbs(root):
		// already absolute
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
	hub = expandLeadingTilde(hub)
	if _, err := os.Stat(hub); err != nil {
		return "", fmt.Errorf("llm-wiki hub path %s: %w", hub, err)
	}
	return hub, nil
}

func expandLeadingTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(path, "~/")))
		}
	}
	return filepath.FromSlash(path)
}

func isLLMWikiRawType(rawType string) bool {
	switch rawType {
	case "articles", "papers", "repos", "notes", "data":
		return true
	default:
		return false
	}
}

func draftWikiRawFileName(today, draftPath string) string {
	base := strings.TrimSuffix(filepath.Base(draftPath), filepath.Ext(draftPath))
	base = slugifyDraftWiki(base)
	if base == "" {
		base = "draft-wiki-note"
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}-`, base); matched {
		return base + ".md"
	}
	return today + "-" + base + ".md"
}

func llmWikiRawNoteContent(draft DraftWikiDraft, targetType, today, draftContent string) string {
	body := strings.TrimSpace(stripDraftWikiFrontmatter(draftContent))
	if body == "" {
		body = "# " + draft.Title + "\n"
	}
	summary := draft.Summary
	if summary == "" {
		summary = "Approved agent-harness draft wiki note promoted from repo-local review staging."
	}
	return fmt.Sprintf(`---
title: %q
source: %q
type: %s
ingested: %s
tags: [agent-harness, draft-wiki]
summary: %q
original_draft: %q
---

%s
`, draft.Title, "agent-harness draft-wiki:"+draft.RelPath, targetType, today, summary, draft.RelPath, body)
}

func stripDraftWikiFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

func appendLLMWikiPromoteLog(logPath, today, title, rawRel, draftRel string) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	entry := fmt.Sprintf("\n## [%s] ingest | %s (%s)\n\n- Source: agent-harness draft-wiki %s\n", today, title, rawRel, draftRel)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
