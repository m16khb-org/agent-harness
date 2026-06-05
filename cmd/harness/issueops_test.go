package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunIssueOpsLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", "/repo/example", "--branch", "feature/provider-linked-branch", "--json"})
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

	worktree := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", "/repo/example.worktrees/feature-demo", "--json"})
	})
	var worktreeRecord map[string]any
	if err := json.Unmarshal([]byte(worktree), &worktreeRecord); err != nil {
		t.Fatalf("worktree link should return JSON: %v\n%s", err, worktree)
	}
	if worktreeRecord["worktree_path"] != "/repo/example.worktrees/feature-demo" {
		t.Fatalf("worktree link should persist exact path: %#v", worktreeRecord)
	}

	branch := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "feature/provider-linked-branch", "--base-branch", "main", "--json"})
	})
	var branchRecord map[string]any
	if err := json.Unmarshal([]byte(branch), &branchRecord); err != nil {
		t.Fatalf("branch prepare should return JSON: %v\n%s", err, branch)
	}
	prepare, ok := branchRecord["branch_prepare"].(map[string]any)
	if !ok || prepare["provider"] != "github" || prepare["branch"] != "feature/provider-linked-branch" {
		t.Fatalf("branch prepare should persist provider-linked contract: %#v", branchRecord)
	}
	steps, ok := prepare["steps"].([]any)
	if !ok || len(steps) != 3 {
		t.Fatalf("branch prepare should include fallback steps: %#v", prepare)
	}

	child := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-child", "--id", id, "--child-url", "https://github.com/example/repo/issues/2", "--title", "write child graph tests", "--json"})
	})
	var childRecord map[string]any
	if err := json.Unmarshal([]byte(child), &childRecord); err != nil {
		t.Fatalf("child issue link should return JSON: %v\n%s", err, child)
	}
	issueLinks, ok := childRecord["issue_links"].([]any)
	if !ok || len(issueLinks) != 1 {
		t.Fatalf("child issue link should be persisted: %#v", childRecord)
	}
	childLink, ok := issueLinks[0].(map[string]any)
	if !ok || childLink["type"] != "child" || childLink["provider"] != "github" {
		t.Fatalf("unexpected child issue link payload: %#v", issueLinks[0])
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
