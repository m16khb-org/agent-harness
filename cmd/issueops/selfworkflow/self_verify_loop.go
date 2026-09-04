package selfworkflow

import "issueops/cmd/issueops/selfworkflow/verifyloop"

var ErrSelfVerificationGateFailed = verifyloop.ErrSelfVerificationGateFailed

type SelfVerifyLoopDeps = verifyloop.Deps
type SelfVerifyRequest = verifyloop.Request

func SelfVerify(request SelfVerifyRequest, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	if deps.IssueOpsRoot == nil {
		deps.IssueOpsRoot = IssueOpsRoot
	}
	return verifyloop.SelfVerify(request, deps)
}
