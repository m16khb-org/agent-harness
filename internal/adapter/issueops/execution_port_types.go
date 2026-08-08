package issueops

import (
	"context"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/port"
	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

// port 인터페이스를 필드나 시그니처로 갖는 선언은 계약이 아니라 어댑터가
// 소유한다. contract 계층은 port를 참조할 수 없다.
type RemotePullRequestCreateHandler func(context.Context, string, RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
type ExecutionActionRequest struct {
	Action                string                               `json:"action"`
	ID                    string                               `json:"id"`
	Mode                  string                               `json:"mode,omitempty"`
	Actor                 issueopscontract.NativeActor         `json:"actor,omitempty"`
	CWD                   string                               `json:"cwd,omitempty"`
	OwnerHost             string                               `json:"owner_host,omitempty"`
	OwnerModel            string                               `json:"owner_model,omitempty"`
	OwnerEffort           string                               `json:"owner_effort,omitempty"`
	Generation            uint64                               `json:"generation,omitempty"`
	ExpectedGeneration    uint64                               `json:"expected_generation,omitempty"`
	CompletionGeneration  uint64                               `json:"completion_generation,omitempty"`
	TokenFile             string                               `json:"claim_token_file,omitempty"`
	IssueBodySHA256       string                               `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256   string                               `json:"context_packet_sha256,omitempty"`
	ReplaceAction         string                               `json:"replace_action,omitempty"`
	InventoryFingerprint  string                               `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string                               `json:"quiescence_fingerprint,omitempty"`
	Reason                string                               `json:"reason,omitempty"`
	Preview               bool                                 `json:"preview,omitempty"`
	Confirm               bool                                 `json:"confirm,omitempty"`
	FinalHead             string                               `json:"final_head,omitempty"`
	TuringReportPath      string                               `json:"turing_report_path,omitempty"`
	Verification          []string                             `json:"verification,omitempty"`
	RemoteArtifactURL     string                               `json:"remote_artifact_url,omitempty"`
	IssueSnapshot         *port.ExecutionIssueSnapshotEvidence `json:"issue_snapshot,omitempty"`
}
type ExecutionActionDependencies struct {
	Prepare   ExecutionPrepareHandler
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	BaseSync  basesyncport.Inspector
	ReadIssue ExecutionIssueSnapshotReadFunc
	Claim     ExecutionClaimHandler
	Release   ExecutionReleaseHandler
	Reseed    ExecutionReseedHandler
	Resume    ExecutionResumeHandler
	Reconcile ExecutionReconcileHandler
	Complete  ExecutionCompleteHandler
	// RemoteReconcile handles remote_pr_create recovery independently of the
	// Orca-specific Reconcile handler.
	RemoteReconcile RemotePullRequestReconcileHandler
}
type ExecutionIssueSnapshotReadFunc func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error)
type ExecutionReconcileDependencies struct {
	Orca            port.ExecutionOrcaProvisioner
	ReadIssue       ExecutionIssueSnapshotReadFunc
	RemoteReconcile RemotePullRequestReconcileHandler
	Now             func() time.Time
	Handler         ExecutionReconcileHandler
}

// ExecutionOrcaProvisioner는 core facade가 port를 직접 import하지 않고도
// cleanup abandon의 orca 인벤토리 실조회 표면을 주입받게 하는 alias다.
type ExecutionOrcaProvisioner = port.ExecutionOrcaProvisioner
type ExecutionOrcaOwnerInspector = port.ExecutionOrcaOwnerInspector
type ExecutionPrepareInvocation struct {
	ReadIssue ExecutionIssueSnapshotReadFunc
}
type ExecutionReconcileHandler func(context.Context, string, ExecutionReconcileRequest, ExecutionReconcileDependencies) (ExecutionReconcileResult, error)
type RemotePublicationHandlers struct {
	Create    RemotePullRequestCreateHandler
	Reconcile RemotePullRequestReconcileHandler
}
type ExecutionClaimDependencies struct {
	ReadIssue ExecutionIssueSnapshotReadFunc
}
type ExecutionReseedRequest struct {
	ID                   string                         `json:"id"`
	ExpectedGeneration   uint64                         `json:"expected_generation"`
	CompletionGeneration uint64                         `json:"completion_generation,omitempty"`
	InventoryFingerprint string                         `json:"inventory_fingerprint,omitempty"`
	Reason               string                         `json:"reason,omitempty"`
	Actor                issueopscontract.NativeActor   `json:"actor"`
	CWD                  string                         `json:"cwd"`
	Confirm              bool                           `json:"confirm,omitempty"`
	ReadIssue            ExecutionIssueSnapshotReadFunc `json:"-"`
}
type ExecutionPrepareHandler func(context.Context, string, ExecutionPrepareRequest, ExecutionPrepareInvocation) (ExecutionPrepareResult, error)
type ExecutionClaimHandler func(context.Context, string, ExecutionClaimRequest, ExecutionClaimDependencies) (ExecutionResult, error)
type ExecutionReseedHandler func(context.Context, string, ExecutionReseedRequest) (ExecutionReplaceResult, error)
