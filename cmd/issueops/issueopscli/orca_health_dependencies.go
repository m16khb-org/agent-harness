package issueopscli

import (
	"context"

	corehealth "issueops/internal/domain/operationalhealth"
	"issueops/internal/port"
)

// orca 자원 조작과 운영 건강도 수집은 외부 프로세스를 부른다. 어떤 구현을 쓸지는
// composition root의 결정이고, 여기서는 필요한 연산만 안다.
var (
	RemoveOrcaWorktree       func(ctx context.Context, worktreeID string, force bool) error
	NewOrcaExecutionIntent   func() port.ExecutionOrcaProvisioner
	NewOrcaExecutionOwner    func() port.ExecutionOrcaOwnerInspector
	CollectOperationalHealth func(ctx context.Context, repo string) corehealth.Snapshot
)
