package harnessapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/harnessapp/responsecontract"
)

func buildMCPResponseContractSnapshot(t *testing.T, replacements map[string]string, workspaceDir, gitRepoDir string) map[string]any {
	t.Helper()
	mcpSnapshot := map[string]any{}
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
	mcpSnapshot["issueops_start"] = responsecontract.NormalizeMCPTextJSON(responsecontract.NormalizeContractValue(issueopsMCPStartRaw, replacements), replacements)
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
	mcpSnapshot["issueops_record_intent"] = runMCPToolContract(t, replacements, "issueops_record_intent", map[string]any{
		"id":                 issueopsMCPID,
		"raw_request":        "Refactor IssueOps intent flow",
		"interpreted_intent": "Persist main-agent judgment before planning",
		"success_criteria":   []string{"intent is recorded"},
		"constraints":        []string{"keep contract deterministic"},
		"ambiguities":        []string{"none"},
		"non_goals":          []string{"do not continue from hook recommendation alone"},
	})
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
	if err := os.MkdirAll(filepath.Join(mcpWorktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mcpWorktree, ".git", "HEAD"), []byte("ref: refs/heads/2-mcp-contract\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	replacements[mcpWorktree] = "$MCP_ISSUEOPS_WORKTREE"
	addEvalSymlinkReplacement(t, replacements, mcpWorktree, "$MCP_ISSUEOPS_WORKTREE")
	mcpSnapshot["issueops_link_worktree"] = runMCPToolContract(t, replacements, "issueops_link_worktree", map[string]any{
		"id":            issueopsMCPID,
		"worktree_path": mcpWorktree,
	})
	mcpSnapshot["issueops_review_design"] = runMCPToolContract(t, replacements, "issueops_review_design", map[string]any{
		"id":              issueopsMCPID,
		"problem_summary": "IssueOps needs explicit design review",
		"proposed_design": "Gate implementation on approved design",
		"refactor_plan":   "Keep changes local to IssueOps state and adapters",
		"risks":           []string{"golden contract drift"},
		"alternatives":    []string{"docs-only guidance"},
		"verification":    []string{"design review checked contract drift risk", "go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden"},
		"approved":        true,
	})
	writeContractFile(t, mcpWorktree, "docs/superpowers/plans/mcp-contract.md", "plan\n")
	mcpSnapshot["issueops_link_plan"] = runMCPToolContract(t, replacements, "issueops_link_plan", map[string]any{
		"id":        issueopsMCPID,
		"plan_path": filepath.Join(mcpWorktree, "docs", "superpowers", "plans", "mcp-contract.md"),
	})
	mcpSnapshot["issueops_remote_create_child"] = runMCPToolContract(t, replacements, "issueops_remote_create_child", map[string]any{
		"id":        issueopsMCPID,
		"title":     "MCP contract child",
		"body":      "contract child body",
		"labels":    []string{"contract"},
		"assignees": []string{"octocat"},
	})
	mcpSnapshot["issueops_link_child"] = runMCPToolContract(t, replacements, "issueops_link_child", map[string]any{
		"id":        issueopsMCPID,
		"child_url": "https://github.com/example/repo/issues/3",
		"title":     "MCP contract child",
	})
	mcpSnapshot["issueops_add_feedback"] = runMCPToolContract(t, replacements, "issueops_add_feedback", map[string]any{
		"id":             issueopsMCPID,
		"source":         "review",
		"body":           "tighten MCP contract",
		"classification": "contract_change",
	})
	mcpSnapshot["issueops_mark_issue_updated"] = runMCPToolContract(t, replacements, "issueops_mark_issue_updated", map[string]any{
		"id": issueopsMCPID,
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
	return mcpSnapshot
}
