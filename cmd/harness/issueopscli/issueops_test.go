package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsLifecycle(t *testing.T) {
	stubIssueOpsChildIssueVerifier(t, nil)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "example")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-provider-linked-branch", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" || record["phase"] != "problem" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	recordIssueOpsCLIIntentForTest(t, id)
	recordIssueOpsCLIPlanPrepForTest(t, id)
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

	branch := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-provider-linked-branch", "--base-branch", "main", "--link-verified", "--json"})
	})
	var branchRecord map[string]any
	if err := json.Unmarshal([]byte(branch), &branchRecord); err != nil {
		t.Fatalf("branch prepare should return JSON: %v\n%s", err, branch)
	}
	prepare, ok := branchRecord["branch_prepare"].(map[string]any)
	if !ok || prepare["provider"] != "github" || prepare["branch"] != "1-provider-linked-branch" {
		t.Fatalf("branch prepare should persist provider-linked contract: %#v", branchRecord)
	}
	steps, ok := prepare["steps"].([]any)
	if !ok || len(steps) != 3 {
		t.Fatalf("branch prepare should include fallback steps: %#v", prepare)
	}

	worktreePath := makeIssueOpsCLIWorktreeForTest(t, repo, "1-provider-linked-branch")
	worktree := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", worktreePath, "--json"})
	})
	var worktreeRecord map[string]any
	if err := json.Unmarshal([]byte(worktree), &worktreeRecord); err != nil {
		t.Fatalf("worktree link should return JSON: %v\n%s", err, worktree)
	}
	if worktreeRecord["worktree_path"] != worktreePath {
		t.Fatalf("worktree link should persist exact path: %#v", worktreeRecord)
	}
	recordIssueOpsCLIDesignForTest(t, id)
	writeIssueOpsCLIFileForTest(t, worktreePath, "docs/superpowers/plans/demo.md", "plan\n")
	plan := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-plan", "--id", id, "--plan-path", filepath.Join(worktreePath, "docs", "superpowers", "plans", "demo.md"), "--json"})
	})
	var planRecord map[string]any
	if err := json.Unmarshal([]byte(plan), &planRecord); err != nil {
		t.Fatalf("plan link should return JSON: %v\n%s", err, plan)
	}
	if planRecord["phase"] != "implement" {
		t.Fatalf("plan link should move to implement phase: %#v", planRecord)
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
	if feedbackRecord["phase"] != "implement" || !strings.Contains(feedback, "tighten acceptance criteria") {
		t.Fatalf("early feedback should be persisted without entering feedback phase: %#v", feedbackRecord)
	}

	beforeCleanReadiness := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", id, "--json"})
	})
	var beforeClean map[string]any
	if err := json.Unmarshal([]byte(beforeCleanReadiness), &beforeClean); err != nil {
		t.Fatalf("readiness should return JSON: %v\n%s", err, beforeCleanReadiness)
	}
	if beforeClean["ready"] == true || !strings.Contains(beforeCleanReadiness, "ai_slop_clean") {
		t.Fatalf("PR readiness should require ai-slop-clean before drafting: %#v", beforeClean)
	}

	if err := runIssueOps([]string{"phase", "--id", id, "--to", "ai-slop-clean", "--json"}); err == nil || !strings.Contains(err.Error(), "implementation_changes") {
		t.Fatalf("ai-slop-clean should require implementation changes, got %v", err)
	}
	writeIssueOpsCLIFileForTest(t, worktreePath, "internal/demo.go", "package demo\n")
	cleaned := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"phase", "--id", id, "--to", "ai-slop-clean", "--json"})
	})
	var cleanedRecord map[string]any
	if err := json.Unmarshal([]byte(cleaned), &cleanedRecord); err != nil {
		t.Fatalf("ai-slop-clean phase should return JSON: %v\n%s", err, cleaned)
	}
	if cleanedRecord["phase"] != "ai-slop-clean" || cleanedRecord["ai_slop_clean_at"] == "" {
		t.Fatalf("ai-slop-clean should be persisted before PR readiness: %#v", cleanedRecord)
	}

	readiness := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", id, "--json"})
	})
	var ready map[string]any
	if err := json.Unmarshal([]byte(readiness), &ready); err != nil {
		t.Fatalf("readiness should return JSON: %v\n%s", err, readiness)
	}
	if ready["ready"] != true {
		t.Fatalf("expected PR readiness once issue, plan, and ai-slop-clean are linked: %#v", ready)
	}

	contractFeedback := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"feedback", "add", "--id", id, "--source", "review", "--body", "acceptance criteria changed", "--classification", "contract_change", "--json"})
	})
	var contractRecord map[string]any
	if err := json.Unmarshal([]byte(contractFeedback), &contractRecord); err != nil {
		t.Fatalf("contract feedback should return JSON: %v\n%s", err, contractFeedback)
	}
	blockedReadiness := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", id, "--json"})
	})
	var blocked map[string]any
	if err := json.Unmarshal([]byte(blockedReadiness), &blocked); err != nil {
		t.Fatalf("blocked readiness should return JSON: %v\n%s", err, blockedReadiness)
	}
	if blocked["ready"] == true || !strings.Contains(blockedReadiness, "contract_feedback_issue_update") {
		t.Fatalf("contract feedback should block PR readiness until issue update is recorded: %#v", blocked)
	}
	marked := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"feedback", "mark-issue-updated", "--id", id, "--json"})
	})
	var markedRecord map[string]any
	if err := json.Unmarshal([]byte(marked), &markedRecord); err != nil {
		t.Fatalf("mark issue updated should return JSON: %v\n%s", err, marked)
	}
	if !strings.Contains(marked, "issue_updated_at") {
		t.Fatalf("mark issue updated should timestamp contract feedback: %#v", markedRecord)
	}
	unblockedReadiness := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", id, "--json"})
	})
	var unblocked map[string]any
	if err := json.Unmarshal([]byte(unblockedReadiness), &unblocked); err != nil {
		t.Fatalf("unblocked readiness should return JSON: %v\n%s", err, unblockedReadiness)
	}
	if unblocked["ready"] != true || strings.Contains(unblockedReadiness, "contract_feedback_issue_update") {
		t.Fatalf("issue update mark should unblock PR readiness: %#v", unblocked)
	}
}
