package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIDocDiffReadsExplicitDiffFile(t *testing.T) {
	root := t.TempDir()
	diffPath := writeAPIDocHelperTestFile(t, root, "api.diff", "diff --git a/api/openapi.yaml b/api/openapi.yaml\n")

	diff, err := apiDocDiff(root, []string{"api/openapi.yaml"}, diffPath)

	if err != nil {
		t.Fatalf("apiDocDiff explicit file failed: %v", err)
	}
	if !strings.Contains(diff, "api/openapi.yaml") {
		t.Fatalf("unexpected explicit diff content:\n%s", diff)
	}
}

func TestAPIDocDiffReadsStagedCandidateDiff(t *testing.T) {
	root := t.TempDir()
	runGitForContract(t, root, "init")
	runGitForContract(t, root, "config", "user.email", "test@example.com")
	runGitForContract(t, root, "config", "user.name", "Tester")
	writeAPIDocHelperTestFile(t, root, "api/openapi.yaml", "openapi: 3.0.0\n")
	runGitForContract(t, root, "add", "api/openapi.yaml")

	diff, err := apiDocDiff(root, []string{"api/openapi.yaml"}, "")

	if err != nil {
		t.Fatalf("apiDocDiff staged diff failed: %v", err)
	}
	if !strings.Contains(diff, "openapi: 3.0.0") || !strings.Contains(diff, "api/openapi.yaml") {
		t.Fatalf("unexpected staged diff:\n%s", diff)
	}
}

func TestMustJSONReturnsIndentedJSON(t *testing.T) {
	out := mustJSON(map[string]any{"ok": true, "items": []string{"one"}})

	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("mustJSON produced invalid JSON: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "\n  \"items\": [") {
		t.Fatalf("expected indented JSON, got:\n%s", string(out))
	}
}

func TestPrintAPIDocReviewPrintsSummaryAndFindings(t *testing.T) {
	line := 42
	out := captureStatusVerifyStdout(t, func() error {
		printAPIDocReview(apiDocReviewResult{
			Verdict: "fail",
			Summary: "missing response docs",
			Findings: []apiDocReviewFinding{
				{File: "src/users.controller.ts", Line: &line, Severity: "blocking", Message: "missing 404 response"},
				{File: "openapi.yaml", Severity: "warning", Message: "description is terse"},
			},
		})
		return nil
	})

	for _, want := range []string{
		"API doc review verdict: fail",
		"missing response docs",
		"- [blocking] src/users.controller.ts:42 missing 404 response",
		"- [warning] openapi.yaml description is terse",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("printAPIDocReview output missing %q:\n%s", want, out)
		}
	}
}

func writeAPIDocHelperTestFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
