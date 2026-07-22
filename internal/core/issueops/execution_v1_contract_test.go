package issueops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestExecutionV1AutoFallbackCreatesAndLinksDirectWorkspace(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)

	got, err := PrepareExecutionV1(context.Background(), stateRoot, ExecutionPrepareRequestV1{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor:     executionActorV1("codex", "direct-session"),
		OwnerHost: "codex", OwnerModel: "gpt-5",
	}, ExecutionPrepareDependenciesV1{Direct: gitworktree.New()})
	if err != nil {
		t.Fatalf("prepare auto fallback: %v", err)
	}
	if got.ResolvedMode != "direct" {
		t.Fatalf("auto fallback must resolve to direct, got %#v", got)
	}
	if got.Workspace.Root == "" || got.Workspace.Root == record.Repo {
		t.Fatalf("direct fallback must select an isolated worktree, got %q", got.Workspace.Root)
	}
	if info, err := os.Stat(got.Workspace.Root); err != nil || !info.IsDir() {
		t.Fatalf("direct fallback did not create its worktree %q: info=%v err=%v", got.Workspace.Root, info, err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution == nil || persisted.WorktreePath != got.Workspace.Root {
		t.Fatalf("direct fallback did not link the canonical worktree: record=%q result=%q", persisted.WorktreePath, got.Workspace.Root)
	}
}

func TestExecutionV1DirectAccessFailureReturnsRelaunchWithoutPartialState(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			provisioner := &executionAccessDeniedProvisioner{host: host}
			_, err := PrepareExecutionV1(context.Background(), stateRoot, ExecutionPrepareRequestV1{
				ID: record.ID, Mode: "direct", CWD: record.Repo, Confirm: true,
				Actor: executionActorV1(host, host+"-session"),
			}, ExecutionPrepareDependenciesV1{Direct: provisioner})
			if err == nil || !strings.Contains(err.Error(), provisioner.command()) {
				t.Fatalf("access denial must return the exact %s relaunch prerequisite: %v", host, err)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.Execution != nil || persisted.WorktreePath != "" || provisioner.prepareCalls != 0 {
				t.Fatalf("access denial left partial execution state: record=%#v prepareCalls=%d", persisted.Execution, provisioner.prepareCalls)
			}
		})
	}
}

type executionAccessDeniedProvisioner struct {
	host         string
	prepareCalls int
}

func (p *executionAccessDeniedProvisioner) command() string {
	return fmt.Sprintf("%s relaunch-with-canonical-base", p.host)
}

func (p *executionAccessDeniedProvisioner) ProbeAccess(context.Context, port.ExecutionWorkspaceRequest, string) (port.ExecutionWorkspaceAccessResult, error) {
	return port.ExecutionWorkspaceAccessResult{Code: "canonical_worktree_base_inaccessible", RelaunchCommand: p.command()}, nil
}

func (p *executionAccessDeniedProvisioner) Prepare(context.Context, port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	p.prepareCalls++
	return port.ExecutionWorkspaceReceipt{}, nil
}

func TestExecutionV1FreshSessionReplacesExitedOwnerWithoutCoordinator(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	branch := "69-replace-owner"
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
	record.Execution = &model.ExecutionV1{
		Mode: model.ExecutionModeDirect,
		Workspace: model.WorkspaceV1{
			SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: baseHead,
			Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: model.WriteLeaseV1{
			Generation: 1, Status: model.LeaseStatusActive,
			Holder: &model.NativeActorV1{
				Host: "codex", SessionID: "exited-owner",
				SessionProcess: &model.NativeProcessReceiptV1{PID: 999999, StartedAt: "2026-07-22T00:00:00Z", Executable: "codex"},
			},
			ClaimedAt: "2026-07-22T00:00:00Z",
		},
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	writeIssueOpsFile(t, worktree, "README.md", "staged dirty bytes\n")
	writeIssueOpsFile(t, worktree, "untracked.bin", "\x00dirty\n")
	if code, _, stderr := preflight.GitCmd(worktree, "add", "README.md"); code != 0 {
		t.Fatalf("git add dirty fixture: %s", stderr)
	}
	beforeStatus := preflight.GitOut(worktree, "status", "--porcelain=v1")
	beforeHead := preflight.GitOut(worktree, "rev-parse", "HEAD")
	beforeSnapshot, err := workspaceSnapshotV1(record.Execution.Workspace)
	if err != nil {
		t.Fatal(err)
	}

	fresh := executionActorV1("claude", "fresh-owner")
	preview, err := ReplaceExecutionV1(stateRoot, ExecutionReplaceRequestV1{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: fresh, CWD: repo,
	})
	if err != nil {
		t.Fatalf("replace preview without coordinator: %v", err)
	}
	revoked, err := ReplaceExecutionV1(stateRoot, ExecutionReplaceRequestV1{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner process exited", Actor: fresh, CWD: repo, Confirm: true,
	})
	if err != nil {
		t.Fatalf("replace revoke without coordinator: %v", err)
	}
	if revoked.Execution.Lease.Status != model.LeaseStatusRevoking || revoked.Execution.Lease.Generation != 2 {
		t.Fatalf("revoke did not fence generation 1: %#v", revoked.Execution.Lease)
	}
	finalPreview, err := ReplaceExecutionV1(stateRoot, ExecutionReplaceRequestV1{
		ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2, Actor: fresh, CWD: worktree,
	})
	if err != nil {
		t.Fatalf("replace finalize preview after exited owner: %v", err)
	}
	finalized, err := ReplaceExecutionV1(stateRoot, ExecutionReplaceRequestV1{
		ID: record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: finalPreview.QuiescenceFingerprint, Actor: fresh, CWD: worktree, Confirm: true,
	})
	if err != nil {
		t.Fatalf("replace finalize without coordinator: %v", err)
	}
	claimed, err := claimExecutionV1(stateRoot, ExecutionClaimRequestV1{
		ID: record.ID, Generation: 2, Actor: fresh, CWD: worktree, TokenFile: finalized.ClaimTokenPath,
	})
	if err != nil {
		t.Fatalf("fresh owner claim without coordinator: %v", err)
	}
	if claimed.Execution.Lease.Holder == nil || claimed.Execution.Lease.Holder.SessionID != fresh.SessionID {
		t.Fatalf("fresh session did not become the current holder: %#v", claimed.Execution.Lease)
	}
	if after := preflight.GitOut(worktree, "status", "--porcelain=v1"); after != beforeStatus {
		t.Fatalf("replace changed dirty worktree status\nbefore=%q\nafter=%q", beforeStatus, after)
	}
	if after := preflight.GitOut(worktree, "rev-parse", "HEAD"); after != beforeHead {
		t.Fatalf("replace changed HEAD: before=%q after=%q", beforeHead, after)
	}
	afterSnapshot, err := workspaceSnapshotV1(record.Execution.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	if afterSnapshot != beforeSnapshot {
		t.Fatalf("replace changed worktree/index byte fingerprint: before=%s after=%s", beforeSnapshot, afterSnapshot)
	}
	if data, err := os.ReadFile(filepath.Join(worktree, "untracked.bin")); err != nil || string(data) != "\x00dirty\n" {
		t.Fatalf("replace lost untracked bytes: data=%q err=%v", data, err)
	}
}
