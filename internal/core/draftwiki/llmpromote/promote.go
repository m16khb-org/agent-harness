package llmpromote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Draft struct {
	Title   string
	RelPath string
	Path    string
	Summary string
}

type Request struct {
	RepoRoot          string
	Draft             Draft
	TargetWiki        string
	TargetType        string
	LLMWikiConfigPath string
}

type Result struct {
	WikiRoot string
	RawPath  string
	RawRel   string
	LogPath  string
}

func Promote(req Request) (Result, error) {
	wikiRoot, err := resolveLLMWikiRoot(req.LLMWikiConfigPath, req.TargetWiki)
	if err != nil {
		return Result{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	if !isLLMWikiRawType(targetType) {
		return Result{}, fmt.Errorf("unsupported llm-wiki raw type %q", targetType)
	}
	bodyBytes, err := os.ReadFile(req.Draft.Path)
	if err != nil {
		return Result{}, err
	}
	today := time.Now().Format(time.DateOnly)
	rawDir := filepath.Join(wikiRoot, "raw", targetType)
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return Result{}, err
	}
	rawName := RawFileName(today, req.Draft.Path)
	rawPath := filepath.Join(rawDir, rawName)
	if _, err := os.Stat(rawPath); err == nil {
		return Result{}, fmt.Errorf("llm-wiki raw file already exists: %s", rawPath)
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}
	rawRel, err := filepath.Rel(wikiRoot, rawPath)
	if err != nil {
		return Result{}, err
	}
	rawRel = filepath.ToSlash(rawRel)
	raw := RawNoteContent(req.Draft, targetType, today, string(bodyBytes))
	if err := os.WriteFile(rawPath, []byte(raw), 0o644); err != nil {
		return Result{}, err
	}
	logPath := filepath.Join(wikiRoot, "log.md")
	if err := appendLLMWikiPromoteLog(logPath, today, req.Draft.Title, rawRel, req.Draft.RelPath); err != nil {
		return Result{}, err
	}
	return Result{
		WikiRoot: wikiRoot,
		RawPath:  rawPath,
		RawRel:   rawRel,
		LogPath:  logPath,
	}, nil
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
