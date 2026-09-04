package basiccli

import (
	guardadapter "issueops/internal/adapter/guard"
	traceadapter "issueops/internal/adapter/trace"
	guardcontract "issueops/internal/contract/guard"
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
