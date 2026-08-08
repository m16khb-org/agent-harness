package basiccli

import (
	guardadapter "agent-harness/internal/adapter/guard"
	traceadapter "agent-harness/internal/adapter/trace"
	guardcontract "agent-harness/internal/contract/guard"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	GuardCheck = guardadapter.GuardCheck
	TraceAnalyze = traceadapter.TraceAnalyze
}

func init() {
	NewGuardBlockedError = func(findings []guardcontract.GuardFinding) error {
		return guardadapter.GuardBlockedError{Findings: findings}
	}
}
