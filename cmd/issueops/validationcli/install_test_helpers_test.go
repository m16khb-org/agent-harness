package validationcli

import (
	augmentplaninsdeps "issueops/cmd/issueops/selfworkflow/augmentplan"
	nativeintegrationinsdeps "issueops/cmd/issueops/validationcli/nativeintegration"
	qagateinsdeps "issueops/cmd/issueops/validationcli/qagate"
	installadapter "issueops/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	augmentplaninsdeps.ListSkillNames = installadapter.ListSkillNames
	nativeintegrationinsdeps.ListSkillNames = installadapter.ListSkillNames
	qagateinsdeps.ListSkillNames = installadapter.ListSkillNames
}
