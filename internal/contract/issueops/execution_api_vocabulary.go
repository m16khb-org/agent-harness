package issueops

import "errors"

// 실행 API의 sentinel 오류와 어휘 상수는 계약이다.
var (
	ErrPrepareHandlerUnavailable                    = errors.New("issueops execution prepare handler is not configured")
	ErrClaimHandlerUnavailable                      = errors.New("issueops execution claim handler is not configured")
	ErrReleaseHandlerUnavailable                    = errors.New("issueops execution release handler is not configured")
	ErrReseedHandlerUnavailable                     = errors.New("issueops execution reseed handler is not configured")
	ErrResumeHandlerUnavailable                     = errors.New("issueops execution resume handler is not configured")
	ErrReconcileHandlerUnavailable                  = errors.New("issueops execution reconcile handler is not configured")
	ErrCompleteHandlerUnavailable                   = errors.New("issueops execution complete handler is not configured")
	ErrRemotePullRequestCreateHandlerUnavailable    = errors.New("remote pull request provider is unavailable")
	ErrRemotePullRequestReconcileHandlerUnavailable = errors.New("remote reconcile provider is unavailable")
)

const (
	ExecutionActionPrepare          = "prepare"
	ExecutionActionStatus           = "status"
	ExecutionActionClaim            = "claim"
	ExecutionActionRelease          = "release"
	ExecutionActionReplace          = "replace"
	ExecutionActionResume           = "resume"
	ExecutionActionReconcile        = "reconcile"
	ExecutionActionComplete         = "complete"
	ExecutionReplacePreview         = "preview"
	ExecutionReplaceRevoke          = "revoke"
	ExecutionReplaceFinalizePreview = "finalize-preview"
	ExecutionReplaceFinalize        = "finalize"
	ExecutionReplaceReseed          = "reseed"
	ExecutionSyncBasePreview        = "preview"
	ExecutionSyncBaseApply          = "apply"
	ExecutionSyncBaseFinalize       = "finalize"
	ExecutionSyncBaseAbort          = "abort"
)
