package issueopsapp

import (
	augmentplaninstalldeps "issueops/cmd/issueops/selfworkflow/augmentplan"
	nativeintegrationinstalldeps "issueops/cmd/issueops/validationcli/nativeintegration"
	qagateinstalldeps "issueops/cmd/issueops/validationcli/qagate"
	installadapter "issueops/internal/adapter/install"
)

// configureInstallReaders는 native runtime 진단과 skill 목록 조회를 설치한다.
func configureInstallReaders() {
	augmentplaninstalldeps.ListSkillNames = installadapter.ListSkillNames
	nativeintegrationinstalldeps.ListSkillNames = installadapter.ListSkillNames
	qagateinstalldeps.ListSkillNames = installadapter.ListSkillNames
}
