package augmentplan

import (
	qagateinsdeps "agent-harness/cmd/harness/validationcli/qagate"
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
