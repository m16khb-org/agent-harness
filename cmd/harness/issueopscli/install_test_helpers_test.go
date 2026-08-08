package issueopscli

import (
	augmentplaninsdeps "agent-harness/cmd/harness/selfworkflow/augmentplan"
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	augmentplaninsdeps.ListSkillNames = installadapter.ListSkillNames
}
