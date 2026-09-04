package issueopsapp

import "testing"

func buildMCPResponseContractSnapshot(t *testing.T, replacements map[string]string, workspaceDir, gitRepoDir string) map[string]any {
	t.Helper()
	mcpSnapshot := map[string]any{}
	mcpSnapshot["harness_inspect"] = inspectMCPContractProjection(t, runMCPToolContract(t, replacements, "harness_inspect", map[string]any{
		"repo": workspaceDir,
	}))
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
	mcpSnapshot["state_prune"] = runMCPToolContract(t, replacements, "state_prune", map[string]any{
		"max_age": "1h",
	})
	mcpSnapshot["state_doctor"] = runMCPToolContract(t, replacements, "state_doctor", map[string]any{})
	issueopsExecutionID := seedIssueOpsExecutionContract(t, workspaceDir, "69-mcp-execution-contract")
	replacements[issueopsExecutionID] = "$MCP_EXECUTION_ID"
	mcpSnapshot["issueops_execution_status"] = runMCPToolContract(t, replacements, "issueops_execution", map[string]any{
		"action": "status",
		"id":     issueopsExecutionID,
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
