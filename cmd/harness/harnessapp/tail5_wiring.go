package harnessapp

import (
	loopgateadapter "agent-harness/internal/adapter/issueops/loopgate"
	pathutiladapter "agent-harness/internal/adapter/issueops/pathutil"
	looprunadapter "agent-harness/internal/adapter/looprun"
	operationalhealthadapter "agent-harness/internal/adapter/operationalhealth"
)

// configureTail5는 경로 정리와 loop gate 조회를 설치한다.
func configureTail5() {
	operationalhealthadapter.CleanAbsPath = pathutiladapter.CleanAbsPath
	loopgateadapter.RepoGateMissing = looprunadapter.RepoGateMissing
}
