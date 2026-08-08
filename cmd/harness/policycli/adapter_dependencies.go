package policycli

import (
	policycontract "agent-harness/internal/contract/policy"
)

// 명령 정책 평가·실행과 git 관측은 composition root가 설치한다. 이 package는
// 프로세스를 어떻게 띄우는지 알지 않는다.
var (
	EvaluateCommandPolicy func(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation
	FakeRunCommand        func(req policycontract.CommandPolicyRequest) policycontract.CommandFakeRunResult
	RunReadOnlyCommand    func(req policycontract.CommandPolicyRequest) policycontract.CommandRunResult
)
