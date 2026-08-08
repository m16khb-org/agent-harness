package operationalhealth

import (
	pathutiladapter "agent-harness/internal/adapter/issueops/pathutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	CleanAbsPath = pathutiladapter.CleanAbsPath
}
