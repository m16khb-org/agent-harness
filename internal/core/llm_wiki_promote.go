package core

import (
	"fmt"
	"os"
	"path/filepath"
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
