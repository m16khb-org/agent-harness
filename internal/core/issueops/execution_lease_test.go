package issueops

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

type claimableExecutionFixture struct {
	record    IssueOpsRecord
	worktree  string
	tokenPath string
}

func TestExecutionConcurrentClaimsChooseOneLifecyclePerSession(t *testing.T) {
	stateRoot := t.TempDir()
	first := newClaimableExecutionFixture(t, stateRoot, "69-first")
	second := newClaimableExecutionFixture(t, stateRoot, "70-second")
	actor := executionActor("codex", "one-native-session")
	start := make(chan struct{})

	requests := []ExecutionClaimRequest{
		{ID: first.record.ID, Generation: 1, Actor: actor, CWD: first.worktree, TokenFile: first.tokenPath},
		{ID: second.record.ID, Generation: 1, Actor: actor, CWD: second.worktree, TokenFile: second.tokenPath},
	}
	errs := make(chan error, len(requests))
	var ready sync.WaitGroup
	ready.Add(len(requests))
	for _, request := range requests {
		request := request
		go func() {
			ready.Done()
			<-start
			_, err := claimViaVertical(stateRoot, request)
			errs <- err
		}()
	}
	ready.Wait()
	close(start)

	successes := 0
	for range requests {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("one native session claimed %d lifecycles; want exactly 1", successes)
	}
	active := 0
	for _, fixture := range []claimableExecutionFixture{first, second} {
		record, err := ReadIssueOps(stateRoot, fixture.record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if record.Execution.Lease.Status == model.LeaseStatusActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active lifecycle count=%d want=1", active)
	}
}

func TestExecutionCompetingClaimsChooseOneHolder(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-competing")
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, actor := range []model.NativeActor{
		executionActor("codex", "session-a"),
		executionActor("claude", "session-b"),
	} {
		actor := actor
		go func() {
			<-start
			_, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
				ID: fixture.record.ID, Generation: 1, Actor: actor,
				CWD: fixture.worktree, TokenFile: fixture.tokenPath,
			})
			errs <- err
		}()
	}
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("competing claim successes=%d want=1", successes)
	}
	record, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Lease.Status != model.LeaseStatusActive || record.Execution.Lease.Holder == nil {
		t.Fatalf("claim winner was not persisted: %#v", record.Execution.Lease)
	}
}

func TestExecutionClaimRetryIsIdempotentAfterTokenConsumption(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-idempotent")
	actor := executionActor("codex", "idempotent-session")
	request := ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}
	if _, err := claimViaVertical(stateRoot, request); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fixture.tokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("claim token remains after successful claim: %v", err)
	}
	if _, err := claimViaVertical(stateRoot, request); err != nil {
		t.Fatalf("same holder retry must be idempotent after token removal: %v", err)
	}
}

func TestExecutionClaimRejectsInsecureOrSymlinkToken(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{name: "world-readable", mutate: func(t *testing.T, path string) {
			if err := os.Chmod(path, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink", mutate: func(t *testing.T, path string) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			target := path + ".target"
			if err := os.WriteFile(target, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			fixture := newClaimableExecutionFixture(t, stateRoot, "69-token-"+testCase.name)
			testCase.mutate(t, fixture.tokenPath)
			_, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
				ID: fixture.record.ID, Generation: 1,
				Actor: executionActor("codex", "token-session"),
				CWD:   fixture.worktree, TokenFile: fixture.tokenPath,
			})
			if err == nil {
				t.Fatal("insecure claim token was accepted")
			}
		})
	}
}

func TestExecutionReseedInvalidatesPriorGenerationToken(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-reseed")
	requester := executionActor("claude", "reseed-requester")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	reseeded, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "lost unclaimed terminal",
		Actor: requester, CWD: fixture.record.Repo, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reseeded.Execution.Lease.Generation != 2 || reseeded.ClaimTokenPath == fixture.tokenPath {
		t.Fatalf("reseed did not rotate generation/token path: %#v", reseeded)
	}
	actor := executionActor("claude", "reseed-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 2, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err == nil {
		t.Fatal("prior-generation token path was accepted")
	}
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 2, Actor: actor,
		CWD: fixture.worktree, TokenFile: reseeded.ClaimTokenPath,
	}); err != nil {
		t.Fatalf("current-generation token was rejected: %v", err)
	}
}

func TestExecutionRevokeRejectsStaleInventoryAndFencesOldGeneration(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-revoke")
	old := executionActor("codex", "old-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: old,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	requester := executionActor("claude", "revoke-requester")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, fixture.worktree, "after-preview.txt", "late dirty bytes\n")
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "stale inventory", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	}); err == nil {
		t.Fatal("revoke accepted a stale worktree inventory fingerprint")
	}
	preview, err = ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner exited", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReleaseExecution(stateRoot, ExecutionReleaseRequest{
		ID: fixture.record.ID, Generation: 1, Actor: old, CWD: fixture.worktree,
	}); err == nil || !strings.Contains(err.Error(), "generation 1") {
		t.Fatalf("old generation release was not fenced: %v", err)
	}
}

func TestExecutionFinalizeRejectsLiveOwnerProcess(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-live")
	old := executionActor("codex", "live-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: old,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	requester := executionActor("claude", "live-revoke-requester")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "operator-confirmed revoke", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2, Actor: requester, CWD: fixture.worktree,
	}); err == nil || !strings.Contains(err.Error(), "still live") {
		t.Fatalf("live owner process did not block finalize: %v", err)
	}
}

func TestExecutionClaimRejectsForgedLiveProcessIdentity(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-forged-process")
	actor := executionActor("codex", "forged-process-session")
	actor.SessionProcess.StartedAt = "1970-01-01T00:00:00Z"
	_, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	})
	if err == nil || !strings.Contains(err.Error(), "process") {
		t.Fatalf("forged live process identity was not rejected: %v", err)
	}
}

func TestExecutionReplaceRejectsForgedRequesterAndForeignCWD(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-forged-replacer")
	actor := executionActor("claude", "forged-replacer")
	actor.SessionProcess.StartedAt = "1970-01-01T00:00:00Z"
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: actor, CWD: fixture.record.Repo,
	}); err == nil || !strings.Contains(err.Error(), "process") {
		t.Fatalf("forged replacement actor was accepted: %v", err)
	}
	actor = executionActor("claude", "foreign-cwd-replacer")
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: actor, CWD: t.TempDir(),
	}); err == nil || !strings.Contains(err.Error(), "source_root or the canonical worktree") {
		t.Fatalf("foreign replacement cwd was accepted: %v", err)
	}
}

func TestExecutionFinalizeRejectsPIDReuseIdentityMismatch(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-pid-reuse")
	actor := executionActor("codex", "pid-reuse-session")
	if _, err := claimViaVertical(stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 1, Actor: actor,
		CWD: fixture.worktree, TokenFile: fixture.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.Holder.SessionProcess.StartedAt = "1970-01-01T00:00:00Z"
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	requester := executionActor("claude", "pid-reuse-requester")
	preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: fixture.record.Repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "PID identity audit", Confirm: true,
		Actor: requester, CWD: fixture.record.Repo,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2, Actor: requester, CWD: fixture.worktree,
	}); err == nil || !strings.Contains(err.Error(), "process identity") {
		t.Fatalf("PID reuse or forged receipt was not reported distinctly: %v", err)
	}
}

func TestExecutionFinalizeRejectsWorkspaceCWDOrWritableFileProcess(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		start func(t *testing.T, fixture claimableExecutionFixture) *exec.Cmd
	}{
		{name: "cwd", start: func(t *testing.T, fixture claimableExecutionFixture) *exec.Cmd {
			cmd := exec.Command("sleep", "30")
			cmd.Dir = fixture.worktree
			startExecutionProcessFixture(t, cmd)
			return cmd
		}},
		{name: "writable-file", start: func(t *testing.T, fixture claimableExecutionFixture) *exec.Cmd {
			path := filepath.Join(fixture.worktree, "open-writer.log")
			cmd := exec.Command("sh", "-c", "exec 3>>\"$1\"; exec sleep 30", "sh", path)
			cmd.Dir = fixture.record.Repo
			startExecutionProcessFixture(t, cmd)
			return cmd
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			fixture := newClaimableExecutionFixture(t, stateRoot, "69-process-"+testCase.name)
			activateExecutionFixtureWithDeadHolder(t, stateRoot, &fixture)
			cmd := testCase.start(t, fixture)
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
			})
			requesterCmd := exec.Command("sleep", "30")
			startExecutionProcessFixture(t, requesterCmd)
			t.Cleanup(func() {
				_ = requesterCmd.Process.Kill()
				_ = requesterCmd.Wait()
			})
			requesterReceipt, err := ObserveNativeProcessReceipt(requesterCmd.Process.Pid)
			if err != nil {
				t.Fatal(err)
			}
			requester := model.NativeActor{
				Host: "claude", SessionID: "process-audit-requester-" + testCase.name,
				SessionProcess: &requesterReceipt, ProcessAncestry: []model.NativeProcessReceipt{requesterReceipt},
			}
			preview, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
				ID: fixture.record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: fixture.record.Repo,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
				ID: fixture.record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
				InventoryFingerprint: preview.InventoryFingerprint, Reason: "detached writer audit", Confirm: true,
				Actor: requester, CWD: fixture.record.Repo,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := ReplaceExecution(stateRoot, ExecutionReplaceRequest{
				ID: fixture.record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2, Actor: requester, CWD: fixture.worktree,
			}); err == nil || !strings.Contains(err.Error(), "workspace process") {
				t.Fatalf("workspace %s process did not block finalize: %v", testCase.name, err)
			}
		})
	}
}

func TestExecutionOrcaFinalizeRequiresTerminalAndTaskQuiescence(t *testing.T) {
	stateRoot := t.TempDir()
	fixture := newClaimableExecutionFixture(t, stateRoot, "69-orca-quiescence")
	activateExecutionFixtureWithDeadHolder(t, stateRoot, &fixture)
	record, err := ReadIssueOps(stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Mode = model.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &model.OrcaBinding{
		RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1", OwnerHost: "claude", OwnerModel: "model-1",
		TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	inspector := &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
		TerminalLive: true, TaskLive: true, TerminalID: "pty-1", TaskStatus: "running", DispatchStatus: "running",
	}}
	deps := ExecutionReplaceDependencies{OrcaOwner: inspector}
	requester := executionActor("codex", "orca-replacement-requester")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "Orca owner exited", Actor: requester, CWD: record.Repo, Confirm: true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2, Actor: requester, CWD: fixture.worktree,
	}, deps); err == nil || !strings.Contains(err.Error(), "Orca owner is not quiescent") {
		t.Fatalf("live Orca terminal/task did not block finalization: %v", err)
	}
	if inspector.last.WorktreeID != "worktree-1" || inspector.last.TaskID != "task-1" || inspector.last.TerminalPTYID != "pty-1" {
		t.Fatalf("Orca inventory did not use stable binding locators: %#v", inspector.last)
	}
}

type executionOrcaOwnerInspectorFake struct {
	inventory port.ExecutionOrcaOwnerInventory
	last      port.ExecutionOrcaOwnerInventoryRequest
}

func (f *executionOrcaOwnerInspectorFake) InspectOwner(_ context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	f.last = req
	return f.inventory, nil
}

func newClaimableExecutionFixture(t *testing.T, stateRoot, branch string) claimableExecutionFixture {
	t.Helper()
	repo := initIssueOpsRepo(t)
	worktree := issueOpsWorktreePathForTest(repo, branch)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", "-b", branch, worktree, "main"); code != 0 {
		t.Fatalf("git worktree add: %s", stderr)
	}
	baseHead := strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.BranchPrepare = &IssueOpsBranchPrepare{
		Provider: "github", IssueURL: "https://github.com/example/agent-harness/issues/69",
		Branch: branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	record.Execution = &model.Execution{
		Mode: model.ExecutionModeDirect,
		Workspace: model.Workspace{
			SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: baseHead,
			Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusClaimable},
	}
	token, tokenPath, err := createClaimToken(record)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.ClaimTokenSHA256 = tokenSHA256(token)
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return claimableExecutionFixture{record: record, worktree: worktree, tokenPath: tokenPath}
}

func executionActor(host, sessionID string) model.NativeActor {
	receipt, err := ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		panic(err)
	}
	return model.NativeActor{
		Host: host, SessionID: sessionID,
		SessionProcess:  &receipt,
		ProcessAncestry: []model.NativeProcessReceipt{receipt},
	}
}

func activateExecutionFixtureWithDeadHolder(t *testing.T, stateRoot string, fixture *claimableExecutionFixture) {
	t.Helper()
	_ = os.Remove(fixture.tokenPath)
	fixture.record.Execution.Lease = model.WriteLease{
		Generation: 1, Status: model.LeaseStatusActive, ClaimedAt: "2026-07-22T00:00:00Z",
		Holder: &model.NativeActor{
			Host: "codex", SessionID: "dead-session",
			SessionProcess: &model.NativeProcessReceipt{
				PID: 999999, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex",
			},
		},
	}
	if _, err := writeIssueOps(stateRoot, fixture.record); err != nil {
		t.Fatal(err)
	}
}

func startExecutionProcessFixture(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveNativeProcessReceipt(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("observe process fixture: %v", err)
	}
}
