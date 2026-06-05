package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func hookTempRepoWithDoc(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", "ARCHITECTURE.md"), []byte("# Arch\n\n## 경계\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func runHookCapture(t *testing.T, stdinJSON string, fn func() error) map[string]any {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() { _, _ = io.WriteString(w, stdinJSON); _ = w.Close() }()
	defer func() { os.Stdin = oldStdin }()
	out := captureStdoutForTest(t, func() {
		if err := fn(); err != nil {
			t.Fatalf("hook: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook output is not JSON: %q: %v", out, err)
	}
	return obj
}

func hookAdditionalContext(obj map[string]any) string {
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return ""
	}
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func TestRunHookRecordsFailureEvent(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_, _ = io.WriteString(w, `{"cwd":"/repo","tool_name":"Bash","tool_input":{"command":"echo TOKEN=secret-value && rg hook cmd"}}`)
	_ = w.Close()
	defer func() { os.Stdin = oldStdin }()

	err = runHook([]string{"unknown-hook", "--token=secret-value"})
	if err == nil {
		t.Fatal("runHook() error = nil")
	}
	body, readErr := os.ReadFile(filepath.Join(stateDir, "hook-failures.jsonl"))
	if readErr != nil {
		t.Fatalf("read hook failure log: %v", readErr)
	}
	text := string(body)
	if !strings.Contains(text, `"hook":"unknown-hook"`) || !strings.Contains(text, "unknown hook subcommand") {
		t.Fatalf("hook failure log missing event details: %s", text)
	}
	if !strings.Contains(text, `"tool":"Bash"`) || !strings.Contains(text, "command_snippet") {
		t.Fatalf("hook failure log missing payload details: %s", text)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("hook failure log leaked secret: %s", text)
	}
}

func TestRunHookFailuresJSON(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	if err := os.WriteFile(filepath.Join(stateDir, "hook-failures.jsonl"), []byte(`{"timestamp":"2026-06-02T00:00:00Z","hook":"pre-tool-use","error":"failed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForTest(t, func() {
		if err := runHook([]string{"failures", "--json"}); err != nil {
			t.Fatalf("runHook failures: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("hook failures output is not JSON: %q: %v", out, err)
	}
	if obj["ok"] != true || obj["path"] == "" {
		t.Fatalf("unexpected hook failures output: %+v", obj)
	}
	events, _ := obj["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected one event, got %+v", obj)
	}
}

func TestRunHookUserPromptDropsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"prompt":"x","cwd":"`+repo+`"}`, func() error { return runHookUserPrompt(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("user-prompt must not carry a catalog systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); strings.Contains(ctx, "project docs (read what's relevant):") || strings.Contains(ctx, "📚") {
		t.Fatalf("user-prompt must not inject the project-doc catalog: %q", ctx)
	}
}

func TestRunHookUserPromptAgyHintsAreOptIn(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	input := `{"prompt":"이 계획을 검토하고 개선점을 분석해줘","cwd":"` + repo + `"}`
	disabled := runHookCapture(t, input, func() error { return runHookUserPrompt(nil) })
	if strings.Contains(hookAdditionalContext(disabled), "agy -p") {
		t.Fatalf("agy hint should be disabled by default: %q", hookAdditionalContext(disabled))
	}
	enabled := runHookCapture(t, input, func() error { return runHookUserPrompt([]string{"--enable-agy-hints"}) })
	if !strings.Contains(hookAdditionalContext(enabled), "agy -p for LLM second-pass review") {
		t.Fatalf("agy hint should be enabled by flag: %q", hookAdditionalContext(enabled))
	}
}

func TestRunHookUserPromptRoutesVCSRemoteWorkToCLIFirstWithMCPFallback(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	input := `{"prompt":"GitLab MR 코멘트를 확인하고 이슈를 업데이트해줘","cwd":"` + repo + `"}`
	obj := runHookCapture(t, input, func() error { return runHookUserPrompt(nil) })
	ctx := hookAdditionalContext(obj)
	if !strings.Contains(ctx, "VCS remote work: use authenticated CLI first") {
		t.Fatalf("VCS prompt should include CLI-first guidance, got %q", ctx)
	}
	if !strings.Contains(ctx, "MCP fallback") || !strings.Contains(ctx, "do not print tokens") {
		t.Fatalf("VCS prompt should include MCP fallback and token hygiene guidance, got %q", ctx)
	}
}

func TestRunHookSessionStartInjectsCatalogClaude(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart(nil) })
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("SessionStart should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("SessionStart should inject the compact catalog additionalContext: %q", ctx)
	}
}

func TestRunHookSessionStartCodexOmitsSystemMessage(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"startup"}`, func() error { return runHookSessionStart([]string{"--host", "codex"}) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("Codex SessionStart must omit systemMessage: %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "• ARCHITECTURE.md") || strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("Codex SessionStart additionalContext should be the readable catalog view: %q", ctx)
	}
}

func TestRunHookSessionStartSkipsOnCompactSource(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`","source":"compact"}`, func() error { return runHookSessionStart(nil) })
	if _, ok := obj["systemMessage"]; ok {
		t.Fatalf("compact-source SessionStart should not inject (PostCompact owns it): %+v", obj)
	}
	if ctx := hookAdditionalContext(obj); ctx != "" {
		t.Fatalf("compact-source SessionStart should emit no additionalContext: %q", ctx)
	}
}

func TestRunHookPostCompactInjectsCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact(nil) })
	if ctx := hookAdditionalContext(obj); !strings.Contains(ctx, "project docs (read what's relevant):") {
		t.Fatalf("PostCompact should re-inject the catalog after compaction: %q", ctx)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") {
		t.Fatalf("PostCompact (claude) should show the pretty catalog via systemMessage: %v", obj["systemMessage"])
	}
}

func TestRunHookPostCompactCodexEmitsCompatibleSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := hookTempRepoWithDoc(t)
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPostCompact([]string{"--host", "codex"}) })
	if _, ok := obj["hookSpecificOutput"]; ok {
		t.Fatalf("Codex PostCompact must not emit hookSpecificOutput: %+v", obj)
	}
	if _, ok := obj["additionalContext"]; ok {
		t.Fatalf("Codex PostCompact must not emit additionalContext: %+v", obj)
	}
	if sysMsg, _ := obj["systemMessage"].(string); !strings.Contains(sysMsg, "📚") || !strings.Contains(sysMsg, "ARCHITECTURE.md") {
		t.Fatalf("Codex PostCompact should use supported systemMessage catalog: %v", obj["systemMessage"])
	}
}

func TestRunHookPreCompactEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`"}`, func() error { return runHookPreCompact(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreCompact hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseEmitsCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error { return runHookPreToolUse(nil) })
	if len(obj) != 0 {
		t.Fatalf("PreToolUse hook host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookPreToolUseRawJSONIsAllowByDefault(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Edit","tool_input":{"file_path":"x.go"}}`, func() error {
		return runHookPreToolUse([]string{"--json"})
	})
	if obj["decision"] != "allow" || obj["source"] != "pre-tool-use" || obj["tool"] != "Edit" {
		t.Fatalf("unexpected PreToolUse raw result: %+v", obj)
	}
}

func TestRunHookPreToolUseEnforcesSearchRoutingForStructuralSourceSearch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg -n \"func Run\" cmd internal"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" || obj["tool"] != "Bash" {
		t.Fatalf("expected enforced source search to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "CodeGraph") || !strings.Contains(reason, "codegraph_context") {
		t.Fatalf("block reason should point agents to CodeGraph, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsLiteralEvidenceSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"response_contracts\" cmd/harness/testdata README.md"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected docs/golden literal search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseHostJSONBlocksWhenSearchRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected host hook JSON to block enforced source search, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsExternalSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"grep -R \"PostToolUse\" -n /Applications/Codex.app/Contents/Resources"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected external binary/app search to remain allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostUsesPermissionDecisionWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"rg \"type Hook\" internal/core"}}`, func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-search-routing"})
	})
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "deny" {
		t.Fatalf("expected Claude PreToolUse permission denial, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksCodeGraphForExactSearchWhenRoutingEnforced(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"codegraph_context","tool_input":{"query":"DATABASE_URL"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-search-routing", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected exact CodeGraph query to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "Use rg first") {
		t.Fatalf("block reason should point agents to rg, got %q", reason)
	}
}

func TestRunHookPreToolUseEnforcesIssueOpsWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	obj := runHookCapture(t, `{"cwd":"`+source+`","tool_name":"apply_patch","tool_input":{"file_path":"`+source+`/.agent-harness/OPERATIONS.md"}}`, func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--expected-worktree", worktree, "--source-checkout", source, "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected source checkout edit to be blocked, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksIssueBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git checkout -b 2387-fix-grpc-ai-dmm-tag-replication-lag",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected issue-number branch creation without source ref to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "source ref") || !strings.Contains(reason, "2387-fix-grpc-ai-dmm-tag-replication-lag") {
		t.Fatalf("expected source-ref guidance, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsIssueBranchCreationWithSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git switch -c 2386-remove-dmm-ranking-ranktype origin/main",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected issue branch creation with source ref to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksBranchCreationWithoutSourceRef(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "git switch -c 2386-remove-dmm-ranking-ranktype",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected branch creation without source ref to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "source ref") || !strings.Contains(reason, "ask the user") {
		t.Fatalf("expected source-ref guidance, got %q", reason)
	}
}

func TestRunHookPreToolUseEnforcesLinkedIssueOpsWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/12",
		Branch:       "12-issue-worktree",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(source), "agent-harness.worktrees", "12-issue-worktree")
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	writeHookFixtureFile(t, worktree, "plans/issue-worktree.md", "plan\n")
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), record.ID, filepath.Join(worktree, "plans", "issue-worktree.md")); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "apply_patch",
		"tool_input": map[string]any{
			"file_path": filepath.Join(source, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected linked IssueOps worktree guard to block source checkout edit, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "linked IssueOps worktree") {
		t.Fatalf("expected linked worktree reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksSourceCheckoutWhenLinkedCycleExists(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: source, Branch: "12-issue-worktree"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/12",
		Branch:       "12-issue-worktree",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(source), "agent-harness.worktrees", "12-issue-worktree")
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/12-issue-worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(worktree, "plans", "demo.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), record.ID, planPath); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path": filepath.Join(source, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected linked IssueOps worktree from another branch to block source checkout edit, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "linked IssueOps worktree") {
		t.Fatalf("expected linked worktree reason, got %q", reason)
	}

	payload, err = json.Marshal(map[string]any{
		"cwd":       source,
		"tool_name": "Edit",
		"tool_input": map[string]any{
			"file_path": filepath.Join(worktree, "internal", "core", "issueops.go"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj = runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-worktree", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected linked IssueOps worktree edit to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseEnforcesKoreanRemoteArtifacts(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `gh pr create --title "Document split and IssueOps guardrails" --body "Summary Changes Verification Risk"`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected English PR artifact to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected Korean remote artifact gate reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsRemoteArtifactEditWithoutBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab issue edit 123 --add-label bug`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("remote artifact edit without body should not require title/body text, got %+v", obj)
	}
}

func TestRunHookPreToolUseInspectsGitLabDescriptionFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "issue.md")
	if err := os.WriteFile(bodyFile, []byte("This issue body is English prose and should be blocked before remote creation.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab issue create --title "Remote artifact check" --description-file issue.md --label bug --assignee habin`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected English GitLab description file to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected Korean remote artifact gate reason, got %q", reason)
	}
}

func TestRunHookPreToolUseEnforcesRemoteCreateAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab mr create --title "IssueOps 담당자 검증" --description "라벨은 있지만 담당자 없는 MR 생성을 막습니다." --label bug`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected missing assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredRemoteCreateReadsGlabFlagsAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"title":             "IssueOps 담당자 검증",
				"description":       "이슈 라벨 복사와 담당자를 함께 지정합니다.",
				"copy_issue_labels": true,
				"assignee":          []any{"m16khb"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected structured labeled and assigned MR create to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredRemoteCreateStillRequiresAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"title":             "IssueOps 담당자 검증",
				"description":       "이슈 라벨 복사 옵션이 있어도 담당자는 필요합니다.",
				"copy_issue_labels": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured MR create without assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForRequiresAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured glab mr for without assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForAllowsAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
				"assignee":    "100",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected structured glab mr for with assignee to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForRejectsUsernameAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
				"assignee":    "m16khb",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured glab mr for username assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "numeric assignee") {
		t.Fatalf("expected numeric assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsIssueBasedGitLabMRWithCombinedGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab mr for 2385 --with-labels --assignee 100`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected issue-based GitLab MR with labels and numeric assignee to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredRelatedIssueMRRequiresNumericAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"related_issue":     "2385",
				"copy_issue_labels": true,
				"assignee":          "m16khb",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured related-issue MR with username assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "numeric assignee") {
		t.Fatalf("expected numeric assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseEnforcesGitOpsKubectl(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `kubectl patch deployment/api -n prod -p '{"spec":{"replicas":1}}'`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected direct kubectl mutation to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "GitOps") || !strings.Contains(reason, "kubectl") {
		t.Fatalf("expected GitOps kubectl reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `kubectl port-forward svc/api 8080:80 -n prod`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl", "--json"})
	})
	if obj["decision"] != "ask" {
		t.Fatalf("expected kubectl live access to ask for confirmation, got %+v", obj)
	}
}

func TestRunHookPreToolUseAsksForBroadStagedChecks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check apps libs"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `npm run lint:check`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-staged-checks", "--json"})
	})
	if obj["decision"] != "ask" {
		t.Fatalf("expected broad staged check to ask for confirmation, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "staged") || !strings.Contains(reason, "apps/libs") {
		t.Fatalf("expected staged check reason, got %q", reason)
	}
}

func TestRunHookPreToolUseCodexHostAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `kubectl port-forward svc/api 8080:80 -n prod`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-gitops-kubectl"})
	})
	if obj["decision"] != nil {
		t.Fatalf("Codex host ask must not use legacy decision field, got %+v", obj)
	}
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "ask" {
		t.Fatalf("expected Codex PreToolUse ask decision, got %+v", obj)
	}
}

func TestRunHookPreToolUseClaudeHostAsksForKubectlLiveAccess(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `kubectl exec -it pod/api-0 -- sh`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--host", "claude", "--enforce-gitops-kubectl"})
	})
	hso, _ := obj["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PreToolUse" || hso["permissionDecision"] != "ask" {
		t.Fatalf("expected Claude PreToolUse ask decision, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksPlanLinkSectionInIssueBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "body.md")
	if err := os.WriteFile(bodyFile, []byte("## Problem\n\n문제 설명입니다.\n\n## Plan Link\n\nTBD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": `gh issue create --title "이슈" --body-file body.md --label bug --assignee @me`},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected Plan Link section to be blocked, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "Plan Link") {
		t.Fatalf("expected Plan Link reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksGitLabRelatedIssuesBodySection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "glab issue create --title 이슈 --description \"## Problem\n\n설명\n\n## Related Issues\n\n- #1\"",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected GitLab Related Issues body section to be blocked, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "linked items") {
		t.Fatalf("expected GitLab linked items reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksEnglishMCPRemoteArtifact(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, tc := range []struct {
		name  string
		tool  string
		input map[string]any
	}{
		{
			name: "gitlab issue",
			tool: "mcp__glab__create_issue",
			input: map[string]any{
				"title":       "Investigate routing regression",
				"description": "The implementation should verify the failing route and update the service contract.",
				"labels":      []any{"bug"},
				"assignee":    "@me",
			},
		},
		{
			name: "gitlab mr",
			tool: "mcp__gitlab__create_merge_request",
			input: map[string]any{
				"title":       "Fix adult routing regression",
				"description": "This merge request updates the service and documents the verification evidence.",
				"labels":      []any{"bug"},
				"assignee":    "@me",
			},
		},
		{
			name: "github pr",
			tool: "mcp__github__create_pull_request",
			input: map[string]any{
				"title":    "Fix adult routing regression",
				"body":     "This pull request updates the service and documents the verification evidence.",
				"labels":   []any{"bug"},
				"assignee": "@me",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"cwd":        repo,
				"tool_name":  tc.tool,
				"tool_input": tc.input,
			})
			if err != nil {
				t.Fatal(err)
			}
			obj := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
			})
			if obj["decision"] != "block" {
				t.Fatalf("expected English MCP remote artifact creation to be blocked, got %+v", obj)
			}
			if reason, _ := obj["reason"].(string); !strings.Contains(reason, "IssueOps remote artifact gate failed") {
				t.Fatalf("expected Korean gate reason, got %q", reason)
			}
		})
	}
}

func TestRunHookPreToolUseRemoteArtifactGateIgnoresCodeGraphQueryText(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__codegraph__codegraph_explore",
		"tool_input": map[string]any{
			"query": `glab mr create --title "IssueOps 담당자 검증" --description "라벨과 담당자 누락을 설명하는 탐색 문자열"`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected CodeGraph query text to bypass remote artifact gates, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsKoreanGitLabMCPRemoteArtifact(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__create_issue",
		"tool_input": map[string]any{
			"title":       "성인 라우팅 회귀 원인 조사",
			"description": "문제 배경과 재현 경로를 정리하고, 변경 후 검증 명령과 운영 확인 결과를 이슈 본문에 기록합니다.",
			"labels":      []any{"bug"},
			"assignee":    "@me",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected Korean GitLab MCP issue creation to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsGitHubRelatedIssuesBodySection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "body.md")
	if err := os.WriteFile(bodyFile, []byte("## Problem\n\n설명입니다.\n\n## Related Issues\n\n- #1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": `gh issue create --title "이슈" --body-file body.md --label bug --assignee @me`},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] == "block" {
		t.Fatalf("GitHub body references are valid and must not be blocked, got %+v", obj)
	}
}

func TestRunHookPostToolUseDoesNotAutoQueueDraftWiki(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	raw := runHookCapture(t, `{
  "cwd": "`+repo+`",
  "tool_name": "Bash",
  "tool_input": {"command": "claude-mem export observations"},
  "tool_response": "A durable observation says this might be reusable draft wiki material."
}`, func() error {
		return runHookPostToolUse([]string{"--json"})
	})
	if _, ok := raw["draft_wiki_queue"]; ok {
		t.Fatalf("PostToolUse must not auto-queue draft wiki material; main agent must judge and call project draft-wiki queue explicitly: %+v", raw)
	}
}

func TestRunHookPostToolUseEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","tool_input":{"command":"true"},"tool_response":"ok"}`, func() error {
		return runHookPostToolUse(nil)
	})
	if len(obj) != 0 {
		t.Fatalf("PostToolUse host output must be a no-op object, got %+v", obj)
	}
}

func TestRunHookStopEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForTest(t, func() {
		if err := runHookStop([]string{"--repo", repo}); err != nil {
			t.Fatalf("runHookStop: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("stop hook output is not JSON: %q: %v", out, err)
	}
	if len(obj) != 0 {
		t.Fatalf("Stop hook host output must be a no-op object, got %s", out)
	}
	if strings.Contains(out, "hookSpecificOutput") || strings.Contains(out, "additionalContext") {
		t.Fatalf("Stop hook output contains unsupported injection fields: %s", out)
	}
}

func TestRunHookStopBlocksMissingNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	// continue must stay true so the agent continues in-turn and presents the
	// choices; continue:false would hard-stop and surface the reason to the user.
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopAllowsStopWhenStopHookActiveMissingChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress the next-action block to avoid loops, got %+v", obj)
	}
}

func TestRunHookStopBlockReasonTellsAgentToPresentContextSpecificChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	message := "완료했습니다.\n\nCommit: 18172349f3 fix(dmm): use persona tags for ranking registration\nIssueOps phase: pr, strict readiness 통과"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+strings.ReplaceAll(message, "\n", "\\n")+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "caused the block") || !strings.Contains(reason, "context-specific") {
		t.Fatalf("expected Stop hook reason to tell the agent why it blocked and to create context-specific choices, got %q", reason)
	}
}

func TestRunHookStopAllowsNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"선택지:\n1. 진행: 검증합니다. (추천)\n2. 축소 진행: 일부만 합니다.\n3. 보류: 멈춥니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("expected Stop hook to allow numbered choices with no-op output, got %+v", obj)
	}
}

func TestRunHookStopBlocksNumberedNextActionsWithoutOneRecommendation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, message := range []string{
		"선택지:\\n1. 진행: 검증합니다.\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다.",
		"선택지:\\n1. 진행: 검증합니다. (추천)\\n2. 축소 진행: 일부만 합니다. (추천)\\n3. 보류: 멈춥니다.",
	} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+message+`"}`, func() error {
			return runHookStop([]string{"--enforce-numbered-next-actions"})
		})
		if obj["continue"] != true || obj["decision"] != "block" {
			t.Fatalf("expected malformed recommendations to block, got %+v", obj)
		}
	}
}

func TestRunHookStopRelaysRecommendedNextActionFactsToMainAgent(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 검증합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to re-enter main agent with observed facts, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	for _, want := range []string{"판단 지점", "근거", "메인 에이전트", "추천 선택지"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("expected factual trigger directive containing %q, got %q", want, reason)
		}
	}
	for _, banned := range []string{"점수", "임계값", "자동진행 후보", "destructive", "eligible", "candidate", "되돌릴 수"} {
		if strings.Contains(reason, banned) {
			t.Fatalf("Stop hook reason must not include hook judgement wording %q: %q", banned, reason)
		}
	}
}

func TestRunHookStopRelaysSameNextActionChoicesOnlyOnce(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "사용자 결정이 필요한 상태입니다.\\n\\n선택지:\\n1. (추천) 사용자가 외부 담당자에게 문의문을 전달하고 답변을 공유한다.\\n2. 임시 조치 변경안을 검토하라고 지시한다.\\n3. 식별자 분리 설계안을 검토하라고 지시한다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	second := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("duplicate next-action choices must not re-enter the agent, got %+v", second)
	}
}

func TestRunHookStopSuppressesRephrasedNextActionChoicesUntilProgress(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	firstMsg := "현재는 사용자 액션이 필요합니다.\\n\\n선택지:\\n1. (추천) 사용자가 FANZA 담당자에게 문의문을 전달하고 답변을 공유한다.\\n2. FANZA 답변 전 임시 변경안을 검토하라고 지시한다.\\n3. FANZA용 character_id 분리 설계를 검토하라고 지시한다."
	secondMsg := "같은 외부 확인 지점입니다.\\n\\n선택지:\\n1. (추천) 사용자가 담당자에게 문의문을 전달하고, 답변을 여기로 공유한다.\\n2. 임시 조치로 DELETE 중단 변경안을 검토하라고 지시한다.\\n3. 동일 id 복구 불가를 전제로 분리 설계안을 검토하라고 지시한다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+firstMsg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	second := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+secondMsg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("rephrased unresolved next-action choices must not re-enter the agent before progress, got %+v", second)
	}
}

func TestRunHookPostToolUseClearsSuppressedNextActionRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	runHookCapture(t, `{"cwd":"`+repo+`","tool_name":"Bash","command":"true"}`, func() error {
		return runHookPostToolUse(nil)
	})
	afterProgress := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if afterProgress["continue"] != true || afterProgress["decision"] != "block" {
		t.Fatalf("expected PostToolUse to clear relay suppression, got %+v", afterProgress)
	}
}

func TestRunHookUserPromptClearsSuppressedNextActionRelay(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["continue"] != true || first["decision"] != "block" {
		t.Fatalf("expected first Stop hook call to relay next-action facts, got %+v", first)
	}
	runHookCapture(t, `{"cwd":"`+repo+`","prompt":"계속 진행해"}`, func() error {
		return runHookUserPrompt(nil)
	})
	afterPrompt := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if afterPrompt["continue"] != true || afterPrompt["decision"] != "block" {
		t.Fatalf("expected UserPromptSubmit to clear relay suppression, got %+v", afterPrompt)
	}
}

func TestRunHookStopKeepsAutoProceedFlagAsRelayAlias(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--auto-proceed-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("deprecated auto-proceed flag should remain a relay alias, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "판단 지점") || strings.Contains(reason, "점수") {
		t.Fatalf("expected alias to use factual relay wording, got %q", reason)
	}
}

func TestRunHookStopDoesNotTreatNumberedExplanationAsAutoProceedChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := strings.Join([]string{
		"설명입니다.",
		"1. Stop hook이 먼저 정적 휴리스틱으로 자동진행 후보를 판정합니다.",
		"2. 메인 에이전트는 실행 여부만 판단합니다.",
		"3. `agent-harness`가 추천 선택지를 분석해서 자동진행 후보라고 판단합니다.",
	}, "\\n")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices, got %+v", obj)
	}
	if !strings.Contains(reason, "lacks well-formed numbered next actions") {
		t.Fatalf("expected missing-next-actions block, got %+v", obj)
	}
	if strings.Contains(reason, "자동진행 후보") {
		t.Fatalf("numbered explanatory text must not be treated as auto-proceed choices, got %q", reason)
	}
}

func TestRunHookStopDoesNotAutoProceedDestructiveCleanup(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 정리 진행: merged worktree와 branch를 삭제합니다. (추천)\\n2. 보류: 유지합니다.\\n3. 확장 정리: 전체를 점검합니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected facts relay block for destructive-looking text, got %+v", obj)
	}
	if !strings.Contains(reason, "판단 지점") || !strings.Contains(reason, "추천 선택지") {
		t.Fatalf("expected factual trigger reason, got %+v", obj)
	}
	for _, banned := range []string{"destructive", "irreversible", "점수", "임계값", "자동 진행 미적용"} {
		if strings.Contains(reason, banned) {
			t.Fatalf("Stop hook must not judge destructive-looking text with %q: %+v", banned, obj)
		}
	}
}

func TestRunHookStopSuppressesNextActionRelayWhenStopHookActive(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress next-action relay to avoid Stop hook loops, got %+v", obj)
	}
}

func TestRunHookStopDoesNotRelayNextActionJudgementWithoutFlag(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// The relay flag is the on/off switch. Even with a valid recommended choice
	// present, omitting it must not re-enter the main agent.
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("next-action judgement relay must require its flag; got %+v", obj)
	}
}

func TestRunHookStopReadsLastAssistantMessageFromTranscript(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"작업했습니다."}]}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to inspect transcript and block with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopIgnoresSystemTranscriptObjectWithAssistantText(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := `{"type":"system","text":"assistant reminder without a final assistant response"}` + "\r\n"
	if err := os.WriteFile(transcript, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--json"})
	})
	next, _ := obj["numbered_next_actions"].(map[string]any)
	if next["decision"] != "allow" || next["reason"] != "no assistant message available to inspect" {
		t.Fatalf("expected system transcript object to be ignored, got %+v", obj)
	}
}

func captureStdoutForTest(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeHookFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
