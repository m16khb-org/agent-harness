package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core"
)

func TestResponseContractsGolden(t *testing.T) {
	stateDir := t.TempDir()
	workspaceDir := t.TempDir()
	gitRepoDir := makeGitRepoForContract(t)
	homeDir := t.TempDir()
	auditLog := filepath.Join(t.TempDir(), "audit.jsonl")
	workerDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_AUDIT_LOG", auditLog)
	t.Setenv("HARNESS_WORKER_DIR", workerDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CODEX_HOME", filepath.Join(homeDir, ".codex"))

	replacements := map[string]string{
		stateDir:      "$STATE_DIR",
		workspaceDir:  "$WORKSPACE",
		gitRepoDir:    "$GIT_REPO",
		harnessRoot(): "$HARNESS_ROOT",
		homeDir:       "$HOME",
		auditLog:      "$AUDIT_LOG",
		workerDir:     "$WORKER_DIR",
	}
	addEvalSymlinkReplacement(t, replacements, stateDir, "$STATE_DIR")
	addEvalSymlinkReplacement(t, replacements, workspaceDir, "$WORKSPACE")
	addEvalSymlinkReplacement(t, replacements, gitRepoDir, "$GIT_REPO")
	addEvalSymlinkReplacement(t, replacements, harnessRoot(), "$HARNESS_ROOT")
	addEvalSymlinkReplacement(t, replacements, workerDir, "$WORKER_DIR")
	addEvalSymlinkReplacement(t, replacements, filepath.Dir(auditLog), "$AUDIT_DIR")

	snapshot := map[string]any{}
	cliSnapshot := map[string]any{}
	snapshot["cli"] = cliSnapshot
	cliSnapshot["inspect"] = runCLIJSONContract(t, replacements, func() error {
		return runInspect([]string{"--json", "--repo", workspaceDir})
	})
	cliSnapshot["docs_index"] = runCLIJSONContract(t, replacements, func() error {
		return runDocs([]string{"--json"})
	})
	cliSnapshot["preflight"] = runCLIJSONContract(t, replacements, func() error {
		return runPreflight([]string{"--json", gitRepoDir})
	})
	cliSnapshot["verify_work"] = runCLIJSONContract(t, replacements, func() error {
		return runVerifyWork([]string{"--repo", gitRepoDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["policy_check"] = runCLIJSONContract(t, replacements, func() error {
		return runPolicy([]string{"check", "--workspace-root", workspaceDir, "--cwd", workspaceDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["state_write"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"write", "--key", "current", "--value", "current content", "--json"})
	})
	cliSnapshot["state_read"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"read", "--key", "current", "--json"})
	})
	cliSnapshot["state_list"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"list", "--json"})
	})
	issueopsStartStdout := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", workspaceDir, "--branch", "1-contract-branch", "--json"})
	})
	var issueopsStartRaw map[string]any
	if err := json.Unmarshal([]byte(issueopsStartStdout), &issueopsStartRaw); err != nil {
		t.Fatalf("unmarshal issueops start JSON %q: %v", issueopsStartStdout, err)
	}
	cliSnapshot["issueops_start"] = normalizeContractValue(issueopsStartRaw, replacements)
	issueopsID, ok := issueopsStartRaw["id"].(string)
	if !ok || issueopsID == "" {
		t.Fatalf("issueops start missing id: %#v", issueopsStartRaw)
	}
	cliSnapshot["issueops_link_issue"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-issue", "--id", issueopsID, "--issue-url", "https://gitlab.example/group/project/-/issues/1", "--json"})
	})
	cliSnapshot["issueops_prepare_branch"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", issueopsID, "--provider", "gitlab", "--issue-url", "https://gitlab.example/group/project/-/issues/1", "--branch", "1-contract-branch", "--base-branch", "main", "--link-verified", "--json"})
	})
	contractWorktree := filepath.Join(filepath.Dir(workspaceDir), filepath.Base(workspaceDir)+".worktrees", "1-contract-branch")
	if err := os.MkdirAll(contractWorktree, 0o755); err != nil {
		t.Fatal(err)
	}
	replacements[contractWorktree] = "$ISSUEOPS_WORKTREE"
	addEvalSymlinkReplacement(t, replacements, contractWorktree, "$ISSUEOPS_WORKTREE")
	cliSnapshot["issueops_link_worktree"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-worktree", "--id", issueopsID, "--worktree-path", contractWorktree, "--json"})
	})
	writeContractFile(t, contractWorktree, "docs/superpowers/plans/contract.md", "plan\n")
	cliSnapshot["issueops_link_plan"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-plan", "--id", issueopsID, "--plan-path", filepath.Join(contractWorktree, "docs", "superpowers", "plans", "contract.md"), "--json"})
	})
	cliSnapshot["issueops_link_child"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-child", "--id", issueopsID, "--child-url", "https://gitlab.example/group/project/-/issues/2", "--title", "contract child", "--json"})
	})
	cliSnapshot["issueops_feedback_add"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"feedback", "add", "--id", issueopsID, "--source", "user", "--body", "tighten contract", "--json"})
	})
	cliSnapshot["issueops_pr_readiness"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", issueopsID, "--json"})
	})

	old := mustStateReadForContract(t, "current")
	old.Record.Key = "old"
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	mustWriteStateRecordForContract(t, stateDir, "old", old.Record)
	cliSnapshot["state_prune_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--json"})
	})
	cliSnapshot["state_prune_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--confirm", "--json"})
	})

	legacy := core.StateRecord{
		Key:       "legacy",
		Content:   "legacy content",
		UpdatedAt: "2000-01-01T00:00:00Z",
		Bytes:     len([]byte("legacy content")),
	}
	mustWriteStateRecordForContract(t, stateDir, "legacy", legacy)
	cliSnapshot["state_migrate_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--json"})
	})
	cliSnapshot["state_migrate_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--confirm", "--json"})
	})
	cliSnapshot["state_doctor_healthy"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	if err := os.WriteFile(filepath.Join(stateDir, "corrupt.json"), []byte("{not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliSnapshot["state_doctor_corrupt"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	writeSelfAugmentCompareFixturesForContract(t, stateDir)
	cliSnapshot["self_augment_plan"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"--target-score", "95", "--json"})
	})
	cliSnapshot["self_augment_lesson"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"lesson", "--candidate", "reflexion-state-memory", "--lesson", "Contract lesson", "--next-action", "Check stored lesson before next cycle", "--state-key", "self-augment-lesson-contract", "--json"})
	})
	cliSnapshot["self_verify_candidates"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"candidates", "--save-state", "--state-key", "self-verify-candidates-contract", "--json"})
	})
	cliSnapshot["self_verify_compare"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"compare", "--baseline-key", "self-verify-baseline", "--candidate-key", "self-verify-candidate", "--json"})
	})
	cliSnapshot["self_verify_promote_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"promote", "--from-key", "self-verify-candidate", "--baseline-key", "self-verify-promoted", "--json"})
	})
	cliSnapshot["self_verify_history"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"history", "--prefix", "self-verify", "--json"})
	})
	cliSnapshot["contract_schema"] = runCLIJSONContract(t, replacements, func() error {
		return runContract([]string{"schema", "--json"})
	})
	traceInput := filepath.Join(workspaceDir, "trace.jsonl")
	if err := os.WriteFile(traceInput, []byte(`{"kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"contract fixture","source":"contract"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliSnapshot["trace_analyze"] = runCLIJSONContract(t, replacements, func() error {
		return runTrace([]string{"analyze", "--input", traceInput, "--json"})
	})
	cliSnapshot["policy_audit"] = runCLIJSONContract(t, replacements, func() error {
		return runPolicy([]string{"audit", "--workspace-root", workspaceDir, "--cwd", workspaceDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["worker_enqueue"] = runCLIJSONContract(t, replacements, func() error {
		return runWorker([]string{"enqueue", "--kind", "contract", "--payload", "TOKEN=secret-value", "--json"})
	})

	mcpSnapshot := map[string]any{}
	snapshot["mcp"] = mcpSnapshot
	mcpSnapshot["harness_inspect"] = runMCPToolContract(t, replacements, "harness_inspect", map[string]any{
		"repo": workspaceDir,
	})
	mcpSnapshot["docs_index"] = runMCPToolContract(t, replacements, "docs_index", map[string]any{})
	mcpSnapshot["atomic_commit_preflight"] = runMCPToolContract(t, replacements, "atomic_commit_preflight", map[string]any{
		"path": gitRepoDir,
	})
	mcpSnapshot["command_policy_check"] = runMCPToolContract(t, replacements, "command_policy_check", map[string]any{
		"workspace_root": workspaceDir,
		"cwd":            workspaceDir,
		"argv":           []string{"git", "status", "--short"},
	})
	mcpSnapshot["command_policy_audit"] = runMCPToolContract(t, replacements, "command_policy_audit", map[string]any{
		"workspace_root": workspaceDir,
		"cwd":            workspaceDir,
		"argv":           []string{"git", "status", "--short"},
	})
	mcpSnapshot["contract_schema"] = runMCPToolContract(t, replacements, "contract_schema", map[string]any{})
	mcpSnapshot["worker_enqueue"] = runMCPToolContract(t, replacements, "worker_enqueue", map[string]any{
		"kind":    "contract",
		"payload": "TOKEN=secret-value",
	})
	mcpSnapshot["worker_run_read_only"] = runMCPToolContract(t, replacements, "worker_run_read_only", map[string]any{
		"kind":           "contract-run",
		"payload":        "TOKEN=secret-value",
		"workspace_root": gitRepoDir,
		"cwd":            gitRepoDir,
		"argv":           []string{"git", "status", "--short"},
	})
	mcpSnapshot["state_prune"] = runMCPToolContract(t, replacements, "state_prune", map[string]any{
		"max_age": "1h",
	})
	mcpSnapshot["state_doctor"] = runMCPToolContract(t, replacements, "state_doctor", map[string]any{})
	mcpSnapshot["state_migrate"] = runMCPToolContract(t, replacements, "state_migrate", map[string]any{})
	issueopsMCPStartRaw := runMCPToolContractRaw(t, "issueops_start", map[string]any{
		"repo":   workspaceDir,
		"branch": "2-mcp-contract",
	})
	mcpSnapshot["issueops_start"] = normalizeMCPTextJSON(normalizeContractValue(issueopsMCPStartRaw, replacements), replacements)
	issueopsMCPID, ok := issueopsMCPStartRaw["content"].([]any)[0].(map[string]any)["text"].(string)
	if !ok || issueopsMCPID == "" {
		t.Fatalf("MCP issueops start missing text: %#v", issueopsMCPStartRaw)
	}
	var issueopsMCPPayload map[string]any
	if err := json.Unmarshal([]byte(issueopsMCPID), &issueopsMCPPayload); err != nil {
		t.Fatalf("unmarshal MCP issueops start text: %v", err)
	}
	issueopsMCPID, ok = issueopsMCPPayload["id"].(string)
	if !ok || issueopsMCPID == "" {
		t.Fatalf("MCP issueops start missing id: %#v", issueopsMCPPayload)
	}
	mcpSnapshot["issueops_link_issue"] = runMCPToolContract(t, replacements, "issueops_link_issue", map[string]any{
		"id":        issueopsMCPID,
		"issue_url": "https://github.com/example/repo/issues/2",
	})
	mcpSnapshot["issueops_prepare_branch"] = runMCPToolContract(t, replacements, "issueops_prepare_branch", map[string]any{
		"id":            issueopsMCPID,
		"provider":      "github",
		"issue_url":     "https://github.com/example/repo/issues/2",
		"branch":        "2-mcp-contract",
		"base_branch":   "main",
		"link_verified": true,
	})
		mcpWorktree := filepath.Join(filepath.Dir(workspaceDir), filepath.Base(workspaceDir)+".worktrees", "2-mcp-contract")
		if err := os.MkdirAll(mcpWorktree, 0o755); err != nil {
			t.Fatal(err)
		}
	replacements[mcpWorktree] = "$MCP_ISSUEOPS_WORKTREE"
	addEvalSymlinkReplacement(t, replacements, mcpWorktree, "$MCP_ISSUEOPS_WORKTREE")
		mcpSnapshot["issueops_link_worktree"] = runMCPToolContract(t, replacements, "issueops_link_worktree", map[string]any{
			"id":            issueopsMCPID,
			"worktree_path": mcpWorktree,
		})
		writeContractFile(t, mcpWorktree, "docs/superpowers/plans/mcp-contract.md", "plan\n")
		mcpSnapshot["issueops_link_plan"] = runMCPToolContract(t, replacements, "issueops_link_plan", map[string]any{
			"id":        issueopsMCPID,
			"plan_path": filepath.Join(mcpWorktree, "docs", "superpowers", "plans", "mcp-contract.md"),
		})
		mcpSnapshot["issueops_link_child"] = runMCPToolContract(t, replacements, "issueops_link_child", map[string]any{
		"id":        issueopsMCPID,
		"child_url": "https://github.com/example/repo/issues/3",
		"title":     "MCP contract child",
	})
	mcpSnapshot["issueops_add_feedback"] = runMCPToolContract(t, replacements, "issueops_add_feedback", map[string]any{
		"id":     issueopsMCPID,
		"source": "review",
		"body":   "tighten MCP contract",
	})
	mcpSnapshot["issueops_pr_readiness"] = runMCPToolContract(t, replacements, "issueops_pr_readiness", map[string]any{
		"id": issueopsMCPID,
	})
	mcpSnapshot["self_augment"] = runMCPToolContract(t, replacements, "self_augment", map[string]any{
		"target_score": 95,
	})
	mcpSnapshot["self_augment_lesson"] = runMCPToolContract(t, replacements, "self_augment_lesson", map[string]any{
		"candidate_id": "reflexion-state-memory",
		"lesson":       "MCP lesson",
		"next_action":  "Check MCP lesson before next cycle",
		"state_key":    "self-augment-lesson-mcp",
	})
	mcpSnapshot["self_verify_candidates"] = runMCPToolContract(t, replacements, "self_verify_candidates", map[string]any{
		"save_state": true,
		"state_key":  "self-verify-candidates-mcp",
	})
	mcpSnapshot["self_verify_compare"] = runMCPToolContract(t, replacements, "self_verify_compare", map[string]any{
		"baseline_key":  "self-verify-baseline",
		"candidate_key": "self-verify-candidate",
	})
	mcpSnapshot["self_verify_promote"] = runMCPToolContract(t, replacements, "self_verify_promote", map[string]any{
		"from_key":     "self-verify-candidate",
		"baseline_key": "self-verify-promoted",
	})
	mcpSnapshot["self_verify_history"] = runMCPToolContract(t, replacements, "self_verify_history", map[string]any{
		"prefix": "self-verify",
	})

	assertJSONGolden(t, "response_contracts.golden.json", snapshot)
}

func makeGitRepoForContract(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitForContract(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# contract fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/contract\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, dir, "add", "README.md", "go.mod")
	runGitForContract(t, dir,
		"-c", "user.name=Contract Test",
		"-c", "user.email=contract@example.invalid",
		"commit", "-q",
		"-m", "docs(contract): add fixture",
		"-m", "Lore:\n- Intent: Normalize preflight contract.\n- Why: Response golden should cover git DTOs.\n- Changes:\n  - Add fixture README.\n- Verify: go test ./cmd/harness -run Golden\n- Risk: Low",
	)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func runGitForContract(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func addEvalSymlinkReplacement(t *testing.T, replacements map[string]string, path, token string) {
	t.Helper()
	eval, err := filepath.EvalSymlinks(path)
	if err == nil && eval != "" {
		replacements[eval] = token
	}
}

func writeContractFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSelfAugmentCompareFixturesForContract(t *testing.T, stateDir string) {
	t.Helper()
	summary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1000},
		},
	}
	fixtures := []struct {
		key         string
		elapsed     int64
		generatedAt string
	}{
		{key: "self-verify-baseline", elapsed: 1000, generatedAt: "2000-01-01T00:00:00Z"},
		{key: "self-verify-candidate", elapsed: 1100, generatedAt: "2000-01-01T00:01:00Z"},
	}
	for _, fixture := range fixtures {
		if err := writeSelfAugmentSnapshotRecord(stateDir, fixture.key, SelfAugmentStateSnapshot{
			SchemaVersion: 1,
			Kind:          "self_verification_summary",
			OK:            true,
			Iterations:    10,
			BaseSeed:      600,
			ElapsedMS:     fixture.elapsed,
			HarnessRoot:   harnessRoot(),
			GeneratedAt:   fixture.generatedAt,
			Summary:       summary,
		}); err != nil {
			t.Fatalf("write compare fixture %s: %v", fixture.key, err)
		}
	}
}

func runCLIJSONContract(t *testing.T, replacements map[string]string, fn func() error) any {
	t.Helper()
	stdout := captureStdoutForContract(t, fn)
	var value any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatalf("unmarshal CLI JSON %q: %v", stdout, err)
	}
	return normalizeContractValue(value, replacements)
}

func runMCPToolContract(t *testing.T, replacements map[string]string, name string, arguments map[string]any) any {
	t.Helper()
	value := runMCPToolContractRaw(t, name, arguments)
	return normalizeMCPTextJSON(normalizeContractValue(value, replacements), replacements)
}

func runMCPToolContractRaw(t *testing.T, name string, arguments map[string]any) map[string]any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := handleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("handleToolCall(%s): %+v", name, rpcErr)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("unexpected MCP result type %T", value)
	}
	return typed
}

func captureStdoutForContract(t *testing.T, fn func() error) string {
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
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}

func mustStateReadForContract(t *testing.T, key string) core.StateResult {
	t.Helper()
	result, err := core.StateRead(key)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustWriteStateRecordForContract(t *testing.T, stateDir, key string, record core.StateRecord) {
	t.Helper()
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, key+".json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func normalizeContractValue(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		isStateCheckpoint := looksLikeStateCheckpoint(v)
		isDynamicStateRecord := looksLikeDynamicStateRecord(v)
		isDynamicStateHistoryEntry := looksLikeDynamicStateHistoryEntry(v)
		isCommandRun := looksLikeCommandRunResult(v)
		for key, child := range v {
			if isDynamicTimeKey(key) {
				out[key] = "$TIMESTAMP"
				continue
			}
			if isCommandRun && key == "duration_ms" {
				out[key] = "$DURATION_MS"
				continue
			}
			if isStateCheckpoint && key == "bytes" {
				out[key] = "$STATE_BYTES"
				continue
			}
			if (isDynamicStateRecord || isDynamicStateHistoryEntry) && key == "bytes" {
				out[key] = "$STATE_RECORD_BYTES"
				continue
			}
			if key == "audit_log_id" {
				out[key] = "$AUDIT_ID"
				continue
			}
			// project_claude_skill / project_codex_skill report whether a repo-local
			// .claude/.codex skill link happens to exist on the running machine. These
			// links are gitignored, project-local install artifacts, so their presence
			// is machine state, not committed contract. Normalize them so the golden
			// does not flake for developers who have project-local skills installed.
			if key == "project_claude_skill" || key == "project_codex_skill" {
				out[key] = "$PROJECT_SKILL_PRESENCE"
				continue
			}
			if key == "id" {
				if s, ok := child.(string); ok && strings.HasPrefix(s, "job-") {
					out[key] = "$WORKER_JOB_ID"
					continue
				}
				if s, ok := child.(string); ok && strings.HasPrefix(s, "io-") {
					out[key] = "$ISSUEOPS_ID"
					continue
				}
			}
			if key == "head" || key == "sha" {
				out[key] = "$GIT_SHA"
				continue
			}
			out[key] = normalizeContractValue(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeContractValue(child, replacements))
		}
		return out
	case string:
		return normalizeContractString(v, replacements)
	default:
		return v
	}
}

func looksLikeDynamicStateRecord(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["schema_version"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func isDynamicStateRecordKey(key string) bool {
	return strings.HasPrefix(key, "self-augment-lesson-") ||
		strings.HasPrefix(key, "self-verify-candidates-") ||
		key == "self-verify-baseline" ||
		key == "self-verify-candidate" ||
		key == "self-verify-promoted"
}

func looksLikeDynamicStateHistoryEntry(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["generated_at"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeStateCheckpoint(value map[string]any) bool {
	if _, ok := value["state_dir"]; !ok {
		return false
	}
	if _, ok := value["path"]; !ok {
		return false
	}
	if _, ok := value["key"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeCommandRunResult(value map[string]any) bool {
	if _, ok := value["executed"]; !ok {
		return false
	}
	if _, ok := value["exit_code"]; !ok {
		return false
	}
	if _, ok := value["started_at"]; !ok {
		return false
	}
	if _, ok := value["finished_at"]; !ok {
		return false
	}
	if _, ok := value["duration_ms"]; !ok {
		return false
	}
	if _, ok := value["policy"]; !ok {
		return false
	}
	return true
}

var gitSubjectPrefixRe = regexp.MustCompile(`^[0-9a-f]{7,40} `)

func normalizeContractString(value string, replacements map[string]string) string {
	keys := make([]string, 0, len(replacements))
	for from := range replacements {
		if from != "" {
			keys = append(keys, from)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, from := range keys {
		value = strings.ReplaceAll(value, from, replacements[from])
	}
	if gitSubjectPrefixRe.MatchString(value) {
		return "$GIT_SHA " + strings.TrimSpace(gitSubjectPrefixRe.ReplaceAllString(value, ""))
	}
	if isRFC3339Like(value) {
		return "$TIMESTAMP"
	}
	return value
}

func normalizeMCPTextJSON(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "text" {
				if text, ok := child.(string); ok {
					var nested any
					if err := json.Unmarshal([]byte(text), &nested); err == nil {
						out["json"] = normalizeContractValue(nested, replacements)
						continue
					}
				}
			}
			out[key] = normalizeMCPTextJSON(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, normalizeMCPTextJSON(child, replacements))
		}
		return out
	default:
		return v
	}
}

func isDynamicTimeKey(key string) bool {
	switch key {
	case "updated_at", "generated_at", "cutoff", "started_at", "finished_at":
		return true
	default:
		return false
	}
}

func isRFC3339Like(value string) bool {
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}
