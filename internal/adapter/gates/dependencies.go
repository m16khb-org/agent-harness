package gates

import (
	policycontract "issueops/internal/contract/policy"
)

// 명령 정책 평가·실행은 composition root가 설치한다. 이 package는 크로스
// 케퍼빌리티 adapter import 없이 policy 실행기를 주입받는다(audit 패턴).
var (
	// EvaluateCommandPolicy는 CHECK 명령의 policy 평가다.
	EvaluateCommandPolicy func(req policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation
	// RunCommand는 policy를 통과한 CHECK argv를 실행한다.
	RunCommand func(req policycontract.CommandPolicyRequest) policycontract.CommandRunResult
)
