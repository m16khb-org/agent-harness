package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunTraceRoutesUsageUnknownAndAnalyzeJSON(t *testing.T) {
	if err := runTrace(nil); err == nil || !strings.Contains(err.Error(), "missing trace subcommand") {
		t.Fatalf("expected missing trace subcommand error, got %v", err)
	}
	if err := runTrace([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown trace subcommand "unknown"`) {
		t.Fatalf("expected unknown trace subcommand error, got %v", err)
	}

	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"deterministic","failed_step":"go test","rerun_commands":["go test ./..."]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return runTrace([]string{"analyze", "--input", input, "--json"})
	})
	var result core.TraceAnalyzeResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode trace analyze JSON: %v\n%s", err, out)
	}
	if !result.OK || result.FindingCount != 1 || result.InputSource != "file" {
		t.Fatalf("unexpected trace analyze result: %#v", result)
	}
}

func TestRunTraceAnalyzeWritesTextSummary(t *testing.T) {
	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"intermittent","failed_step":"guard","rerun_commands":["agent-harness guard check"]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return runTraceAnalyze([]string{input})
	})
	if !strings.Contains(out, "trace analysis: 1 finding") || !strings.Contains(out, "verify: agent-harness guard check") {
		t.Fatalf("unexpected trace text output:\n%s", out)
	}
}

func TestRunGuardRoutesAndChecksExplicitFiles(t *testing.T) {
	if err := runGuard(nil); err == nil || !strings.Contains(err.Error(), "missing guard subcommand") {
		t.Fatalf("expected missing guard subcommand error, got %v", err)
	}
	if err := runGuard([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown guard subcommand "unknown"`) {
		t.Fatalf("expected unknown guard subcommand error, got %v", err)
	}

	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "ok_test.go"), "package main\n\nfunc TestSpecificBehavior(t *testing.T) {}\n")
	out := captureStatusVerifyStdout(t, func() error {
		return runGuard([]string{"check", "--repo", repo, "--json", "--", "ok_test.go"})
	})
	var result core.GuardCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode guard JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Mode != "files" || len(result.CheckedFiles) != 1 {
		t.Fatalf("unexpected guard result: %#v", result)
	}
}

func TestRunPolicyRoutesFakeRunAndReadOnlyRun(t *testing.T) {
	repo := t.TempDir()
	runStatusVerifyTestCommand(t, repo, "git", "init")

	if err := runPolicy(nil); err == nil || !strings.Contains(err.Error(), "missing policy subcommand") {
		t.Fatalf("expected missing policy subcommand error, got %v", err)
	}
	if err := runPolicy([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown policy subcommand "unknown"`) {
		t.Fatalf("expected unknown policy subcommand error, got %v", err)
	}

	fakeOut := captureStatusVerifyStdout(t, func() error {
		return runPolicy([]string{"fake-run", "--workspace-root", repo, "--cwd", repo, "--write", "--json", "--", "touch", "marker"})
	})
	var fake core.CommandFakeRunResult
	if err := json.Unmarshal([]byte(fakeOut), &fake); err != nil {
		t.Fatalf("decode fake-run JSON: %v\n%s", err, fakeOut)
	}
	if !fake.OK || fake.Executed || !fake.Policy.Allowed {
		t.Fatalf("unexpected fake-run result: %#v", fake)
	}

	runOut := captureStatusVerifyStdout(t, func() error {
		return runPolicy([]string{"run", "--read-only", "--workspace-root", repo, "--cwd", repo, "--json", "--", "git", "status", "--short"})
	})
	var run core.CommandRunResult
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
	if err := runPolicyRun([]string{"--workspace-root", repo, "--cwd", repo, "--", "git", "status", "--short"}); err == nil || !strings.Contains(err.Error(), "requires --read-only") {
		t.Fatalf("expected read-only requirement error, got %v", err)
	}

	err := runPolicyFakeRun([]string{"--workspace-root", repo, "--cwd", repo, "--", "sh", "-c", "true"})
	var denied core.PolicyDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("expected policy denied error, got %T %v", err, err)
	}
}

func TestRunGuardCheckTextOutputReturnsBlockedError(t *testing.T) {
	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "slow_test.go"), "package main\n\nfunc TestSlow(t *testing.T) {\n\ttime.Sleep(1)\n}\n")

	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return runGuardCheck([]string{"--repo", repo, "--", "slow_test.go"})
	})
	var blocked core.GuardBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected guard blocked error, got %T %v", err, err)
	}
	if !strings.Contains(out, "guard files: 1 file(s), block=1") || !strings.Contains(out, "sleep-in-test") {
		t.Fatalf("unexpected guard text output:\n%s", out)
	}
}

func TestRunPolicyCheckAndAuditWriteTextOutput(t *testing.T) {
	repo := t.TempDir()
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("HARNESS_AUDIT_LOG", auditLog)

	checkOut := captureStatusVerifyStdout(t, func() error {
		return runPolicyCheck([]string{"--workspace-root", repo, "--cwd", repo, "--", "git", "status", "--short"})
	})
	if !strings.Contains(checkOut, "policy allowed:") {
		t.Fatalf("unexpected policy check text output:\n%s", checkOut)
	}

	auditOut := captureStatusVerifyStdout(t, func() error {
		return runPolicyAudit([]string{"--workspace-root", repo, "--cwd", repo, "--write", "--", "touch", "marker"})
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
		return runPolicyRun([]string{"--read-only", "--workspace-root", repo, "--cwd", repo, "--", "git", "diff", "--exit-code"})
	})
	if err == nil || !strings.Contains(err.Error(), "command exited 1") {
		t.Fatalf("expected command exit error, got %v", err)
	}
	if !strings.Contains(out, "policy allowed:") || !strings.Contains(out, "dirty.txt") {
		t.Fatalf("unexpected policy run text output:\n%s", out)
	}
}

func captureTraceGuardPolicyStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	defer func() {
		os.Stdout = oldStdout
	}()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	defer r.Close()
	os.Stdout = w
	callErr := fn()
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("close stdout pipe: %v", closeErr)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	return out.String(), callErr
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
