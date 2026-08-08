package issueopscli

import (
	"context"

	operationalhealthadapter "agent-harness/internal/adapter/operationalhealth"
	orcaadapter "agent-harness/internal/adapter/orca"
	corehealth "agent-harness/internal/domain/operationalhealth"
	"agent-harness/internal/port"
)

// production wiring과 같은 orca client와 health collector를 설치한다.
func init() {
	RemoveOrcaWorktree = func(ctx context.Context, worktreeID string, force bool) error {
		return orcaadapter.New().RemoveWorktree(ctx, worktreeID, force)
	}
	NewOrcaExecutionIntent = func() port.ExecutionOrcaProvisioner { return orcaadapter.NewExecution() }
	NewOrcaExecutionOwner = func() port.ExecutionOrcaOwnerInspector { return orcaadapter.NewExecution() }
	CollectOperationalHealth = func(ctx context.Context, repo string) corehealth.Snapshot {
		collector := operationalhealthadapter.Collector{Git: operationalhealthadapter.ExecGitRunner{}, Orca: orcaadapter.New()}
		return collector.Collect(ctx, repo)
	}
}
