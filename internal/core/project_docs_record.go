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

func ReadProjectDoc(repoRoot, relPath string) (ProjectDocsReadResult, error) {
	root, err := normalizeRepoRoot(repoRoot)
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(relPath)
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	result := ProjectDocsReadResult{
		OK:          true,
		Kind:        "project_docs_read",
		RepoRoot:    root,
		RelPath:     rel,
		Path:        path,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		result.Exists = false
		result.Warnings = []string{"document_missing: run project_docs_bootstrap_plan or agent-harness project bootstrap first"}
		return result, nil
	}
	if err != nil {
		return ProjectDocsReadResult{}, err
	}
	result.Exists = true
	result.Content = string(b)
	result.SHA256 = sha256Hex(result.Content)
	return result, nil
}

func UpdateProjectDoc(req ProjectDocsUpdateRequest) (ProjectDocsUpdateResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return ProjectDocsUpdateResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(req.RelPath)
	if err != nil {
		return ProjectDocsUpdateResult{}, err
	}
	content := strings.TrimRight(req.Content, "\n") + "\n"
	if strings.TrimSpace(content) == "" {
		return ProjectDocsUpdateResult{}, fmt.Errorf("content is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return ProjectDocsUpdateResult{}, fmt.Errorf("summary is required")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	current := ""
	currentSHA := ""
	if b, err := os.ReadFile(path); err == nil {
		current = string(b)
		currentSHA = sha256Hex(current)
	} else if !os.IsNotExist(err) {
		return ProjectDocsUpdateResult{}, err
	}
	if currentSHA != "" {
		expected := strings.TrimSpace(req.ExpectedSHA256)
		if expected == "" {
			return ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 is required when updating an existing project doc; call project_docs_read first")
		}
		if expected != currentSHA {
			return ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 mismatch for %s: current %s", rel, currentSHA)
		}
	}
	action := "create"
	if current != "" {
		action = plannedFileAction(path, content)
	}
	nextSHA := sha256Hex(content)
	warnings := []string{}
	if !req.Confirm {
		warnings = append(warnings, "dry_run_only: pass confirm=true to write the updated .agent-harness document")
	} else if action != "unchanged" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return ProjectDocsUpdateResult{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return ProjectDocsUpdateResult{}, err
		}
	}
	return ProjectDocsUpdateResult{
		OK:            true,
		Kind:          "project_docs_update",
		RepoRoot:      root,
		RelPath:       rel,
		Path:          path,
		Action:        action,
		Confirmed:     req.Confirm,
		DryRun:        !req.Confirm,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		CurrentSHA256: currentSHA,
		NextSHA256:    nextSHA,
		Bytes:         len([]byte(content)),
		Summary:       summary,
		Evidence:      nonEmptyStrings(req.Evidence),
		Warnings:      warnings,
	}, nil
}

func renderProjectDocsRecordEntry(kind string, req ProjectDocsRecordRequest, now time.Time) string {
	var b strings.Builder
	stamp := now.Format("2006-01-02")
	fmt.Fprintf(&b, "\n## %s — %s\n\n", stamp, strings.TrimSpace(req.Title))
	fmt.Fprintf(&b, "- Kind: `%s`\n", kind)
	if source := strings.TrimSpace(req.Source); source != "" {
		fmt.Fprintf(&b, "- Source: %s\n", source)
	}
	fmt.Fprintf(&b, "- Summary: %s\n", strings.TrimSpace(req.Summary))
	if v := strings.TrimSpace(req.Context); v != "" {
		fmt.Fprintf(&b, "- Context: %s\n", v)
	}
	if kind == "caution" {
		if v := strings.TrimSpace(req.Resolution); v != "" {
			fmt.Fprintf(&b, "- Resolution: %s\n", v)
		}
	} else {
		if v := strings.TrimSpace(req.Decision); v != "" {
			fmt.Fprintf(&b, "- Decision: %s\n", v)
		}
		if v := strings.TrimSpace(req.Consequences); v != "" {
			fmt.Fprintf(&b, "- Consequences: %s\n", v)
		}
	}
	if len(req.Evidence) > 0 {
		b.WriteString("- Evidence:\n")
		for _, ev := range req.Evidence {
			if ev = strings.TrimSpace(ev); ev != "" {
				fmt.Fprintf(&b, "  - %s\n", ev)
			}
		}
	}
	if len(req.Alternatives) > 0 {
		b.WriteString("- Alternatives / rejected options:\n")
		for _, alt := range req.Alternatives {
			if alt = strings.TrimSpace(alt); alt != "" {
				fmt.Fprintf(&b, "  - %s\n", alt)
			}
		}
	}
	return b.String()
}
