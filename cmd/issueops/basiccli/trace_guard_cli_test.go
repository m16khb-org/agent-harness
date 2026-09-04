package basiccli

import (
	"encoding/json"
	"errors"
	guard "issueops/internal/adapter/guard"
	guardcontract "issueops/internal/contract/guard"
	trace "issueops/internal/contract/trace"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTraceRoutesUsageUnknownAndAnalyzeJSON(t *testing.T) {
	if err := RunTrace(nil); err == nil || !strings.Contains(err.Error(), "missing trace subcommand") {
		t.Fatalf("expected missing trace subcommand error, got %v", err)
	}
	if err := RunTrace([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown trace subcommand "unknown"`) {
		t.Fatalf("expected unknown trace subcommand error, got %v", err)
	}

	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"deterministic","failed_step":"go test","rerun_commands":["go test ./..."]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return RunTrace([]string{"analyze", "--input", input, "--json"})
	})
	var result trace.TraceAnalyzeResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode trace analyze JSON: %v\n%s", err, out)
	}
	if !result.OK || result.FindingCount != 1 || result.InputSource != "file" {
		t.Fatalf("unexpected trace analyze result: %#v", result)
	}
}

func TestRunTraceAnalyzeWritesTextSummary(t *testing.T) {
	input := filepath.Join(t.TempDir(), "trace.json")
	writeFileForCLITest(t, input, `{"summary":{"failed_steps":1,"failure_class":"intermittent","failed_step":"guard","rerun_commands":["issueops guard check"]}}`)
	out := captureStatusVerifyStdout(t, func() error {
		return RunTraceAnalyze([]string{input})
	})
	if !strings.Contains(out, "trace analysis: 1 finding") || !strings.Contains(out, "verify: issueops guard check") {
		t.Fatalf("unexpected trace text output:\n%s", out)
	}
}

func TestRunGuardRoutesAndChecksExplicitFiles(t *testing.T) {
	if err := RunGuard(nil); err == nil || !strings.Contains(err.Error(), "missing guard subcommand") {
		t.Fatalf("expected missing guard subcommand error, got %v", err)
	}
	if err := RunGuard([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown guard subcommand "unknown"`) {
		t.Fatalf("expected unknown guard subcommand error, got %v", err)
	}

	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "ok_test.go"), "package main\n\nfunc TestSpecificBehavior(t *testing.T) {}\n")
	out := captureStatusVerifyStdout(t, func() error {
		return RunGuard([]string{"check", "--repo", repo, "--json", "--", "ok_test.go"})
	})
	var result guardcontract.GuardCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode guard JSON: %v\n%s", err, out)
	}
	if !result.OK || result.Mode != "files" || len(result.CheckedFiles) != 1 {
		t.Fatalf("unexpected guard result: %#v", result)
	}
}

func TestRunGuardCheckTextOutputReturnsBlockedError(t *testing.T) {
	repo := t.TempDir()
	writeFileForCLITest(t, filepath.Join(repo, "slow_test.go"), "package main\n\nfunc TestSlow(t *testing.T) {\n\ttime.Sleep(1)\n}\n")

	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return RunGuardCheck([]string{"--repo", repo, "--", "slow_test.go"})
	})
	var blocked guard.GuardBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected guard blocked error, got %T %v", err, err)
	}
	if !strings.Contains(out, "guard files: 1 file(s), block=1") || !strings.Contains(out, "sleep-in-test") {
		t.Fatalf("unexpected guard text output:\n%s", out)
	}
}
