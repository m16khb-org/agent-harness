package projectdocs

import (
	projectdocscontract "agent-harness/internal/contract/projectdocs"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/repopath"
)

func ReadProjectDoc(repoRoot, relPath string) (projectdocscontract.ProjectDocsReadResult, error) {
	root, err := repopath.NormalizeRoot(repoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(relPath)
	if err != nil {
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	result := projectdocscontract.ProjectDocsReadResult{
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
		return projectdocscontract.ProjectDocsReadResult{}, err
	}
	result.Exists = true
	result.Content = string(b)
	result.SHA256 = sha256Hex(result.Content)
	return result, nil
}

func UpdateProjectDoc(req projectdocscontract.ProjectDocsUpdateRequest) (projectdocscontract.ProjectDocsUpdateResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsUpdateResult{}, err
	}
	rel, err := normalizeProjectDocRelPath(req.RelPath)
	if err != nil {
		return projectdocscontract.ProjectDocsUpdateResult{}, err
	}
	content := strings.TrimRight(req.Content, "\n") + "\n"
	if strings.TrimSpace(content) == "" {
		return projectdocscontract.ProjectDocsUpdateResult{}, fmt.Errorf("content is required")
	}
	summary := strings.TrimSpace(req.Summary)
	if summary == "" {
		return projectdocscontract.ProjectDocsUpdateResult{}, fmt.Errorf("summary is required")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	current := ""
	currentSHA := ""
	if b, err := os.ReadFile(path); err == nil {
		current = string(b)
		currentSHA = sha256Hex(current)
	} else if !os.IsNotExist(err) {
		return projectdocscontract.ProjectDocsUpdateResult{}, err
	}
	if currentSHA != "" {
		expected := strings.TrimSpace(req.ExpectedSHA256)
		if expected == "" {
			return projectdocscontract.ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 is required when updating an existing project doc; call project_docs_read first")
		}
		if expected != currentSHA {
			return projectdocscontract.ProjectDocsUpdateResult{}, fmt.Errorf("expected_sha256 mismatch for %s: current %s", rel, currentSHA)
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
			return projectdocscontract.ProjectDocsUpdateResult{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return projectdocscontract.ProjectDocsUpdateResult{}, err
		}
	}
	return projectdocscontract.ProjectDocsUpdateResult{
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
