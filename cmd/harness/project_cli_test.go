package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunProject_dispatchesBootstrapText_whenRepoIsPositional(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runProject([]string{"bootstrap", "--dry-run", repo})
	})

	// Then
	if !strings.Contains(out, "project docs would update") {
		t.Fatalf("bootstrap text should describe dry-run plan, got:\n%s", out)
	}
	if !strings.Contains(out, "lifecycle state:") {
		t.Fatalf("bootstrap text should include lifecycle state, got:\n%s", out)
	}
}

func TestRunProjectDocs_printsRouteJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runProject([]string{"docs", "--repo", repo, "--json"})
	})

	// Then
	var result core.ProjectDocsRouteResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode project docs json: %v\n%s", err, out)
	}
	if result.Kind != "project_docs_route" || result.Task != "general" {
		t.Fatalf("unexpected route result: %+v", result)
	}
	if len(result.Docs) == 0 {
		t.Fatal("expected routed docs")
	}
}

func TestRunProjectRouteDocs_joinsTaskArgs_whenTaskFlagIsOmitted(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectRouteDocs([]string{"--repo", repo, "--json", "architecture", "test"})
	})

	// Then
	var result core.ProjectDocsRouteResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode route-docs json: %v\n%s", err, out)
	}
	if result.Task != "architecture test" {
		t.Fatalf("expected joined task args, got %q", result.Task)
	}
	if !projectRouteHasRel(result, filepath.ToSlash(filepath.Join(core.ProjectDocsDir, "TESTING.md"))) {
		t.Fatalf("expected testing doc route, got %+v", result.Docs)
	}
}

func TestRunProjectRecord_recordsADR_whenRequiredFieldsAreProvided(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectRecord([]string{
			"--repo", repo,
			"--kind", "adr",
			"--title", "Keep project CLI thin",
			"--summary", "Project CLI delegates to core project-docs operations.",
			"--decision", "Move wrapper code into a focused file.",
			"--json",
		})
	})

	// Then
	var result core.ProjectDocsRecordResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode record json: %v\n%s", err, out)
	}
	if result.RecordKind != "adr" || result.RelPath != ".agent-harness/ADR.md" {
		t.Fatalf("unexpected record result: %+v", result)
	}
	adr, err := os.ReadFile(filepath.Join(repo, ".agent-harness", "ADR.md"))
	if err != nil {
		t.Fatalf("read ADR: %v", err)
	}
	if !strings.Contains(string(adr), "Keep project CLI thin") {
		t.Fatalf("expected ADR entry to be appended, got:\n%s", string(adr))
	}
}

func TestRunProject_rejectsMissingAndUnknownSubcommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing", args: nil, wantErr: "missing project subcommand"},
		{name: "unknown", args: []string{"missing-command"}, wantErr: `unknown project subcommand "missing-command"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			stderr, err := captureProjectCLIStderr(func() error {
				return runProject(tt.args)
			})

			// Then
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
			if !strings.Contains(stderr, "agent-harness project bootstrap") {
				t.Fatalf("expected usage on stderr, got:\n%s", stderr)
			}
		})
	}
}

func TestRunProjectRecord_returnsValidationError_whenTitleIsMissing(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	err := runProjectRecord([]string{"--repo", repo, "--kind", "caution", "--summary", "summary"})

	// Then
	if err == nil || !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("expected title validation error, got %v", err)
	}
}

func projectRouteHasRel(result core.ProjectDocsRouteResult, rel string) bool {
	for _, doc := range result.Docs {
		if doc.RelPath == rel {
			return true
		}
	}
	return false
}

func captureProjectCLIStderr(fn func() error) (string, error) {
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	os.Stderr = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		return "", closeErr
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		return "", err
	}
	return out.String(), callErr
}
