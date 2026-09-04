package augmentplan

import (
	qagateinsdeps "issueops/cmd/issueops/validationcli/qagate"
	installadapter "issueops/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
