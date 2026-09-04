package issueopsapp

import (
	"context"

	"issueops/cmd/issueops/issueopscli"
	operationalhealthadapter "issueops/internal/adapter/operationalhealth"
	orcaadapter "issueops/internal/adapter/orca"
	corehealth "issueops/internal/domain/operationalhealth"
	"issueops/internal/port"
)

// configureIssueOpsRuntime은 orca 자원 조작과 운영 건강도 수집을 IssueOps CLI에
// 조립한다.
//
// 이 조립은 원래 issueopscli 안에 있었다. 어떤 orca client와 어떤 git runner를
// 쓸지는 CLI의 관심사가 아니라 composition root의 결정이다.
func configureIssueOpsRuntime() {
	issueopscli.RemoveOrcaWorktree = func(ctx context.Context, worktreeID string, force bool) error {
		return orcaadapter.New().RemoveWorktree(ctx, worktreeID, force)
	}
	issueopscli.NewOrcaExecutionIntent = func() port.ExecutionOrcaProvisioner { return orcaadapter.NewExecution() }
	issueopscli.NewOrcaExecutionOwner = func() port.ExecutionOrcaOwnerInspector { return orcaadapter.NewExecution() }
	issueopscli.CollectOperationalHealth = func(ctx context.Context, repo string) corehealth.Snapshot {
		collector := operationalhealthadapter.Collector{Git: operationalhealthadapter.ExecGitRunner{}, Orca: orcaadapter.New()}
		return collector.Collect(ctx, repo)
	}
}
