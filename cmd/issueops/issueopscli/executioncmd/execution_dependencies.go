package executioncmd

import (
	"context"
	"errors"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

var errExecutionNotConfigured = errors.New("issueops execution is not configured")

// 실행 액션은 상태 저장소와 Orca를 다루는 I/O다. CLI는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var execDeps = ExecutionDeps{
	ExecuteExecution: func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error) {
		return nil, errExecutionNotConfigured
	},
	ObserveNativeProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
		return nil, errExecutionNotConfigured
	},
	IssueOpsStateRoot: func() string { return "" },
	SwitchExecutionMode: func(context.Context, string, issueopscontract.ExecutionSwitchModeRequest, issueopscontract.ExecutionSwitchModeDependencies) (issueopscontract.ExecutionSwitchModeResult, error) {
		return issueopscontract.ExecutionSwitchModeResult{}, errExecutionNotConfigured
	},
	SyncExecutionBase: func(context.Context, string, issueopscontract.ExecutionSyncBaseRequest, issueopscontract.ExecutionSyncBaseDeps) (issueopscontract.ExecutionSyncBaseResult, error) {
		return issueopscontract.ExecutionSyncBaseResult{}, errExecutionNotConfigured
	},
}

// ExecutionDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type ExecutionDeps struct {
	ExecuteExecution             func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error)
	ObserveNativeProcessAncestry func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	IssueOpsStateRoot            func() string
	SwitchExecutionMode          func(context.Context, string, issueopscontract.ExecutionSwitchModeRequest, issueopscontract.ExecutionSwitchModeDependencies) (issueopscontract.ExecutionSwitchModeResult, error)
	SyncExecutionBase            func(context.Context, string, issueopscontract.ExecutionSyncBaseRequest, issueopscontract.ExecutionSyncBaseDeps) (issueopscontract.ExecutionSyncBaseResult, error)
}

func ConfigureExecution(deps ExecutionDeps) {
	if deps.ExecuteExecution != nil {
		execDeps.ExecuteExecution = deps.ExecuteExecution
	}
	if deps.ObserveNativeProcessAncestry != nil {
		execDeps.ObserveNativeProcessAncestry = deps.ObserveNativeProcessAncestry
	}
	if deps.IssueOpsStateRoot != nil {
		execDeps.IssueOpsStateRoot = deps.IssueOpsStateRoot
	}
	if deps.SwitchExecutionMode != nil {
		execDeps.SwitchExecutionMode = deps.SwitchExecutionMode
	}
	if deps.SyncExecutionBase != nil {
		execDeps.SyncExecutionBase = deps.SyncExecutionBase
	}
}
