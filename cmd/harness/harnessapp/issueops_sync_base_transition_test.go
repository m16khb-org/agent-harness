package harnessapp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
)

func TestReleasedSyncBaseReachableThroughProductionClaimCompleteTransitions(t *testing.T) {
	stateRoot, record, tokenPath := seedSyncBaseTransition(t)
	actor := claimWiringActor(t)

	claimed, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionClaim, ID: record.ID, Generation: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root, TokenFile: tokenPath,
	}, issueops.ExecutionActionDependencies{Claim: issueOpsClaimHandler})
	if err != nil {
		t.Fatalf("production claim: %v", err)
	}
	if result := claimed.(issueops.ExecutionResult); result.Execution.Lease.Status != issueopscontract.LeaseStatusActive {
		t.Fatalf("claim result=%+v", result)
	}

	reportPath := filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "turing", "sync-base-transition.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	finalHead := strings.TrimSpace(claimWiringGit(t, record.Execution.Workspace.Root, "rev-parse", "HEAD"))
	completed, err := issueops.ExecuteExecution(context.Background(), stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionComplete, ID: record.ID, Generation: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root, FinalHead: finalHead,
		TuringReportPath: reportPath, Verification: []string{"go test ./... -count=1"},
		RemoteArtifactURL: record.RemoteArtifact.URL, Confirm: true,
	}, issueops.ExecutionActionDependencies{Complete: issueOpsCompleteHandler})
	if err != nil {
		t.Fatalf("production complete: %v", err)
	}
	completedResult := completed.(issueops.ExecutionResult)
	if completedResult.Execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		completedResult.Execution.Lease.Holder != nil || completedResult.Execution.Completion == nil ||
		completedResult.Execution.Completion.Generation != 1 {
		t.Fatalf("complete did not create released current authority: %+v", completedResult.Execution)
	}

	before, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	completionBefore := *before.Execution.Completion
	historyBefore := append([]issueopscontract.ExecutionCompletionHistory(nil), before.Execution.CompletionHistory...)
	phaseBefore := before.Phase
	git := newTransitionSyncBaseGit(t, record.Branch, finalHead)

	preview, err := issueops.SyncExecutionBase(context.Background(), stateRoot, issueops.ExecutionSyncBaseRequest{
		ID: record.ID, Mode: issueops.ExecutionSyncBasePreview, CompletionGeneration: 1,
		CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionSyncBaseDeps{Git: git.run})
	if err != nil || len(preview.Fingerprint) != 64 || !preview.MergeNeeded {
		t.Fatalf("released preview result=%+v err=%v", preview, err)
	}

	apply, err := issueops.SyncExecutionBase(context.Background(), stateRoot, issueops.ExecutionSyncBaseRequest{
		ID: record.ID, Mode: issueops.ExecutionSyncBaseApply, CompletionGeneration: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root, Confirm: true, Fingerprint: preview.Fingerprint,
	}, issueops.ExecutionSyncBaseDeps{Git: git.run})
	if err != nil || !apply.MergeInProgress || apply.Pushed || apply.NextCommand == "" || apply.AbortCommand == "" {
		t.Fatalf("released conflict apply result=%+v err=%v", apply, err)
	}

	finalized, err := issueops.SyncExecutionBase(context.Background(), stateRoot, issueops.ExecutionSyncBaseRequest{
		ID: record.ID, Mode: issueops.ExecutionSyncBaseFinalize, CompletionGeneration: 1,
		Actor: actor, CWD: record.Execution.Workspace.Root,
	}, issueops.ExecutionSyncBaseDeps{Git: git.run})
	if err != nil || !finalized.Merged || !finalized.Pushed || finalized.MergeCommit != git.mergeOID {
		t.Fatalf("released finalize result=%+v err=%v", finalized, err)
	}

	after, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Execution.Completion == nil || !reflect.DeepEqual(*after.Execution.Completion, completionBefore) ||
		!reflect.DeepEqual(after.Execution.CompletionHistory, historyBefore) || after.Phase != phaseBefore ||
		after.Execution.Lease.Status != issueopscontract.LeaseStatusReleased || after.Execution.Lease.Holder != nil ||
		len(after.Execution.SyncBaseEvents) != 1 {
		t.Fatalf("sync-base changed immutable completion authority: before=%+v after=%+v", before.Execution, after.Execution)
	}
}

func seedSyncBaseTransition(t *testing.T) (string, issueopscontract.IssueOpsRecord, string) {
	t.Helper()
	stateRoot := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	worktree := filepath.Join(t.TempDir(), "worktree")
	const branch = "318-production-transition"
	claimWiringGit(t, "", "init", "-q", "-b", "main", source)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# transition fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimWiringGit(t, source, "add", "README.md")
	claimWiringGit(t, source, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: transition fixture")
	claimWiringGit(t, source, "worktree", "add", "-q", "-b", branch, worktree, "main")
	baseHead := strings.TrimSpace(claimWiringGit(t, worktree, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: source, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhasePR
	record.IssueURL = "https://github.com/example/agent-harness/issues/318"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/agent-harness/pull/318",
		Labels: []string{"bug"}, Assignees: []string{"m16khb"}, TargetBranch: "main", VerifiedAt: "2026-08-04T00:00:00Z",
	}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: source, Root: worktree, Branch: branch, BaseHead: baseHead,
			Driver: "git", LinkedAt: "2026-08-04T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable},
	}
	token := "sync-base-transition-token"
	record.Execution.Lease.ClaimTokenSHA256 = claimWiringSHA256(token)
	key := claimWiringSHA256(record.ID)[:16]
	tokenPath := filepath.Join(worktree, ".agent-harness", "state", "issueops-v1", key, "lease-1.token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, tokenPath
}

type transitionSyncBaseGit struct {
	gitDir          string
	branch          string
	remoteOID       string
	baseOID         string
	headOID         string
	mergeOID        string
	mergeInProgress bool
}

func newTransitionSyncBaseGit(t *testing.T, branch, headOID string) *transitionSyncBaseGit {
	t.Helper()
	return &transitionSyncBaseGit{
		gitDir: t.TempDir(), branch: branch, remoteOID: headOID, headOID: headOID,
		baseOID: strings.Repeat("b", 40), mergeOID: strings.Repeat("c", 40),
	}
}

func (g *transitionSyncBaseGit) run(_ context.Context, _ string, args ...string) (int, string) {
	switch args[0] {
	case "branch":
		return 0, g.branch
	case "ls-remote":
		return 0, g.remoteOID + "\trefs/heads/" + g.branch
	case "rev-parse":
		return g.revParse(args[1:])
	case "status", "fetch", "diff", "push":
		return 0, ""
	case "merge-base":
		if args[len(args)-2] == g.remoteOID {
			return 0, ""
		}
		return 1, ""
	case "merge-tree":
		return 1, "treeoid\x00internal/conflict.go\x00\x00CONFLICT (content)\n"
	case "merge":
		g.mergeInProgress = true
		return 1, "CONFLICT (content): Merge conflict in internal/conflict.go"
	case "ls-files":
		return 0, ""
	case "commit":
		g.mergeInProgress = false
		g.headOID = g.mergeOID
		return 0, ""
	default:
		return 0, ""
	}
}

func (g *transitionSyncBaseGit) revParse(args []string) (int, string) {
	target := args[len(args)-1]
	switch target {
	case "MERGE_HEAD":
		if g.mergeInProgress {
			return 0, g.baseOID
		}
		return 1, ""
	case "CHERRY_PICK_HEAD", "REBASE_HEAD":
		return 1, ""
	case "rebase-merge", "rebase-apply", "MERGE_MSG":
		return 0, filepath.Join(g.gitDir, target)
	case "FETCH_HEAD":
		return 0, g.baseOID
	case "HEAD":
		return 0, g.headOID
	default:
		return 1, ""
	}
}
