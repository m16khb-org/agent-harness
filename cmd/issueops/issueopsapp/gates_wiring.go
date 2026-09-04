package issueopsapp

import (
	"issueops/cmd/issueops/gatescli"
	gatesadapter "issueops/internal/adapter/gates"
	gatesgatedeps "issueops/internal/adapter/issueops/gatesgate"
)

// gatesDependencies는 gates CLI에 concrete adapter를 조립해 넘긴다.
func gatesDependencies() gatescli.Dependencies {
	return gatescli.Dependencies{
		Check:   gatesadapter.Check,
		Init:    gatesadapter.Init,
		Abandon: gatesadapter.Abandon,
	}
}

// configureGatesGate는 IssueOps PR readiness에 합성되는 gates ledger 조회·평가
// 연산을 설치한다. 크로스 케퍼빌리티 adapter edge는 composition root만 만든다.
func configureGatesGate() {
	gatesgatedeps.DiscoverGateFiles = gatesadapter.DiscoverGateFiles
	gatesgatedeps.CheckGateLedger = gatesadapter.Check
}
