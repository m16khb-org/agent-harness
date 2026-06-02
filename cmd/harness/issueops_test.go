package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunIssueOpsLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", "/repo/example", "--branch", "feature/demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" || record["phase"] != "problem" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	issue := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	var issueRecord map[string]any
	if err := json.Unmarshal([]byte(issue), &issueRecord); err != nil {
		t.Fatalf("issue link should return JSON: %v\n%s", err, issue)
	}
	if issueRecord["phase"] != "plan" {
		t.Fatalf("issue link should move to plan phase: %#v", issueRecord)
	}

	plan := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-plan", "--id", id, "--plan-path", "docs/superpowers/plans/demo.md", "--json"})
	})
	var planRecord map[string]any
	if err := json.Unmarshal([]byte(plan), &planRecord); err != nil {
		t.Fatalf("plan link should return JSON: %v\n%s", err, plan)
	}
	if planRecord["phase"] != "implement" {
		t.Fatalf("plan link should move to implement phase: %#v", planRecord)
	}

	feedback := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"feedback", "add", "--id", id, "--source", "user", "--body", "tighten acceptance criteria", "--json"})
	})
	var feedbackRecord map[string]any
	if err := json.Unmarshal([]byte(feedback), &feedbackRecord); err != nil {
		t.Fatalf("feedback should return JSON: %v\n%s", err, feedback)
	}
	if feedbackRecord["phase"] != "feedback" || !strings.Contains(feedback, "tighten acceptance criteria") {
		t.Fatalf("feedback should be persisted: %#v", feedbackRecord)
	}

	readiness := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", id, "--json"})
	})
	var ready map[string]any
	if err := json.Unmarshal([]byte(readiness), &ready); err != nil {
		t.Fatalf("readiness should return JSON: %v\n%s", err, readiness)
	}
	if ready["ready"] != true {
		t.Fatalf("expected PR readiness once issue and plan are linked: %#v", ready)
	}
}
