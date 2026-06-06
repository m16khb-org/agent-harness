package projectdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/repopath"
)

func ReadProjectDoc(repoRoot, relPath string) (ProjectDocsReadResult, error) {
	root, err := repopath.NormalizeRoot(repoRoot)
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
	root, err := repopath.NormalizeRoot(req.RepoRoot)
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
