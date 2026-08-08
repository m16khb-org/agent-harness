package hookcli

import (
	hookcataloginsdeps "agent-harness/cmd/harness/hookcli/hookcatalog"
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	DiagnoseNativeRuntime = installadapter.DiagnoseNativeRuntime
	NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
	hookcataloginsdeps.NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
}
