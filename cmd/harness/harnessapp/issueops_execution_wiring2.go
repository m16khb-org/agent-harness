package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli/executioncmd"
	"agent-harness/cmd/harness/mcpcli"
	issueopscore "agent-harness/internal/adapter/issueops"
)

// 실행 CLI는 액션 구현을 알지 않는다. 어댑터를 아는 곳은 composition root
// 하나뿐이다.
func configureIssueOpsExecutionRunners() {
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
