package stateio

import (
	augmentplaninsdeps "agent-harness/cmd/harness/selfworkflow/augmentplan"
	qagateinsdeps "agent-harness/cmd/harness/validationcli/qagate"
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	augmentplaninsdeps.ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
