package issueops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestMCPIssueOpsHandoffLifecycleParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpHandoffRecord(t)
	common := map[string]any{
		"id": record.ID, "attempt": 1, "ownership_epoch": "epoch-1", "context_sha256": strings.Repeat("a", 64),
	}
	claim := cloneHandoffArgs(common)
	claim["action"], claim["host"], claim["session_id"], claim["agent_id"] = "claim", "codex", "session-1", "worker-1"
	claim["cwd"], claim["orca_worktree_id"] = record.WorktreePath, "wt-1"
	claimed := callMCPToolForIssueOpsTest(t, "issueops_handoff", claim)
	if nestedMap(claimed, "execution_handoff")["state"] != handoff.StateClaimed {
		t.Fatalf("claim parity failed: %#v", claimed)
	}
	finish := cloneHandoffArgs(common)
	finish["action"], finish["host"], finish["session_id"], finish["agent_id"] = "finish", "codex", "session-1", "worker-1"
	finish["outcome"], finish["final_head"] = "completed", "head-1"
	finish["changed_files"] = []string{"internal/x.go"}
	finish["turing_report_path"] = ".agent-harness/research/report.md"
	finish["verification"] = []string{"go test: pass"}
	finish["cleanup_receipts"] = []string{"temp removed"}
	finish["task_id"], finish["dispatch_id"] = "task-1", "dispatch-1"
	submitted := callMCPToolForIssueOpsTest(t, "issueops_handoff", finish)
	if nestedMap(submitted, "execution_handoff")["state"] != handoff.StateSubmitted {
		t.Fatalf("finish parity failed: %#v", submitted)
	}
	accept := cloneHandoffArgs(common)
	accept["action"], accept["final_head"] = "accept", "head-1"
	closed := callMCPToolForIssueOpsTest(t, "issueops_handoff", accept)
	if nestedMap(closed, "execution_handoff")["closed_disposition"] != handoff.DispositionAccepted {
		t.Fatalf("accept parity failed: %#v", closed)
	}
}

func TestMCPIssueOpsHandoffUsesOneActionTool(t *testing.T) {
	for _, forbidden := range []string{"issueops_handoff_start", "issueops_handoff_claim", "issueops_handoff_finish", "issueops_handoff_accept", "issueops_handoff_recover"} {
		if msg := callMCPUnknownTool(t, forbidden); !strings.Contains(msg, "Unknown tool") {
			t.Fatalf("duplicate tool %s was advertised or handled: %s", forbidden, msg)
		}
	}
}

func mcpHandoffRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	repo := makeIssueOpsCLIRepoForTest(t, "handoff")
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "1-handoff")
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/1-handoff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "1-handoff"})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateDispatched, Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64),
		CoordinatorRoot: repo, WorkerRoot: worktree,
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-1", WorktreePath: worktree, TaskID: "task-1", DispatchID: "dispatch-1"},
	}
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func cloneHandoffArgs(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+1)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func nestedMap(value map[string]any, key string) map[string]any {
	result, _ := value[key].(map[string]any)
	return result
}

func callMCPUnknownTool(t *testing.T, name string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": name, "arguments": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	_, rpcErr := mcpcli.HandleToolCall(raw)
	if rpcErr == nil {
		return "handled"
	}
	return rpcErr.Message
}
