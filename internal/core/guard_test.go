package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGuardCheckBlocksPortableTestAntiPatterns(t *testing.T) {
	repo := initGuardRepo(t)
	testName := "Test" + "Works"
	sleepCall := "time." + "Sl" + "eep(time.Second)"
	externalURL := "http" + "s://api.example.com/live"
	content := `package pkg

import "time"

func ` + testName + `(t *testing.T) {
	` + sleepCall + `
	_ = "` + externalURL + `"
}
`
	writeGuardFile(t, repo, "pkg/foo_test.go", content)
	if code, _, stderr := GitCmd(repo, "add", "pkg/foo_test.go"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}

	result := GuardCheck(GuardCheckRequest{RepoRoot: repo, Staged: true})
	if result.OK || result.Summary.Block != 2 {
		t.Fatalf("expected blocking findings: %+v", result)
	}
	if !guardHasFinding(result, "sleep-in-test") || !guardHasFinding(result, "real-external-service-in-test") {
		t.Fatalf("expected sleep and external service findings: %+v", result.Findings)
	}
	if !guardHasFinding(result, "ambiguous-test-name") {
		t.Fatalf("expected ambiguous test name warning: %+v", result.Findings)
	}
}

func TestGuardCheckFlagsReuseBeforeNewAsReview(t *testing.T) {
	repo := initGuardRepo(t)
	writeGuardFile(t, repo, "internal/core/existing.go", `package core

func normalizeTargetDocs(items []string) []string { return items }
`)
	writeGuardFile(t, repo, "internal/core/new.go", `package core

func normalizeTargetDoc(items []string) []string { return items }
`)
	if code, _, stderr := GitCmd(repo, "add", "internal/core/new.go"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}

	result := GuardCheck(GuardCheckRequest{RepoRoot: repo, Staged: true})
	if result.Summary.Block != 0 || result.Summary.Review == 0 {
		t.Fatalf("expected non-blocking reuse review: %+v", result)
	}
	if !guardHasFinding(result, "reuse-before-new") {
		t.Fatalf("expected reuse finding: %+v", result.Findings)
	}
}

func TestGuardCheckWarnsOnProdChangeWithoutTest(t *testing.T) {
	repo := initGuardRepo(t)
	writeGuardFile(t, repo, "cmd/app/main.go", `package main

func main() {}
`)
	if code, _, stderr := GitCmd(repo, "add", "cmd/app/main.go"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}

	result := GuardCheck(GuardCheckRequest{RepoRoot: repo, Staged: true})
	if result.Summary.Block != 0 || !result.OK {
		t.Fatalf("prod-only warning should not block: %+v", result)
	}
	if !guardHasFinding(result, "prod-change-without-test") {
		t.Fatalf("expected prod without test warning: %+v", result.Findings)
	}
}

func TestGuardCheckBlocksSecretLikePaths(t *testing.T) {
	repo := initGuardRepo(t)
	writeGuardFile(t, repo, ".env", "TOKEN=secret\n")
	if code, _, stderr := GitCmd(repo, "add", ".env"); code != 0 {
		t.Fatalf("git add failed: %s", stderr)
	}

	result := GuardCheck(GuardCheckRequest{RepoRoot: repo, Staged: true})
	if result.OK || !guardHasFinding(result, "secret-like-path") {
		t.Fatalf("expected secret path block: %+v", result)
	}
}

func initGuardRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if code, _, stderr := GitCmd(repo, "init"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	return repo
}

func writeGuardFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func guardHasFinding(result GuardCheckResult, rule string) bool {
	for _, finding := range result.Findings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}
