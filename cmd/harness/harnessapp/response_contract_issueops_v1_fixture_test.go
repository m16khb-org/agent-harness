package harnessapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
)

func seedIssueOpsExecutionV1Contract(t *testing.T, repo, branch string) string {
	t.Helper()
	record, err := issueops.StartIssueOps(core.IssueOpsStateRoot(), issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", branch)
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.Phase = issueops.IssueOpsPhasePR
	record.IssueURL = "https://github.com/example/repo/issues/69"
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main",
		BaseSHA: strings.Repeat("a", 40), LinkVerified: true,
	}
	record.Execution = &model.ExecutionV1{
		Mode: model.ExecutionModeDirect,
		Workspace: model.WorkspaceV1{
			SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: strings.Repeat("a", 40),
			Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: model.WriteLeaseV1{
			Generation: 1, Status: model.LeaseStatusClaimable, ClaimTokenSHA256: strings.Repeat("b", 64),
		},
	}
	if _, err := issueops.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return record.ID
}
