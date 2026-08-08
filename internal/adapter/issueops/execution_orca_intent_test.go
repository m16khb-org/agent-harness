package issueops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func TestOrcaIntentWorktreeReceiptPersistsPlanBeforeNextIntent(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	const plan = "# Intent owner plan\n"
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
		t.Fatal(err)
	}
	worktree := issueOpsWorktreePathForTest(record.Repo, record.Branch)
	workspace := port.ExecutionWorkspaceRequest{
		LifecycleID: record.ID, SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
		BaseBranch: record.BranchPrepare.BaseBranch, BaseHead: record.BranchPrepare.BaseSHA, Confirm: true,
	}
	probe := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		Provider: "github", Issue: 16, Marker: "readiness-marker",
	}
	issueBody := "## Acceptance\n- AC-01 persist plan\n\n## Verification\n```bash\ngo test ./... -count=1\n```\n"
	snapshot := executionOwnerSnapshot{issue: executionOwnerIssue{
		URL: record.IssueURL, Body: issueBody, BodySHA256: digestExecutionOwnerBytes([]byte(issueBody)),
	}}
	prepared, intent, err := beginOrcaExecutionIntent(
		stateRoot, record, workspace, probe,
		ExecutionPrepareRequest{ID: record.ID, Mode: "orca", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh"},
		snapshot, func() time.Time { return time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	receipt := port.ExecutionOrcaIntentReceipt{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
			BaseHead: record.BranchPrepare.BaseSHA, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
	}}
	readIssue := func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: issueBody}, nil
	}

	advanced, next, err := advanceOrcaIntentReceipt(context.Background(), stateRoot, prepared, intent, receipt, readIssue, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(worktree, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	if advanced.PlanPath != wantPath || next.Stage != "terminal_create" {
		t.Fatalf("advanced plan=%q stage=%q want plan=%q terminal_create", advanced.PlanPath, next.Stage, wantPath)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.PlanPath != wantPath || persisted.Execution.Pending == nil || persisted.Execution.Pending.Kind != "owner_launch" {
		t.Fatalf("persisted plan/intent=%q %#v", persisted.PlanPath, persisted.Execution.Pending)
	}
	content, err := os.ReadFile(wantPath)
	if err != nil || string(content) != plan {
		t.Fatalf("materialized plan=%q err=%v", content, err)
	}
	if advanced.Execution.Lease.Status != issueops.LeaseStatusReleased || advanced.Execution.Orca != nil {
		t.Fatalf("worktree receipt advanced lease or owner binding: %#v", advanced.Execution)
	}
}
