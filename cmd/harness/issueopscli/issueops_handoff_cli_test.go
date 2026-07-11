package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestRunIssueOpsHandoffLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := handoffCLIRecord(t, handoff.StateDispatched)
	common := []string{"--id", record.ID, "--attempt", "1", "--ownership-epoch", "epoch-1", "--context-sha256", strings.Repeat("a", 64)}
	claim := append([]string{"handoff", "claim"}, common...)
	claim = append(claim, "--host", "codex", "--session-id", "session-1", "--agent-id", "worker-1", "--cwd", record.WorktreePath, "--orca-worktree-id", "wt-1", "--json")
	if out := captureStdoutForContract(t, func() error { return runIssueOps(claim) }); !strings.Contains(out, `"state": "claimed"`) {
		t.Fatalf("claim output: %s", out)
	}
	finish := append([]string{"handoff", "finish"}, common...)
	finish = append(finish, "--host", "codex", "--session-id", "session-1", "--agent-id", "worker-1", "--outcome", "completed", "--final-head", "head-1", "--changed-file", "internal/x.go", "--turing-report", ".agent-harness/research/report.md", "--verification", "go test: pass", "--cleanup-receipt", "temp removed", "--task-id", "task-1", "--dispatch-id", "dispatch-1", "--json")
	if out := captureStdoutForContract(t, func() error { return runIssueOps(finish) }); !strings.Contains(out, `"state": "submitted"`) {
		t.Fatalf("finish output: %s", out)
	}
	accept := append([]string{"handoff", "accept"}, common...)
	accept = append(accept, "--final-head", "head-1", "--json")
	if out := captureStdoutForContract(t, func() error { return runIssueOps(accept) }); !strings.Contains(out, `"closed_disposition": "accepted"`) {
		t.Fatalf("accept output: %s", out)
	}
}

func TestRunIssueOpsHandoffRequiresConfirmationForMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := handoffCLIRecord(t, handoff.StateDispatched)
	if err := runIssueOps([]string{"handoff", "recover", "--id", record.ID, "--action", "cancel", "--json"}); err == nil {
		t.Fatal("cancel without confirmation must fail")
	}
	persisted, _ := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if persisted.ExecutionHandoff.State != handoff.StateDispatched {
		t.Fatal("unconfirmed recover mutated state")
	}
}

func TestOrcaHandoffResumeBindRefusedReadOnly(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := handoffCLIRecord(t, handoff.StateDispatched)
	before, _ := json.Marshal(record)
	if err := runIssueOps([]string{"resume", "--repo", record.Repo, "--id", record.ID, "--bind", "--json"}); err == nil || !strings.Contains(err.Error(), "supervised handoff") {
		t.Fatalf("expected normalized bind refusal, got %v", err)
	}
	afterRecord, _ := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("refused resume bind mutated record")
	}
}

func handoffCLIRecord(t *testing.T, state string) core.IssueOpsRecord {
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
		ProtocolVersion: handoff.ProtocolVersion, State: state, Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: strings.Repeat("a", 64),
		CoordinatorRoot: repo, WorkerRoot: worktree,
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{WorktreeID: "wt-1", WorktreePath: worktree, TaskID: "task-1", DispatchID: "dispatch-1"},
	}
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
