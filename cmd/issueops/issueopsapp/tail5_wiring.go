package issueopsapp

import (
	loopgateadapter "issueops/internal/adapter/issueops/loopgate"
	pathutiladapter "issueops/internal/adapter/issueops/pathutil"
	looprunadapter "issueops/internal/adapter/looprun"
	operationalhealthadapter "issueops/internal/adapter/operationalhealth"
)

// configureTail5는 경로 정리와 loop gate 조회를 설치한다.
func configureTail5() {
	operationalhealthadapter.CleanAbsPath = pathutiladapter.CleanAbsPath
	loopgateadapter.RepoGateMissing = looprunadapter.RepoGateMissing
}
