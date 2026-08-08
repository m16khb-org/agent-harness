package vcsissue

import (
	hookclideps "agent-harness/cmd/harness/hookcli"
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다. 이 package의 테스트는
// hookcli의 pre-tool-use 경로를 거쳐 native runtime 진단에 닿는다.
func init() {
	hookclideps.DiagnoseNativeRuntime = installadapter.DiagnoseNativeRuntime
	hookclideps.NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
}
