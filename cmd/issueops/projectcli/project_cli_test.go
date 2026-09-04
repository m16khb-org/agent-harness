package projectcli

import (
	"encoding/json"
	projectdocs "issueops/internal/contract/projectdocs"
	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdoc "issueops/internal/domain/projectdoc"
	"issueops/internal/testsupport"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunProject_dispatchesBootstrapText_whenRepoIsPositional(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return Run([]string{"bootstrap", "--dry-run", repo})
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
		return Run([]string{"docs", "--repo", repo, "--json"})
	})

	// Then
	var result projectdocs.ProjectDocsRouteResult
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
		return RunRouteDocs([]string{"--repo", repo, "--json", "architecture", "test"})
	})

	// Then
	var result projectdocs.ProjectDocsRouteResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode route-docs json: %v\n%s", err, out)
	}
	if result.Task != "architecture test" {
		t.Fatalf("expected joined task args, got %q", result.Task)
	}
	if !projectRouteHasRel(result, filepath.ToSlash(filepath.Join(projectdoc.ProjectDocsDir, "TESTING.md"))) {
		t.Fatalf("expected testing doc route, got %+v", result.Docs)
	}
}

func TestRunProjectRecord_recordsADR_whenRequiredFieldsAreProvided(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return RunRecord([]string{
			"--repo", repo,
			"--kind", "adr",
			"--title", "Keep project CLI thin",
			"--summary", "Project CLI delegates to core project-docs operations.",
			"--decision", "Move wrapper code into a focused file.",
			"--json",
		})
	})

	// Then
	var result projectdocscontract.ProjectDocsAppendResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode record json: %v\n%s", err, out)
	}
	if result.RecordKind != "adr" || !strings.HasPrefix(result.RelPath, ".issueops/adr/") || !strings.HasSuffix(result.RelPath, "-keep-project-cli-thin.md") {
		t.Fatalf("unexpected record result: %+v", result)
	}
	adr, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RelPath)))
	if err != nil {
		t.Fatalf("read ADR record: %v", err)
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
			stderr, err := captureProjectCLIStderr(t, func() error {
				return Run(tt.args)
			})

			// Then
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
			if !strings.Contains(stderr, "issueops project bootstrap") {
				t.Fatalf("expected usage on stderr, got:\n%s", stderr)
			}
		})
	}
}

func TestRunProjectRecord_returnsValidationError_whenTitleIsMissing(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	err := RunRecord([]string{"--repo", repo, "--kind", "caution", "--summary", "summary"})

	// Then
	if err == nil || !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("expected title validation error, got %v", err)
	}
}

func projectRouteHasRel(result projectdocs.ProjectDocsRouteResult, rel string) bool {
	for _, doc := range result.Docs {
		if doc.RelPath == rel {
			return true
		}
	}
	return false
}

func captureProjectCLIStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStderrAndError(t, fn)
}
