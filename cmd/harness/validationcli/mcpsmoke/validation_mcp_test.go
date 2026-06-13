package mcpsmoke

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMCPSmokeInputAndExpectedMarkers(t *testing.T) {
	input := MCPSmokeInput()
	if got := len(splitLines(input)); got != 12 {
		t.Fatalf("MCPSmokeInput line count = %d", got)
	}
	for _, want := range []string{"initialize", "tools/list", "harness://project-docs", "state_migrate"} {
		if !strings.Contains(input, want) {
			t.Fatalf("expected input to contain %q", want)
		}
	}
	markers := MCPSmokeExpectedMarkers()
	if len(markers) == 0 || !MCPSmokeHasExpectedMarkers(strings.Join(markers, "\n")) {
		t.Fatal("expected marker set to satisfy marker check")
	}
	if MCPSmokeHasExpectedMarkers("atomic_commit_preflight only") {
		t.Fatal("partial marker output should fail")
	}
}

func TestValidateMCPSmokeContract(t *testing.T) {
	stdout := validMCPSmokeStdout()
	step := StepResult{OK: true, Stdout: stdout}
	ValidateMCPSmokeContract(&step)
	if !step.OK || step.Error != "" {
		t.Fatalf("valid contract failed: %#v", step)
	}
	for _, tc := range []struct {
		name   string
		stdout string
		want   string
	}{
		{name: "wrong count", stdout: `{"result":{}}`, want: "expected 12 MCP responses"},
		{name: "bad json", stdout: strings.Repeat(`{"result":{}}`+"\n", 11) + `not json`, want: "invalid JSON"},
		{name: "missing result", stdout: strings.Repeat(`{"result":{}}`+"\n", 11) + `{"error":{}}`, want: "has no result"},
		{name: "missing markers", stdout: strings.Repeat(`{"result":{}}`+"\n", 12), want: "expected tool/resource"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := StepResult{OK: true, Stdout: tc.stdout}
			ValidateMCPSmokeContract(&step)
			if step.OK || !strings.Contains(step.Error, tc.want) {
				t.Fatalf("got %#v, want error containing %q", step, tc.want)
			}
		})
	}
}

func TestValidateMCPWithDepsRunsSmokeAndCleanup(t *testing.T) {
	var removed []string
	var stopCalled bool
	deps := MCPValidationDeps{
		MkdirTemp: func(_, pattern string) (string, error) {
			return "/tmp/" + strings.TrimSuffix(pattern, "*") + "x", nil
		},
		RemoveAll: func(path string) error {
			removed = append(removed, path)
			return nil
		},
		RunCommandStepEnv: func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult {
			stopCalled = label == "MCP daemon stop" && name == "bin" && len(args) == 3
			return StepResult{OK: true}
		},
		RunCommandStepEnvWithBudget: func(dir, label string, timeout time.Duration, stdin string, env []string, outputBudget int, name string, args ...string) StepResult {
			if dir != "/repo" || label != "MCP smoke" || name != "bin" || len(args) != 1 || args[0] != "mcp" {
				return StepResult{OK: false, Error: "unexpected command"}
			}
			if !strings.Contains(stdin, "tools/list") || !containsEnv(env, "HARNESS_MCP_DIRECT=1") {
				return StepResult{OK: false, Error: "missing smoke input/env"}
			}
			return StepResult{OK: true, Stdout: validMCPSmokeStdout()}
		},
	}
	step := ValidateMCPWithDeps("bin", "/repo", deps)
	if !step.OK || step.Error != "" {
		t.Fatalf("ValidateMCPWithDeps failed: %#v", step)
	}
	if !stopCalled || len(removed) != 2 {
		t.Fatalf("cleanup not called as expected: stop=%v removed=%#v", stopCalled, removed)
	}
	if !step.StdoutTruncated || step.StdoutBytes <= aggregateOutputBudgetBytes {
		t.Fatalf("expected truncated aggregate stdout, got bytes=%d truncated=%v", step.StdoutBytes, step.StdoutTruncated)
	}
}

func TestValidateMCPWithDepsFailurePaths(t *testing.T) {
	step := ValidateMCPWithDeps("bin", "/repo", MCPValidationDeps{
		MkdirTemp: func(_, _ string) (string, error) { return "", errors.New("mkdir failed") },
	})
	if step.OK || !strings.Contains(step.Error, "mkdir failed") {
		t.Fatalf("expected mkdir failure, got %#v", step)
	}
	step = ValidateMCPWithDeps("bin", "/repo", MCPValidationDeps{
		MkdirTemp: func(_, pattern string) (string, error) {
			if strings.HasPrefix(pattern, "ahd-") {
				return "", errors.New("daemon mkdir failed")
			}
			return "/tmp/state", nil
		},
		RemoveAll: func(string) error { return nil },
	})
	if step.OK || !strings.Contains(step.Error, "daemon mkdir failed") {
		t.Fatalf("expected daemon mkdir failure, got %#v", step)
	}
	step = ValidateMCPWithDeps("bin", "/repo", MCPValidationDeps{
		MkdirTemp: func(_, pattern string) (string, error) { return "/tmp/" + pattern, nil },
		RemoveAll: func(string) error { return nil },
		RunCommandStepEnv: func(string, string, time.Duration, string, []string, string, ...string) StepResult {
			return StepResult{OK: true}
		},
		RunCommandStepEnvWithBudget: func(string, string, time.Duration, string, []string, int, string, ...string) StepResult {
			return StepResult{OK: false, Error: "mcp failed"}
		},
	})
	if step.OK || step.Error != "mcp failed" {
		t.Fatalf("expected command failure, got %#v", step)
	}
}

func TestDepsDefaultsAndSmallHelpers(t *testing.T) {
	deps := (MCPValidationDeps{}).withDefaults()
	if deps.MkdirTemp == nil || deps.RemoveAll == nil || deps.RunCommandStepEnv == nil || deps.RunCommandStepEnvWithBudget == nil {
		t.Fatal("defaults should populate dependencies")
	}
	step := failedStep("label", errors.New("boom"))
	if step.OK || step.Label != "label" || !strings.Contains(step.Error, "boom") {
		t.Fatalf("unexpected failed step: %#v", step)
	}
	stdout, truncated, bytes := tailWithBudget("abcdef", 3)
	if stdout != "[tr" || !truncated || bytes != 6 {
		t.Fatalf("unexpected tail budget result stdout=%q truncated=%v bytes=%d", stdout, truncated, bytes)
	}
	if lines := splitLines(" a \n\n b \n"); len(lines) != 3 {
		t.Fatalf("splitLines preserves non-trimmed internal blank shape, got %#v", lines)
	}
}

func validMCPSmokeStdout() string {
	markers := strings.Join(MCPSmokeExpectedMarkers(), " ") + strings.Repeat("x", 800)
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, `{"result":{"id":%d,"text":%q}}`+"\n", i, markers)
	}
	return b.String()
}

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}
