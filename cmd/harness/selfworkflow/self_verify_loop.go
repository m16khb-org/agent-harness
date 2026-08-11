package selfworkflow

import "agent-harness/cmd/harness/selfworkflow/verifyloop"

var ErrSelfVerificationGateFailed = verifyloop.ErrSelfVerificationGateFailed

type SelfVerifyLoopDeps = verifyloop.Deps
type SelfVerifyRequest = verifyloop.Request

func SelfVerify(request SelfVerifyRequest, deps SelfVerifyLoopDeps) (SelfAugmentResult, error) {
	if deps.HarnessRoot == nil {
		deps.HarnessRoot = HarnessRoot
	}
	return verifyloop.SelfVerify(request, deps)
}
