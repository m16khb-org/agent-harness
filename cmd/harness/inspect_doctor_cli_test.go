package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunInspect_printsText_whenRepoIsPositional(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runInspect([]string{repo})
	})

	// Then
	if !strings.Contains(out, "agent-harness root:") || !strings.Contains(out, "target repo: "+repo) {
		t.Fatalf("unexpected inspect text:\n%s", out)
	}
	if !strings.Contains(out, "codex skill installed:") || !strings.Contains(out, "claude skill installed:") {
		t.Fatalf("inspect text should include native integration status:\n%s", out)
	}
}

func TestRunInspect_printsJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runInspect([]string{"--repo", repo, "--json"})
	})

	// Then
	var result core.InspectInfo
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode inspect json: %v\n%s", err, out)
	}
	if !result.OK || result.TargetRepo != repo {
		t.Fatalf("unexpected inspect result: %+v", result)
	}
}

func TestRunDoctor_printsHealthyText_whenProjectDocsAreInitialized(t *testing.T) {
	// Given
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: repo, Write: true}); err != nil {
		t.Fatalf("bootstrap project docs: %v", err)
	}

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runDoctor([]string{"--repo", repo})
	})

	// Then
	if !strings.Contains(out, "agent-harness doctor healthy: "+repo) {
		t.Fatalf("unexpected healthy doctor text:\n%s", out)
	}
}

func TestRunDoctor_printsIssuesText_whenProjectDocsAreMissing(t *testing.T) {
	// Given
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runDoctor([]string{repo})
	})

	// Then
	if !strings.Contains(out, "agent-harness doctor found") || !strings.Contains(out, "project_docs_missing") {
		t.Fatalf("unexpected unhealthy doctor text:\n%s", out)
	}
	if !strings.Contains(out, "fix: agent-harness project bootstrap --repo") {
		t.Fatalf("doctor text should include fix command:\n%s", out)
	}
}

func TestRunDoctor_printsJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runDoctor([]string{"--repo", repo, "--json"})
	})

	// Then
	var result core.HarnessDoctorResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode doctor json: %v\n%s", err, out)
	}
	if !result.OK || result.RepoRoot != repo || result.Healthy {
		t.Fatalf("unexpected doctor result: %+v", result)
	}
	if !doctorResultHasIssue(result, "project_docs_missing") {
		t.Fatalf("expected project docs issue: %+v", result.Issues)
	}
}

func TestRunDoctor_returnsError_whenRepoPathIsInvalid(t *testing.T) {
	// Given
	badRepo := filepath.Join(t.TempDir(), "missing")

	// When
	err := runDoctor([]string{"--repo", badRepo})

	// Then
	if err == nil || !strings.Contains(err.Error(), badRepo) {
		t.Fatalf("expected invalid repo error, got %v", err)
	}
}

func doctorResultHasIssue(result core.HarnessDoctorResult, code string) bool {
	for _, issue := range result.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestRunDoctor_doesNotRequireRealHome(t *testing.T) {
	// Given
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return runDoctor([]string{"--repo", repo})
	})

	// Then
	if !strings.Contains(out, "codex_hooks_missing") {
		t.Fatalf("doctor should inspect configured HOME integration paths:\n%s", out)
	}
}
