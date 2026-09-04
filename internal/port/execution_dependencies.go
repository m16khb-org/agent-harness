package port

import (
	"context"
	"time"

	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	basesyncport "issueops/internal/port/issueopsbasesync"
)

// 실행 의존 묶음은 port 인터페이스를 필드로 갖는다. DTO가 아니라 주입 묶음이므로
// 계약이 아니라 port 계층이 소유한다 — 계약은 인터페이스를 물지 않는다.
type ExecutionActionDependencies struct {
	Prepare   issueopscontract.ExecutionPrepareHandler
	Orca      ExecutionOrcaProvisioner
	OrcaOwner ExecutionOrcaOwnerInspector
	BaseSync  basesyncport.Inspector
	ReadIssue executionissue.ExecutionIssueSnapshotReadFunc
	Claim     issueopscontract.ExecutionClaimHandler
	Release   issueopscontract.ExecutionReleaseHandler
	Reseed    issueopscontract.ExecutionReseedHandler
	Resume    issueopscontract.ExecutionResumeHandler
	Reconcile ExecutionReconcileHandler
	Complete  issueopscontract.ExecutionCompleteHandler
	// RemoteReconcile handles remote_pr_create recovery independently of the
	// Orca-specific Reconcile handler.
	RemoteReconcile issueopscontract.RemotePullRequestReconcileHandler
}

type ExecutionReconcileDependencies struct {
	Orca            ExecutionOrcaProvisioner
	ReadIssue       executionissue.ExecutionIssueSnapshotReadFunc
	RemoteReconcile issueopscontract.RemotePullRequestReconcileHandler
	Now             func() time.Time
	Handler         ExecutionReconcileHandler
}

// ExecutionReconcileHandler는 의존 묶음을 파라미터로 받으므로 그 묶음과 같은
// 계층에 있어야 한다.
type ExecutionReconcileHandler func(context.Context, string, issueopscontract.ExecutionReconcileRequest, ExecutionReconcileDependencies) (issueopscontract.ExecutionReconcileResult, error)
