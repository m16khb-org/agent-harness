package issueopsapp

import (
	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedIssueOpsExecutionContract(t *testing.T, repo, branch string) string {
	t.Helper()
	record, err := issueops.StartIssueOps(issueops.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
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
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main",
		BaseSHA: strings.Repeat("a", 40), LinkVerified: true,
	}
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: strings.Repeat("a", 40),
			Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusClaimable, ClaimTokenSHA256: strings.Repeat("b", 64),
		},
	}
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return record.ID
}
