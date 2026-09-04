package issueopscli

import (
	loopgatet5d "issueops/internal/adapter/issueops/loopgate"
	pathutiladapter "issueops/internal/adapter/issueops/pathutil"
	looprunadapter "issueops/internal/adapter/looprun"
	operationalhealtht5d "issueops/internal/adapter/operationalhealth"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	loopgatet5d.RepoGateMissing = looprunadapter.RepoGateMissing
	operationalhealtht5d.CleanAbsPath = pathutiladapter.CleanAbsPath
}
