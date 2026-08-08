package harnessapp

import (
	hookcliinstalldeps "agent-harness/cmd/harness/hookcli"
	hookcataloginstalldeps "agent-harness/cmd/harness/hookcli/hookcatalog"
	augmentplaninstalldeps "agent-harness/cmd/harness/selfworkflow/augmentplan"
	nativeintegrationinstalldeps "agent-harness/cmd/harness/validationcli/nativeintegration"
	qagateinstalldeps "agent-harness/cmd/harness/validationcli/qagate"
	installadapter "agent-harness/internal/adapter/install"
)

// configureInstallReaders는 native runtime 진단과 skill 목록 조회를 설치한다.
func configureInstallReaders() {
	augmentplaninstalldeps.ListSkillNames = installadapter.ListSkillNames
	hookcataloginstalldeps.NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
	hookcliinstalldeps.DiagnoseNativeRuntime = installadapter.DiagnoseNativeRuntime
	hookcliinstalldeps.NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
	nativeintegrationinstalldeps.ListSkillNames = installadapter.ListSkillNames
	qagateinstalldeps.ListSkillNames = installadapter.ListSkillNames
}
