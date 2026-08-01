package harnessapp

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestIssueOpsPrepareWiringRunsRealDirectPreviewWithoutPersistence(t *testing.T) {
	stateRoot := t.TempDir()
	repo := t.TempDir()
	claimWiringGit(t, repo, "init", "-q", "-b", "main")
	claimWiringGit(t, repo, "config", "user.name", "IssueOps Test")
	claimWiringGit(t, repo, "config", "user.email", "issueops@example.invalid")
	claimWiringGit(t, repo, "commit", "--allow-empty", "-q", "-m", "initial")
	baseHead := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "199-preparation-wiring"})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	direct := &preparationWiringDirectFake{}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Direct: direct, Now: func() time.Time { return time.Date(2026, 8, 2, 4, 5, 6, 7, time.UTC) },
		NewOperationID: func() (string, error) { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil },
	})
	process := &model.NativeProcessReceipt{PID: 199, StartedAt: "2026-08-02T00:00:00Z", Executable: "/usr/local/bin/codex"}

	result, err := handler(context.Background(), stateRoot, issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: "direct", Actor: model.NativeActor{Host: "codex", SessionID: "session", SessionProcess: process},
		CWD: repo, Confirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Preview || result.ResolvedMode != "direct" || direct.calls != 1 {
		t.Fatalf("result=%#v direct calls=%d", result, direct.calls)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution != nil || persisted.WorktreePath != "" {
		t.Fatalf("preview persisted execution: %#v", persisted)
	}
}

type preparationWiringDirectFake struct{ calls int }

func (fake *preparationWiringDirectFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	fake.calls++
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git",
	}, nil
}
