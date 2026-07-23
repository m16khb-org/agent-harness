package harnessapp

import (
	"encoding/json"
	"testing"

	"agent-harness/cmd/harness/harnessapp/responsecontract"
)

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
