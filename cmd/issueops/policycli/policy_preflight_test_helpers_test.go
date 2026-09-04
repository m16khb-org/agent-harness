package policycli

import (
	auditppdeps "issueops/internal/adapter/audit"
	policyadapter "issueops/internal/adapter/policy"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	FakeRunCommand = policyadapter.FakeRunCommand
	RunReadOnlyCommand = policyadapter.RunReadOnlyCommand
	auditppdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
}
