package lifecycle

import (
	"os/exec"
	"testing"
)

func TestExecutionAllowsOnlyStandaloneCommandVObservation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for _, command := range []string{
		"command -v go",
		"command -v definitely_missing_agent_harness_tool",
	} {
		req := executionRequest(active, worker, "codex", "observer-session", command)
		req.AgentID = ""
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact command availability observation was blocked: %q -> %+v", command, got)
		}
	}

	for _, command := range []string{
		"command -v go && pwd",
		"pwd && command -v go",
		"command -v go; pwd",
		"pwd; command -v go",
		"command -v go\npwd",
		"pwd\ncommand -v go",
		"command -v go | head -1",
		"pwd | command -v go",
		"command -v $TOOL",
		"command -v ./go",
		"command -v go other",
	} {
		req := executionRequest(active, worker, "codex", "observer-session", command)
		req.AgentID = ""
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
			t.Fatalf("inexact command availability probe was not fail-closed: %q -> %+v", command, got)
		}
	}
}

func TestExecutionPreservesCommandVFoundAndNotFoundResults(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for _, test := range []struct {
		name        string
		command     string
		wantSuccess bool
	}{
		{name: "found", command: "command -v sh", wantSuccess: true},
		{name: "not found", command: "command -v definitely_missing_agent_harness_tool", wantSuccess: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := executionRequest(active, worker, "codex", "observer-session", test.command)
			req.AgentID = ""
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("command availability observation was blocked: %+v", got)
			}

			err := exec.Command("sh", "-c", test.command).Run()
			if test.wantSuccess && err != nil {
				t.Fatalf("found command result was not preserved: %v", err)
			}
			if !test.wantSuccess && err == nil {
				t.Fatal("not-found command result was not preserved")
			}
		})
	}
}
