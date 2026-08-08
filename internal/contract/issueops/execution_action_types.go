// 실행 액션의 요청과 핸들러 계약이다. port는 이제 계약 어휘로 말하므로 이
// 선언들은 인터페이스를 물지 않는 한 계약 계층이 소유한다.
package issueops

import (
	"context"

	executionissue "agent-harness/internal/contract/executionissue"
)

// port 인터페이스를 필드나 시그니처로 갖는 선언은 계약이 아니라 어댑터가
// 소유한다. contract 계층은 port를 참조할 수 없다.
type RemotePullRequestCreateHandler func(context.Context, string, RemotePullRequestRequest) (executionissue.IssueProviderCreatePullRequestResult, error)
type ExecutionActionRequest struct {
	Action      string      `json:"action"`
	ID          string      `json:"id"`
	Mode        string      `json:"mode,omitempty"`
	Actor       NativeActor `json:"actor,omitempty"`
	CWD         string      `json:"cwd,omitempty"`
	OwnerHost   string      `json:"owner_host,omitempty"`
	OwnerModel  string      `json:"owner_model,omitempty"`
	OwnerEffort string      `json:"owner_effort,omitempty"`
	// 선택 영수증: 어떤 모드를 요청했고 준비도 탐침이 무엇을 봤는지 보존한다.
	IssueSnapshotFile            string                                         `json:"issue_snapshot_file,omitempty"`
	DirectReason                 string                                         `json:"direct_reason,omitempty"`
	ExpectedReadinessFingerprint string                                         `json:"expected_readiness_fingerprint,omitempty"`
	Generation                   uint64                                         `json:"generation,omitempty"`
	ExpectedGeneration           uint64                                         `json:"expected_generation,omitempty"`
	CompletionGeneration         uint64                                         `json:"completion_generation,omitempty"`
	TokenFile                    string                                         `json:"claim_token_file,omitempty"`
	IssueBodySHA256              string                                         `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256          string                                         `json:"context_packet_sha256,omitempty"`
	ReplaceAction                string                                         `json:"replace_action,omitempty"`
	InventoryFingerprint         string                                         `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint        string                                         `json:"quiescence_fingerprint,omitempty"`
	Reason                       string                                         `json:"reason,omitempty"`
	Preview                      bool                                           `json:"preview,omitempty"`
	Confirm                      bool                                           `json:"confirm,omitempty"`
	FinalHead                    string                                         `json:"final_head,omitempty"`
	TuringReportPath             string                                         `json:"turing_report_path,omitempty"`
	Verification                 []string                                       `json:"verification,omitempty"`
	RemoteArtifactURL            string                                         `json:"remote_artifact_url,omitempty"`
	IssueSnapshot                *executionissue.ExecutionIssueSnapshotEvidence `json:"issue_snapshot,omitempty"`
}
type RemotePublicationHandlers struct {
	Create    RemotePullRequestCreateHandler
	Reconcile RemotePullRequestReconcileHandler
}
type ExecutionClaimDependencies struct {
	ReadIssue executionissue.ExecutionIssueSnapshotReadFunc
}
type ExecutionReseedRequest struct {
	ID                   string                                        `json:"id"`
	ExpectedGeneration   uint64                                        `json:"expected_generation"`
	CompletionGeneration uint64                                        `json:"completion_generation,omitempty"`
	InventoryFingerprint string                                        `json:"inventory_fingerprint,omitempty"`
	Reason               string                                        `json:"reason,omitempty"`
	Actor                NativeActor                                   `json:"actor"`
	CWD                  string                                        `json:"cwd"`
	Confirm              bool                                          `json:"confirm,omitempty"`
	ReadIssue            executionissue.ExecutionIssueSnapshotReadFunc `json:"-"`
}
type ExecutionPrepareHandler func(context.Context, string, ExecutionPrepareRequest, executionissue.ExecutionPrepareInvocation) (ExecutionPrepareResult, error)
type ExecutionClaimHandler func(context.Context, string, ExecutionClaimRequest, ExecutionClaimDependencies) (ExecutionResult, error)
type ExecutionReseedHandler func(context.Context, string, ExecutionReseedRequest) (ExecutionReplaceResult, error)
