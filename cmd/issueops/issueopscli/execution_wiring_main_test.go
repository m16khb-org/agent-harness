package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/executioncmd"
	"issueops/cmd/issueops/mcpcli"
	issueopscore "issueops/internal/adapter/issueops"
)

// 프로덕션에서는 issueopsapp이 주입한다. 실행 계약 테스트는 CLI와 MCP 표면을 함께
// 비교하므로 같은 배선을 재현한다.
func wireExecutionRunnersForTests() {
	executioncmd.ConfigureExecution(executioncmd.ExecutionDeps{
		ExecuteExecution:             issueopscore.ExecuteExecution,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		IssueOpsStateRoot:            issueopscore.IssueOpsStateRoot,
		SwitchExecutionMode:          issueopscore.SwitchExecutionMode,
		SyncExecutionBase:            issueopscore.SyncExecutionBase,
	})
	mcpcli.ConfigureExecution(mcpcli.ExecutionDeps{
		ExecuteExecution:             issueopscore.ExecuteExecution,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		IssueOpsStateRoot:            issueopscore.IssueOpsStateRoot,
		SwitchExecutionMode:          issueopscore.SwitchExecutionMode,
		SyncExecutionBase:            issueopscore.SyncExecutionBase,
	})
}
