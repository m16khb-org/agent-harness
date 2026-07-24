package basiccli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunDocs_printsJSON_whenJSONFlagIsSet(t *testing.T) {
	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDocs([]string{"--json"})
	})

	// 검증
	var result core.DocsIndexResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode docs json: %v\n%s", err, out)
	}
	if !result.OK || result.Version != deps.Version || result.HarnessRoot == "" {
		t.Fatalf("unexpected docs result: %+v", result)
	}
	if !docsIndexHasRel(result, "AGENTS.md") {
		t.Fatalf("docs index missing AGENTS.md: %+v", result.Docs)
	}
}

func TestRunDocs_acceptsIndexAliasForTextOutput(t *testing.T) {
	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDocs([]string{"index"})
	})

	// 검증
	if !strings.Contains(out, "AGENTS.md") {
		t.Fatalf("docs text should include AGENTS.md:\n%s", out)
	}
	if !strings.Contains(out, " — ") {
		t.Fatalf("docs text should include titled document separator:\n%s", out)
	}
}

func TestRunDocsWithRoot_printsRelPathOnly_whenTitleIsMissing(t *testing.T) {
	// 준비
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("No title here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 실행
	out := captureStatusVerifyStdout(t, func() error {
		return RunDocsWithRoot([]string{}, root)
	})

	// 검증
	got := strings.TrimSpace(out)
	if got != "AGENTS.md" {
		t.Fatalf("expected untitled doc to print rel path only, got %q", got)
	}
}

func TestRunDocsWithRoot_rejectsInvalidFlag(t *testing.T) {
	// 실행
	err := RunDocsWithRoot([]string{"--missing"}, t.TempDir())

	// 검증
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("expected flag parsing error, got %v", err)
	}
}

func docsIndexHasRel(result core.DocsIndexResult, rel string) bool {
	for _, doc := range result.Docs {
		if doc.RelPath == rel {
			return true
		}
	}
	return false
}
