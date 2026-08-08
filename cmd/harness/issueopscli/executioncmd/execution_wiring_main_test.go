package executioncmd

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 실행 CLI 테스트는 실제 액션 경로를
// 검증하므로 같은 배선을 재현한다.
func wireExecutionForTests() {
	ConfigureExecution(ExecutionDeps{
		ExecuteExecution:             issueopscore.ExecuteExecution,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		IssueOpsStateRoot:            issueopscore.IssueOpsStateRoot,
		SwitchExecutionMode:          issueopscore.SwitchExecutionMode,
		SyncExecutionBase:            issueopscore.SyncExecutionBase,
	})
}

func TestMain(m *testing.M) {
	wireExecutionForTests()
	os.Exit(m.Run())
}
