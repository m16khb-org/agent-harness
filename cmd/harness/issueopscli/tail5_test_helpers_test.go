package issueopscli

import (
	loopgatet5d "agent-harness/internal/adapter/issueops/loopgate"
	pathutiladapter "agent-harness/internal/adapter/issueops/pathutil"
	looprunadapter "agent-harness/internal/adapter/looprun"
	operationalhealtht5d "agent-harness/internal/adapter/operationalhealth"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	loopgatet5d.RepoGateMissing = looprunadapter.RepoGateMissing
	operationalhealtht5d.CleanAbsPath = pathutiladapter.CleanAbsPath
}
