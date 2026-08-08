package policycli

import (
	policy "agent-harness/internal/domain/policy"
	"agent-harness/internal/testsupport"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPolicyRoutesFakeRunAndReadOnlyRun(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	if err := Run(nil); err == nil || !strings.Contains(err.Error(), "missing policy subcommand") {
		t.Fatalf("expected missing policy subcommand error, got %v", err)
	}
	if err := Run([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown policy subcommand "unknown"`) {
		t.Fatalf("expected unknown policy subcommand error, got %v", err)
	}

	fakeOut := captureStatusVerifyStdout(t, func() error {
		return Run([]string{"fake-run", "--workspace-root", repo, "--cwd", repo, "--write", "--json", "--", "touch", "marker"})
	})
	var fake policy.CommandFakeRunResult
	if err := json.Unmarshal([]byte(fakeOut), &fake); err != nil {
		t.Fatalf("decode fake-run JSON: %v\n%s", err, fakeOut)
	}
	if !fake.OK || fake.Executed || !fake.Policy.Allowed {
		t.Fatalf("unexpected fake-run result: %#v", fake)
	}

	runOut := captureStatusVerifyStdout(t, func() error {
		return Run([]string{"run", "--read-only", "--workspace-root", repo, "--cwd", repo, "--json", "--", "git", "status", "--short"})
	})
	var run policy.CommandRunResult
	if err := json.Unmarshal([]byte(runOut), &run); err != nil {
		t.Fatalf("decode policy run JSON: %v\n%s", err, runOut)
	}
	if !run.OK || !run.Executed || !run.ReadOnly || !run.Policy.Allowed {
		t.Fatalf("unexpected policy run result: %#v", run)
	}
}

func TestRunPolicyRunRequiresReadOnlyAndDeniesShell(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	if err := RunReadOnly([]string{"--workspace-root", repo, "--cwd", repo, "--", "git", "status", "--short"}); err == nil || !strings.Contains(err.Error(), "requires --read-only") {
		t.Fatalf("expected read-only requirement error, got %v", err)
	}

	err := RunFakeRun([]string{"--workspace-root", repo, "--cwd", repo, "--", "sh", "-c", "true"})
	var denied policy.PolicyDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected policy denied error, got %T %v", err, err)
	}
}

func TestRunPolicyCheckAndAuditWriteTextOutput(t *testing.T) {
	repo := t.TempDir()
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("HARNESS_AUDIT_LOG", auditLog)

	checkOut := captureStatusVerifyStdout(t, func() error {
		return RunCheck([]string{"--workspace-root", repo, "--cwd", repo, "--", "git", "status", "--short"})
	})
	if !strings.Contains(checkOut, "policy allowed:") {
		t.Fatalf("unexpected policy check text output:\n%s", checkOut)
	}

	auditOut := captureStatusVerifyStdout(t, func() error {
		return RunAudit([]string{"--workspace-root", repo, "--cwd", repo, "--write", "--", "touch", "marker"})
	})
	if !strings.Contains(auditOut, "policy allowed:") || !strings.Contains(auditOut, "audit log: "+auditLog) {
		t.Fatalf("unexpected policy audit text output:\n%s", auditOut)
	}
	content, err := os.ReadFile(auditLog)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(content), `"kind":"command_policy_audit"`) {
		t.Fatalf("audit log missing record:\n%s", content)
	}
}

func TestRunPolicyRunReturnsCommandExitCodeError(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")
	writeFileForCLITest(t, filepath.Join(repo, "dirty.txt"), "clean\n")
	runStatusVerifyTestCommand(t, repo, "git", "add", "dirty.txt")
	writeFileForCLITest(t, filepath.Join(repo, "dirty.txt"), "dirty\n")

	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return RunReadOnly([]string{"--read-only", "--workspace-root", repo, "--cwd", repo, "--", "git", "diff", "--exit-code"})
	})
	if err == nil || !strings.Contains(err.Error(), "command exited 1") {
		t.Fatalf("expected command exit error, got %v", err)
	}
	if !strings.Contains(out, "policy allowed:") || !strings.Contains(out, "dirty.txt") {
		t.Fatalf("unexpected policy run text output:\n%s", out)
	}
}

func captureStatusVerifyStdout(t *testing.T, fn func() error) string {
	t.Helper()
	out, err := captureTraceGuardPolicyStdout(t, fn)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	return out
}

func captureTraceGuardPolicyStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	return testsupport.CaptureStdoutAndError(t, fn)
}

func writeFileForCLITest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
