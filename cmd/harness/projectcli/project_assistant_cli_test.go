package projectcli

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunProjectCommitSuggest_printsNoChangesMessage_whenDiffIsEmpty(t *testing.T) {
	// Given
	repo := newProjectAssistantGitRepo(t)

	// When
	stderr, err := captureProjectCLIStderr(func() error {
		return RunCommitSuggest([]string{"--repo", repo})
	})

	// Then
	if err != nil {
		t.Fatalf("commit-suggest should succeed without changes: %v", err)
	}
	if !strings.Contains(stderr, "No changes detected. Nothing to suggest.") {
		t.Fatalf("expected no-changes message, got:\n%s", stderr)
	}
}

func TestRunProjectCommitSuggest_printsNoChangesJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	repo := newProjectAssistantGitRepo(t)

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return RunCommitSuggest([]string{"--repo", repo, "--staged", "--json"})
	})

	// Then
	var result core.CommitSuggestResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode commit-suggest json: %v\n%s", err, out)
	}
	if !result.OK || result.Executed || !result.Staged {
		t.Fatalf("unexpected commit-suggest result: %+v", result)
	}
}

func TestRunProjectLintDiagnose_printsSuccessText_whenCommandPasses(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return RunLintDiagnose([]string{"--repo", repo, "--", "/bin/sh", "-c", "printf ok"})
	})

	// Then
	if !strings.Contains(out, "Command completed successfully. No failure detected.") {
		t.Fatalf("expected success text, got:\n%s", out)
	}
}

func TestRunProjectLintDiagnose_printsSuccessJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	out := captureStatusVerifyStdout(t, func() error {
		return RunLintDiagnose([]string{"--repo", repo, "--json", "--", "/bin/sh", "-c", "printf ok"})
	})

	// Then
	var result core.LintDiagnoseResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode lint-diagnose json: %v\n%s", err, out)
	}
	if !result.OK || result.Failed || result.ExitCode != 0 {
		t.Fatalf("unexpected lint-diagnose result: %+v", result)
	}
}

func TestRunProjectLintDiagnose_rejectsMissingCommand(t *testing.T) {
	// Given
	repo := t.TempDir()

	// When
	err := RunLintDiagnose([]string{"--repo", repo})

	// Then
	if err == nil || !strings.Contains(err.Error(), "missing command to run") {
		t.Fatalf("expected missing command error, got %v", err)
	}
}

func newProjectAssistantGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runProjectAssistantGit(t, repo, "init")
	return repo
}

func runProjectAssistantGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
