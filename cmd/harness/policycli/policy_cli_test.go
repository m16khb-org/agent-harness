package policycli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/testsupport"
)

func TestParseCommandPolicyFlagsUsesDefaultRootCWDAndEnvAllowlist(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("PWD", root)
	t.Cleanup(setPolicyCLITestResolveTarget())

	req, jsonOut, err := parseCommandPolicyFlags("policy check", []string{"--json", "--env", "HOME, PATH,,HARNESS_STATE_DIR", "--", "git", "status"})
	if err != nil {
		t.Fatalf("parseCommandPolicyFlags returned error: %v", err)
	}

	wantRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	if !jsonOut {
		t.Fatalf("jsonOut = false, want true")
	}
	if req.WorkspaceRoot != wantRoot || req.CWD != wantRoot {
		t.Fatalf("root/cwd = (%q, %q), want (%q, %q)", req.WorkspaceRoot, req.CWD, wantRoot, wantRoot)
	}
	if !reflect.DeepEqual(req.EnvAllowlist, []string{"HOME", "PATH", "HARNESS_STATE_DIR"}) {
		t.Fatalf("env allowlist = %#v", req.EnvAllowlist)
	}
	if !reflect.DeepEqual(req.Argv, []string{"git", "status"}) {
		t.Fatalf("argv = %#v", req.Argv)
	}
	if req.Timeout != "30s" || req.WriteAllowed || req.NetworkAllowed || req.ShellAllowed || req.ShellReason != "" {
		t.Fatalf("unexpected policy request defaults: %#v", req)
	}
}

func TestParseCommandPolicyRunFlagsUsesReadOnlyJSONAndExplicitCWD(t *testing.T) {
	root := t.TempDir()
	cwd := t.TempDir()

	req, jsonOut, readOnly, err := parseCommandPolicyRunFlags([]string{
		"--read-only",
		"--json",
		"--workspace-root", root,
		"--cwd", cwd,
		"--timeout", "5s",
		"--env", "HOME",
		"--",
		"git", "status", "--short",
	})
	if err != nil {
		t.Fatalf("parseCommandPolicyRunFlags returned error: %v", err)
	}

	if !jsonOut || !readOnly {
		t.Fatalf("jsonOut/readOnly = (%v, %v), want (true, true)", jsonOut, readOnly)
	}
	if req.WorkspaceRoot != root || req.CWD != cwd {
		t.Fatalf("root/cwd = (%q, %q), want (%q, %q)", req.WorkspaceRoot, req.CWD, root, cwd)
	}
	if req.Timeout != "5s" || !reflect.DeepEqual(req.EnvAllowlist, []string{"HOME"}) {
		t.Fatalf("unexpected run request: %#v", req)
	}
	if !reflect.DeepEqual(req.Argv, []string{"git", "status", "--short"}) {
		t.Fatalf("argv = %#v", req.Argv)
	}
}

func TestRunPolicyRunReturnsDeniedPolicyError(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	err := runPolicyRun([]string{"--read-only", "--workspace-root", repo, "--cwd", repo, "--", "sh", "-c", "true"})

	var denied core.PolicyDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected policy denied error, got %T %v", err, err)
	}
	if !containsString(denied.Reasons, "shell_interpreter_not_allowed") {
		t.Fatalf("deny reasons %v missing shell_interpreter_not_allowed", denied.Reasons)
	}
}

func TestRunPolicyCheckUsesWorkspacePolicyOverride(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "policy.json"), []byte(`{"additional_read_only_commands":["repo-tool"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout := capturePolicyCLIStdout(t, func() error {
		return runPolicyCheck([]string{"--json", "--workspace-root", repo, "--cwd", repo, "--", "repo-tool"})
	})
	var result core.CommandPolicyEvaluation
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal policy check output %q: %v", stdout, err)
	}
	if !result.Allowed {
		t.Fatalf("workspace policy override was not applied: %+v", result)
	}
}

func runStatusVerifyTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}

func capturePolicyCLIStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func setPolicyCLITestResolveTarget() func() {
	Configure(Deps{ResolveTarget: func(arg string) string {
		if arg != "" {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return arg
			}
			return abs
		}
		if env := testEnv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := testEnv("PWD"); env != "" {
			arg = env
		}
		abs, err := filepath.Abs(arg)
		if err != nil {
			return arg
		}
		return abs
	}})
	return Reset
}

func testEnv(name string) string {
	value, ok := os.LookupEnv(name)
	if !ok {
		return ""
	}
	return value
}
