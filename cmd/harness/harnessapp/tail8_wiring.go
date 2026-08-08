package harnessapp

import (
	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/contractcli"
	guardadapter "agent-harness/internal/adapter/guard"
	mcpadapter "agent-harness/internal/adapter/mcp"
	guardcontract "agent-harness/internal/contract/guard"
)

// configureTail8은 guard 차단 오류 생성과 MCP conformance probe를 설치한다.
func configureTail8() {
	basiccli.NewGuardBlockedError = func(findings []guardcontract.GuardFinding) error {
		return guardadapter.GuardBlockedError{Findings: findings}
	}
	contractcli.ServeConformanceProbe = mcpadapter.ServeConformanceProbe
}
