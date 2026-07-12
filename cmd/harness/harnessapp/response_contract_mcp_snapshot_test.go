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
	mcpSnapshot["docs_index"] = docsIndexMCPContractProjection(t, runMCPToolContract(t, replacements, "docs_index", map[string]any{}))
	mcpSnapshot["daemon_status"] = runMCPToolContract(t, replacements, "daemon_status", map[string]any{})
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
	workpoolMCPCreateRaw := runMCPToolContractRaw(t, "workpool_create", map[string]any{
		"repo": workspaceDir, "name": "contract mcp pool", "size": 2, "lease_ttl": "1h",
	})
	var workpoolMCPCreateText map[string]any
	if text, ok := workpoolMCPCreateRaw["content"].([]any)[0].(map[string]any)["text"].(string); !ok {
		t.Fatalf("MCP workpool create missing text: %#v", workpoolMCPCreateRaw)
	} else if err := json.Unmarshal([]byte(text), &workpoolMCPCreateText); err != nil {
		t.Fatalf("unmarshal MCP workpool create text: %v\n%s", err, text)
	}
	workpoolMCPID, ok := workpoolMCPCreateText["id"].(string)
	if !ok || workpoolMCPID == "" {
		t.Fatalf("MCP workpool create missing id: %#v", workpoolMCPCreateText)
	}
	replacements[workpoolMCPID] = "$MCP_WORKPOOL_ID"
	mcpSnapshot["workpool_create"] = responsecontract.NormalizeMCPTextJSON(responsecontract.NormalizeContractValue(workpoolMCPCreateRaw, replacements), replacements)
	workpoolMCPTaskRaw := runMCPToolContractRaw(t, "workpool_add_task", map[string]any{
		"pool": workpoolMCPID, "title": "mcp contract task", "instructions": "check response contract", "scope": []string{"contract fixture"}, "acceptance": []string{"JSON is stable"},
	})
	var workpoolMCPTaskText map[string]any
	if text, ok := workpoolMCPTaskRaw["content"].([]any)[0].(map[string]any)["text"].(string); !ok {
		t.Fatalf("MCP workpool add-task missing text: %#v", workpoolMCPTaskRaw)
	} else if err := json.Unmarshal([]byte(text), &workpoolMCPTaskText); err != nil {
		t.Fatalf("unmarshal MCP workpool add-task text: %v\n%s", err, text)
	}
	if workpoolMCPTaskID, ok := workpoolMCPTaskText["id"].(string); ok && workpoolMCPTaskID != "" {
		replacements[workpoolMCPTaskID] = "$MCP_WORKTASK_ID"
	}
	mcpSnapshot["workpool_add_task"] = responsecontract.NormalizeMCPTextJSON(responsecontract.NormalizeContractValue(workpoolMCPTaskRaw, replacements), replacements)
	mcpSnapshot["workpool_status"] = runMCPToolContract(t, replacements, "workpool_status", map[string]any{"pool": workpoolMCPID})
	mcpSnapshot["workpool_close"] = runMCPToolContract(t, replacements, "workpool_close", map[string]any{
		"pool": workpoolMCPID, "force": true, "reason": "contract fixture closes pending task",
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
	mcpSnapshot["issueops_plan_prep_record"] = runMCPToolContract(t, replacements, "issueops_plan_prep_record", map[string]any{
		"id":                    issueopsMCPID,
		"decisions_evidence":    []string{".agent-harness/ADR.md"},
		"related_score_ref":     []string{"remote score: selected=#1(0.81), threshold=0.70"},
		"web_research_evidence": []string{".agent-harness/research/contract.md"},
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
	// Phase-ledger tools. The cycle is already at plan (link_issue auto-advances
	// once plan readiness is met), so the sequence exercises real transitions:
	// status (derived ledger at plan) -> regress_for_replan (Brooks stop, plan ->
	// grill, stale-marks the ledger) -> record_domain_review (re-satisfies the
	// grill gate) -> set_phase (grill -> plan, a real forward transition that
	// stamps the ledger) -> record_ai_slop_clean_evidence -> resolve_feedback.
	mcpSnapshot["issueops_status"] = runMCPToolContract(t, replacements, "issueops_status", map[string]any{
		"id": issueopsMCPID,
	})
	mcpSnapshot["issueops_record_devils_advocate_review"] = runMCPToolContract(t, replacements, "issueops_record_devils_advocate_review", map[string]any{
		"id":       issueopsMCPID,
		"verdict":  "stop",
		"findings": []string{"second-system effect: three cache backends for one need"},
	})
	mcpSnapshot["issueops_remote_reflect_devils_advocate"] = runMCPToolContract(t, replacements, "issueops_remote_reflect_devils_advocate", map[string]any{
		"id":      issueopsMCPID,
		"confirm": true,
	})
	mcpSnapshot["issueops_regress_for_replan"] = runMCPToolContract(t, replacements, "issueops_regress_for_replan", map[string]any{
		"id":     issueopsMCPID,
		"reason": "brooks stop: scope too broad for one cycle",
	})
	mcpSnapshot["issueops_record_domain_review"] = runMCPToolContract(t, replacements, "issueops_record_domain_review", map[string]any{
		"id":          issueopsMCPID,
		"model_fit":   "fits the IssueOps phase-ledger domain model",
		"terminology": []string{"phase ledger"},
		"risks":       []string{"ledger drift between CLI and MCP"},
	})
	mcpSnapshot["issueops_set_phase"] = runMCPToolContract(t, replacements, "issueops_set_phase", map[string]any{
		"id": issueopsMCPID,
		"to": "plan",
	})
	mcpSnapshot["issueops_record_ai_slop_clean_evidence"] = runMCPToolContract(t, replacements, "issueops_record_ai_slop_clean_evidence", map[string]any{
		"id":           issueopsMCPID,
		"categories":   []string{"naming"},
		"verification": []string{"go test ./cmd/harness/harnessapp -run Golden"},
	})
	mcpSnapshot["issueops_resolve_feedback"] = runMCPToolContract(t, replacements, "issueops_resolve_feedback", map[string]any{
		"id":         issueopsMCPID,
		"index":      0,
		"resolution": "valid-defect",
	})
	// Error-path snapshot: a tool-level FAILURE (missing model_fit/terminology) must
	// pin the #8 normalized error contract — {ok:false,error:...} content marked as
	// an error result — rather than a -32602 "Invalid params" JSON-RPC error.
	mcpSnapshot["issueops_record_domain_review_error"] = runMCPToolContract(t, replacements, "issueops_record_domain_review", map[string]any{
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
