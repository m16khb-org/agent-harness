package issueopscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

type cliWorkerDoneFake struct{ calls int }

func (f *cliWorkerDoneFake) SendWorkerDone(context.Context, port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error) {
	f.calls++
	return port.OrcaWorkerDoneResult{MessageID: "msg-cli", Sequence: 11}, nil
}

func TestRunIssueOpsHandoffLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	projector := &cliWorkerDoneFake{}
	previousProjector := issueOpsWorkerDoneProjectionClient
	issueOpsWorkerDoneProjectionClient = func() core.IssueOpsWorkerDoneProjectionClient { return projector }
	t.Cleanup(func() { issueOpsWorkerDoneProjectionClient = previousProjector })
	record := handoffCLIRecord(t, handoff.StateDispatched)
	common := []string{"--id", record.ID, "--attempt", "1", "--ownership-epoch", "epoch-1", "--context-sha256", strings.Repeat("a", 64)}
	claim := append([]string{"handoff", "claim"}, common...)
	claim = append(claim, "--host", "codex", "--session-id", "session-1", "--agent-id", "worker-1", "--cwd", record.WorktreePath, "--orca-worktree-id", "wt-1", "--json")
	if out := captureStdoutForContract(t, func() error { return runIssueOps(claim) }); !strings.Contains(out, `"state": "claimed"`) {
		t.Fatalf("claim output: %s", out)
	}
	finalHead := commitCLIHandoffResult(t, record.WorktreePath)
	finish := append([]string{"handoff", "finish"}, common...)
	finish = append(finish, "--host", "codex", "--session-id", "session-1", "--agent-id", "worker-1", "--outcome", "completed", "--final-head", finalHead, "--changed-file", "internal/x.go", "--changed-file", ".agent-harness/research/report.md", "--turing-report", ".agent-harness/research/report.md", "--verification", "go test: pass", "--cleanup-receipt", "temp removed", "--task-id", "task-1", "--dispatch-id", "dispatch-1", "--json")
	if out := captureStdoutForContract(t, func() error { return runIssueOps(finish) }); !strings.Contains(out, `"state": "submitted"`) || !strings.Contains(out, `"worker_done_projection"`) || projector.calls != 1 {
		t.Fatalf("finish output: %s", out)
	}
	accept := append([]string{"handoff", "accept"}, common...)
	accept = append(accept, "--final-head", finalHead, "--json")
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

func TestIssueOpsHandoffUsageExposesCodexHookTrustBypassAttestation(t *testing.T) {
	for _, want := range []string{"--coordinator-recipient", "--allow-codex-hook-trust-bypass", "--expected-context-sha256"} {
		if !strings.Contains(issueOpsHandoffUsage, want) {
			t.Fatalf("handoff start usage must expose %s", want)
		}
	}
}

func TestIssueOpsHandoffUsageExposesForceAbandon(t *testing.T) {
	for _, fragment := range []string{"reconcile|abandon|cancel|finalize-cancel|retry", "--confirm", "--force", "--reason"} {
		if !strings.Contains(issueOpsHandoffUsage, fragment) {
			t.Fatalf("handoff recover usage missing %q", fragment)
		}
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
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "1-handoff"}, {"config", "user.name", "CLI Test"}, {"config", "user.email", "cli@example.test"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeCLIFile(t, worktree, "plans/handoff.md", "# plan\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: prepare handoff"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "1-handoff"})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.PlanPath = filepath.Join(worktree, "plans", "handoff.md")
	record.Phase = core.IssueOpsPhaseImplement
	baseHead := strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: state, Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: baseHead, ContextSHA256: strings.Repeat("a", 64),
		ContextVersion: handoff.ContextVersion, ContextOptions: &issueopsmodel.IssueOpsExecutionHandoffContextOptions{}, Driver: "orca", Agent: "codex", DeliveryMode: "inject",
		CoordinatorRoot: repo, CoordinatorMailboxHandle: "term_coordinator", WorkerRoot: worktree,
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/1-handoff", WorktreeID: "wt-1", WorktreeInstanceID: "instance-1", WorktreePath: worktree,
			WorkerPTYID: "pty-1", WorkerTerminalHandle: "term_worker", WorkerMailboxHandle: "term_worker", TaskID: "task-1", DispatchID: "dispatch-1",
		},
	}
	record.ExecutionHandoff.ContextSourceSHA256, err = handoff.ContextSourceSHA256(record)
	if err != nil {
		t.Fatal(err)
	}
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func commitCLIHandoffResult(t *testing.T, worktree string) string {
	t.Helper()
	writeCLIFile(t, worktree, "internal/x.go", "package internal\n")
	writeCLIFile(t, worktree, ".agent-harness/research/report.md", "# evidence\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: finish handoff"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	return strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
}

func writeCLIFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
