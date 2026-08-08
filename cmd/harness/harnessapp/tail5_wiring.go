package harnessapp

import (
	hookclideps "agent-harness/cmd/harness/hookcli"
	loopgateadapter "agent-harness/internal/adapter/issueops/loopgate"
	pathutiladapter "agent-harness/internal/adapter/issueops/pathutil"
	doctargetadapter "agent-harness/internal/adapter/lifecycle/doctarget"
	looprunadapter "agent-harness/internal/adapter/looprun"
	operationalhealthadapter "agent-harness/internal/adapter/operationalhealth"
)

// configureTail5는 경로 정리와 loop gate 조회를 설치한다.
func configureTail5() {
	operationalhealthadapter.CleanAbsPath = pathutiladapter.CleanAbsPath
	loopgateadapter.RepoGateMissing = looprunadapter.RepoGateMissing
	hookclideps.ToolUseMayMutateLifecycleFiles = doctargetadapter.ToolUseMayMutateLifecycleFiles
}
