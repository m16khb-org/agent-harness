package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsLifecycle(t *testing.T) {
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

func TestRunIssueOpsPhaseFailureWithJSONEmitsStructuredError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "json-failure")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "456-provider-linked-branch", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"phase", "--id", id, "--to", "pr", "--json"})
	})
	if err == nil {
		t.Fatalf("pr phase without readiness should still return an error")
	}
	var failure map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &failure); unmarshalErr != nil {
		t.Fatalf("phase failure with --json should emit JSON stdout: %v\n%s", unmarshalErr, out)
	}
	errorText, _ := failure["error"].(string)
	if failure["ok"] != false || !strings.Contains(errorText, "cannot enter pr phase") {
		t.Fatalf("unexpected structured failure payload: %#v", failure)
	}
}

func TestRunIssueOpsWorktreePrepareToolsRunsCodeGraphAgainstWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "codegraph.log")
	codegraph := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+logPath+"'\ncase \"$1\" in\nstatus) exit 1 ;;\ninit) exit 0 ;;\n*) exit 0 ;;\nesac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := makeIssueOpsCLIRepoForTest(t, "example")
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "1-demo")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatal(err)
	}
	id := record["id"].(string)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-demo", "--base-branch", "main", "--link-verified", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", worktree, "--json"})
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	var prepared map[string]any
	if err := json.Unmarshal([]byte(out), &prepared); err != nil {
		t.Fatalf("prepare-tools should return JSON: %v\n%s", err, out)
	}
	if prepared["codegraph_ready"] != true || prepared["codegraph_project_path"] != worktree {
		t.Fatalf("unexpected prepare-tools result: %#v", prepared)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "status "+worktree) || !strings.Contains(string(log), "init -i "+worktree) {
		t.Fatalf("codegraph should be checked and initialized against worktree, got:\n%s", log)
	}
}

func TestRunIssueOpsWorktreePrepareToolsFailureWithJSONEmitsStructuredError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "prepare-tools-json-failure")
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatalf("start should return JSON: %v\n%s", err, start)
	}
	id, ok := record["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected start record: %#v", record)
	}

	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	if err == nil {
		t.Fatalf("prepare-tools without linked worktree should still return an error")
	}
	var failure map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &failure); unmarshalErr != nil {
		t.Fatalf("prepare-tools failure with --json should emit JSON stdout: %v\n%s", unmarshalErr, out)
	}
	errorText, _ := failure["error"].(string)
	if failure["ok"] != false || !strings.Contains(errorText, "worktree_path is required") {
		t.Fatalf("unexpected structured failure payload: %#v", failure)
	}
}

func TestRunIssueOpsWorktreePrepareToolsInstallsPnpmDependencies(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tools.log")
	codegraph := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nprintf 'codegraph %s\\n' \"$*\" >> '"+logPath+"'\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	pnpm := filepath.Join(bin, "pnpm")
	if err := os.WriteFile(pnpm, []byte("#!/bin/sh\nprintf 'pnpm cwd=%s args=%s\\n' \"$PWD\" \"$*\" >> '"+logPath+"'\nmkdir -p node_modules\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo := makeIssueOpsCLIRepoForTest(t, "example")
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "1-demo")
	if err := os.WriteFile(filepath.Join(worktree, "package.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", repo, "--branch", "1-demo", "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(start), &record); err != nil {
		t.Fatal(err)
	}
	id := record["id"].(string)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-issue", "--id", id, "--issue-url", "https://github.com/example/repo/issues/1", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", id, "--provider", "github", "--issue-url", "https://github.com/example/repo/issues/1", "--branch", "1-demo", "--base-branch", "main", "--link-verified", "--json"})
	})
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"link-worktree", "--id", id, "--worktree-path", worktree, "--json"})
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"worktree", "prepare-tools", "--id", id, "--json"})
	})
	var prepared map[string]any
	if err := json.Unmarshal([]byte(out), &prepared); err != nil {
		t.Fatalf("prepare-tools should return JSON: %v\n%s", err, out)
	}
	if prepared["package_manager"] != "pnpm" || prepared["dependencies_ready"] != true || prepared["dependencies_action"] != "pnpm_install" {
		t.Fatalf("prepare-tools should install pnpm dependencies: %#v", prepared)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(log)
	if !strings.Contains(text, "pnpm cwd="+worktree+" args=install --frozen-lockfile --prefer-offline") {
		t.Fatalf("pnpm install should run in worktree, got:\n%s", text)
	}
	if !strings.Contains(text, "codegraph status "+worktree) {
		t.Fatalf("codegraph should still be checked, got:\n%s", text)
	}
}

func makeIssueOpsCLIRepoForTest(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func captureStdoutAndErrorForIssueOps(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	closeErr := w.Close()
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return string(out), runErr
}

func makeIssueOpsCLIWorktreeForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func writeIssueOpsCLIFileForTest(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
