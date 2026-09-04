package feedbackcleanup

import (
	"context"
	"errors"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

var errCleanupNotConfigured = errors.New("issueops cleanup is not configured")

// 정리 연산은 상태 저장소와 원격 제공자를 다루는 I/O다. CLI는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다. Deps 조립도 root의 일이므로
// 여기서는 CLI가 이미 가진 Deps를 그대로 넘긴다.
var cleanupDeps CleanupDeps

// CleanupDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type CleanupDeps struct {
	AddIssueOpsFeedbackWithActor                      func(stateRoot, id, source, body, classification string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	CleanupAbandon                                    func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, deps Deps, prov port.IssueProvider) (issueopscontract.CleanupAbandonResult, error)
	CleanupFinish                                     func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, deps Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error)
	CleanupRemoteBranch                               func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, deps Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error)
	CleanupLinkedBranch                               func(ctx context.Context, stateRoot string, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error)
	CloseIssueOpsChildren                             func(stateRoot, id string, req issueopscontract.IssueOpsCloseChildrenRequest, provider func(string) (port.IssueProvider, error)) (issueopscontract.IssueOpsCloseChildrenResult, error)
	FinalizeIssueOpsCleanupStatus                     func(issueopscontract.IssueOpsCleanupStatus) issueopscontract.IssueOpsCleanupStatus
	IssueOpsCleanupStatusForRecord                    func(issueopscontract.IssueOpsRecord, issueopscontract.IssueOpsCleanupStatusRequest) issueopscontract.IssueOpsCleanupStatus
	IssueOpsRemoteArtifactMissing                     func(issueopscontract.IssueOpsRecord) []string
	IssueOpsStateRoot                                 func() string
	MarkIssueOpsContractFeedbackIssueUpdatedWithActor func(stateRoot, id string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ObserveNativeProcessAncestry                      func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReadIssueOps                                      func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	ReadRemoteIssueSnapshot                           func(ctx context.Context, prov port.IssueProvider, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error)
	ReflectCleanupAudit                               func(stateRoot string, record issueopscontract.IssueOpsRecord, completion port.IssueProviderCompletionSection, audit string, prov port.IssueProvider) error
	ResolveRecordProvider                             func(issueopscontract.IssueOpsRecord) string
}

// ConfigureCleanup는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureCleanup(deps CleanupDeps) { cleanupDeps = deps }

func init() { cleanupDeps = neutralCleanupDeps() }

// 배선 누락이 패닉이 아니라 명시적 오류로 드러나도록 중립 기본값을 둔다.
func neutralCleanupDeps() CleanupDeps {
	return CleanupDeps{
		AddIssueOpsFeedbackWithActor: func(string, string, string, string, string, issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errCleanupNotConfigured
		},
		CleanupAbandon: func(context.Context, string, issueopscontract.CleanupAbandonRequest, Deps, port.IssueProvider) (issueopscontract.CleanupAbandonResult, error) {
			return issueopscontract.CleanupAbandonResult{}, errCleanupNotConfigured
		},
		CleanupFinish: func(context.Context, string, issueopscontract.CleanupFinishRequest, Deps, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			return issueopscontract.CleanupFinishResult{}, errCleanupNotConfigured
		},
		CleanupRemoteBranch: func(context.Context, string, issueopscontract.CleanupRemoteBranchRequest, Deps, port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
			return issueopscontract.CleanupRemoteBranchResult{}, errCleanupNotConfigured
		},
		CleanupLinkedBranch: func(context.Context, string, issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
			return issueopscontract.CleanupLinkedBranchResult{}, errCleanupNotConfigured
		},
		CloseIssueOpsChildren: func(string, string, issueopscontract.IssueOpsCloseChildrenRequest, func(string) (port.IssueProvider, error)) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			return issueopscontract.IssueOpsCloseChildrenResult{}, errCleanupNotConfigured
		},
		FinalizeIssueOpsCleanupStatus: func(s issueopscontract.IssueOpsCleanupStatus) issueopscontract.IssueOpsCleanupStatus { return s },
		IssueOpsCleanupStatusForRecord: func(issueopscontract.IssueOpsRecord, issueopscontract.IssueOpsCleanupStatusRequest) issueopscontract.IssueOpsCleanupStatus {
			return issueopscontract.IssueOpsCleanupStatus{}
		},
		IssueOpsRemoteArtifactMissing: func(issueopscontract.IssueOpsRecord) []string { return nil },
		IssueOpsStateRoot:             func() string { return "" },
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: func(string, string, issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errCleanupNotConfigured
		},
		ObserveNativeProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, errCleanupNotConfigured },
		ReadIssueOps: func(string, string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errCleanupNotConfigured
		},
		ReadRemoteIssueSnapshot: func(context.Context, port.IssueProvider, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			return port.ExecutionIssueSnapshot{}, errCleanupNotConfigured
		},
		ReflectCleanupAudit: func(string, issueopscontract.IssueOpsRecord, port.IssueProviderCompletionSection, string, port.IssueProvider) error {
			return errCleanupNotConfigured
		},
		ResolveRecordProvider: func(issueopscontract.IssueOpsRecord) string { return "" },
	}
}
