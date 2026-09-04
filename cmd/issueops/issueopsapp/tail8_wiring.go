package issueopsapp

import (
	"issueops/cmd/issueops/basiccli"
	"issueops/cmd/issueops/contractcli"
	guardadapter "issueops/internal/adapter/guard"
	mcpadapter "issueops/internal/adapter/mcp"
	guardcontract "issueops/internal/contract/guard"
)

// configureTail8은 guard 차단 오류 생성과 MCP conformance probe를 설치한다.
func configureTail8() {
	basiccli.NewGuardBlockedError = func(findings []guardcontract.GuardFinding) error {
		return guardadapter.GuardBlockedError{Findings: findings}
	}
	contractcli.ServeConformanceProbe = mcpadapter.ServeConformanceProbe
}
