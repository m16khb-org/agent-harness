package issueops

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// 실행 API의 요청·결과·핸들러는 계약이다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	ExecutionClaimRequest             = issueopscontract.ExecutionClaimRequest
	ExecutionCompleteHandler          = issueopscontract.ExecutionCompleteHandler
	ExecutionCompleteRequest          = issueopscontract.ExecutionCompleteRequest
	ExecutionPrepareRequest           = issueopscontract.ExecutionPrepareRequest
	ExecutionPrepareResult            = issueopscontract.ExecutionPrepareResult
	ExecutionReconcileRequest         = issueopscontract.ExecutionReconcileRequest
	ExecutionReconcileResult          = issueopscontract.ExecutionReconcileResult
	ExecutionReleaseHandler           = issueopscontract.ExecutionReleaseHandler
	ExecutionReleaseRequest           = issueopscontract.ExecutionReleaseRequest
	ExecutionReplaceResult            = issueopscontract.ExecutionReplaceResult
	ExecutionResult                   = issueopscontract.ExecutionResult
	ExecutionResumeHandler            = issueopscontract.ExecutionResumeHandler
	ExecutionResumeRequest            = issueopscontract.ExecutionResumeRequest
	ExecutionResumeResult             = issueopscontract.ExecutionResumeResult
	ExecutionSwitchModeDependencies   = issueopscontract.ExecutionSwitchModeDependencies
	ExecutionSwitchModeRequest        = issueopscontract.ExecutionSwitchModeRequest
	ExecutionSwitchModeResult         = issueopscontract.ExecutionSwitchModeResult
	ExecutionSyncBaseDeps             = issueopscontract.ExecutionSyncBaseDeps
	ExecutionSyncBaseRequest          = issueopscontract.ExecutionSyncBaseRequest
	ExecutionSyncBaseResult           = issueopscontract.ExecutionSyncBaseResult
	RemotePullRequestReconcileHandler = issueopscontract.RemotePullRequestReconcileHandler
	RemotePullRequestRequest          = issueopscontract.RemotePullRequestRequest
)

// 어휘 상수도 계약이 소유한다.
const (
	ExecutionActionClaim            = issueopscontract.ExecutionActionClaim
	ExecutionActionComplete         = issueopscontract.ExecutionActionComplete
	ExecutionActionPrepare          = issueopscontract.ExecutionActionPrepare
	ExecutionActionReconcile        = issueopscontract.ExecutionActionReconcile
	ExecutionActionRelease          = issueopscontract.ExecutionActionRelease
	ExecutionActionReplace          = issueopscontract.ExecutionActionReplace
	ExecutionActionResume           = issueopscontract.ExecutionActionResume
	ExecutionActionStatus           = issueopscontract.ExecutionActionStatus
	ExecutionReplaceFinalize        = issueopscontract.ExecutionReplaceFinalize
	ExecutionReplaceFinalizePreview = issueopscontract.ExecutionReplaceFinalizePreview
	ExecutionReplacePreview         = issueopscontract.ExecutionReplacePreview
	ExecutionReplaceReseed          = issueopscontract.ExecutionReplaceReseed
	ExecutionReplaceRevoke          = issueopscontract.ExecutionReplaceRevoke
	ExecutionSyncBaseAbort          = issueopscontract.ExecutionSyncBaseAbort
	ExecutionSyncBaseApply          = issueopscontract.ExecutionSyncBaseApply
	ExecutionSyncBaseFinalize       = issueopscontract.ExecutionSyncBaseFinalize
	ExecutionSyncBasePreview        = issueopscontract.ExecutionSyncBasePreview
	IssueOpsPhaseAISlopClean        = issueopscontract.IssueOpsPhaseAISlopClean
	IssueOpsPhaseDone               = issueopscontract.IssueOpsPhaseDone
	IssueOpsPhaseGrill              = issueopscontract.IssueOpsPhaseGrill
	IssueOpsPhaseImplement          = issueopscontract.IssueOpsPhaseImplement
	IssueOpsPhasePR                 = issueopscontract.IssueOpsPhasePR
	IssueOpsPhasePlan               = issueopscontract.IssueOpsPhasePlan
	IssueOpsPhaseProblem            = issueopscontract.IssueOpsPhaseProblem
)

// sentinel 오류도 계약이 소유한다.
var (
	ErrClaimHandlerUnavailable                      = issueopscontract.ErrClaimHandlerUnavailable
	ErrCompleteHandlerUnavailable                   = issueopscontract.ErrCompleteHandlerUnavailable
	ErrPrepareHandlerUnavailable                    = issueopscontract.ErrPrepareHandlerUnavailable
	ErrReconcileHandlerUnavailable                  = issueopscontract.ErrReconcileHandlerUnavailable
	ErrReleaseHandlerUnavailable                    = issueopscontract.ErrReleaseHandlerUnavailable
	ErrRemotePullRequestCreateHandlerUnavailable    = issueopscontract.ErrRemotePullRequestCreateHandlerUnavailable
	ErrRemotePullRequestReconcileHandlerUnavailable = issueopscontract.ErrRemotePullRequestReconcileHandlerUnavailable
	ErrReseedHandlerUnavailable                     = issueopscontract.ErrReseedHandlerUnavailable
	ErrResumeHandlerUnavailable                     = issueopscontract.ErrResumeHandlerUnavailable
)

// port가 소유하는 실행 준비 경로도 같은 이름으로 재노출한다.
type (
	ExecutionIssueSnapshotReadFunc = port.ExecutionIssueSnapshotReadFunc
	ExecutionPrepareInvocation     = port.ExecutionPrepareInvocation
)

// cleanup DTO도 계약이 소유한다.
type (
	CleanupAbandonRequest      = issueopscontract.CleanupAbandonRequest
	CleanupFinishRequest       = issueopscontract.CleanupFinishRequest
	CleanupRemoteBranchRequest = issueopscontract.CleanupRemoteBranchRequest
	IssueOpsActor              = issueopscontract.IssueOpsActor
)

type (
	CleanupAbandonResult      = issueopscontract.CleanupAbandonResult
	CleanupFinishResult       = issueopscontract.CleanupFinishResult
	CleanupRemoteBranchResult = issueopscontract.CleanupRemoteBranchResult
)
