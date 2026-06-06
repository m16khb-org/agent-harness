package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func AppendProjectDocsRecord(req ProjectDocsRecordRequest) (ProjectDocsRecordResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	recordKind := strings.ToLower(strings.TrimSpace(req.Kind))
	recordKind = strings.ReplaceAll(recordKind, "_", "-")
	recordKind = strings.ReplaceAll(recordKind, " ", "-")
	var rel string
	switch recordKind {
	case "caution", "cautions", "false-case", "failure", "problem":
		recordKind = "caution"
		rel = filepath.ToSlash(filepath.Join(ProjectDocsDir, "CAUTIONS.md"))
	case "adr", "decision", "architecture-decision":
		recordKind = "adr"
		rel = filepath.ToSlash(filepath.Join(ProjectDocsDir, "ADR.md"))
	default:
		return ProjectDocsRecordResult{}, fmt.Errorf("unsupported record kind %q: use caution or adr", req.Kind)
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return ProjectDocsRecordResult{}, fmt.Errorf("title is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return ProjectDocsRecordResult{}, fmt.Errorf("summary is required")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ProjectDocsRecordResult{}, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		seed := "# " + map[string]string{"caution": "Cautions", "adr": "Architecture Decision Records"}[recordKind] + "\n\n"
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			return ProjectDocsRecordResult{}, err
		}
	}
	entry := renderProjectDocsRecordEntry(recordKind, req, time.Now())
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	if _, err := f.WriteString(entry); err != nil {
		_ = f.Close()
		return ProjectDocsRecordResult{}, err
	}
	if err := f.Close(); err != nil {
		return ProjectDocsRecordResult{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectDocsRecordResult{}, err
	}
	return ProjectDocsRecordResult{
		OK:            true,
		Kind:          "project_docs_record",
		RecordKind:    recordKind,
		RepoRoot:      root,
		RelPath:       rel,
		Path:          path,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		BytesAppended: len([]byte(entry)),
		SHA256:        sha256Hex(string(b)),
	}, nil
}
