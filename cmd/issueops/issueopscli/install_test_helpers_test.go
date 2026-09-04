package issueopscli

import (
	augmentplaninsdeps "issueops/cmd/issueops/selfworkflow/augmentplan"
	installadapter "issueops/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	augmentplaninsdeps.ListSkillNames = installadapter.ListSkillNames
}
