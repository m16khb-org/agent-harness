package gatesgate

import (
	gatesadapter "agent-harness/internal/adapter/gates"
	policyadapter "agent-harness/internal/adapter/policy"
)

// production wiring과 같은 구현을 설치한다. gates adapter의 policy 실행기도
// composition root와 같은 배선으로 넣는다.
func init() {
	DiscoverGateFiles = gatesadapter.DiscoverGateFiles
	CheckGateLedger = gatesadapter.Check
	gatesadapter.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	gatesadapter.RunCommand = policyadapter.RunCommand
}
