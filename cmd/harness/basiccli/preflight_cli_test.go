package basiccli

import (
	preflight "agent-harness/internal/contract/preflight"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPreflightPrintsJSONForExplicitTarget(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	out := captureStatusVerifyStdout(t, func() error {
		return RunPreflight([]string{"--json", repo})
	})

	var result preflight.PreflightResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode preflight JSON: %v\n%s", err, out)
	}
	if !result.OK || !sameCleanPath(result.RepoRoot, repo) {
		t.Fatalf("unexpected preflight result: %#v", result)
	}
}

func TestRunPreflightFalseJSONFlagStillPrintsJSON(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	out := captureStatusVerifyStdout(t, func() error {
		return RunPreflight([]string{"--json=false", repo})
	})

	if !strings.Contains(out, `"ok": true`) || !strings.Contains(out, `"repo_root": "`) {
		t.Fatalf("unexpected preflight output:\n%s", out)
	}
}

func TestRunPreflightDefaultsTargetFromEnvironment(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("PWD", repo)

	out := captureStatusVerifyStdout(t, func() error {
		return RunPreflight(nil)
	})

	var result preflight.PreflightResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode preflight JSON: %v\n%s", err, out)
	}
	if !sameCleanPath(result.RepoRoot, repo) {
		t.Fatalf("repo root = %q, want path equivalent to %q", result.RepoRoot, repo)
	}
}

func TestRunPreflightReturnsFlagParseError(t *testing.T) {
	if err := RunPreflight([]string{"--missing"}); err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("expected flag parse error, got %v", err)
	}
}

func sameCleanPath(got, want string) bool {
	gotEval, gotErr := filepath.EvalSymlinks(got)
	wantEval, wantErr := filepath.EvalSymlinks(want)
	if gotErr == nil && wantErr == nil {
		return gotEval == wantEval
	}
	return filepath.Clean(got) == filepath.Clean(want)
}
