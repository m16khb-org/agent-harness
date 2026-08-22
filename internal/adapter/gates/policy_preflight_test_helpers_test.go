package gates

import (
	policyadapter "agent-harness/internal/adapter/policy"
)

// production wiring과 같은 실행기를 설치한다.
func init() {
	EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	RunCommand = policyadapter.RunCommand
}
