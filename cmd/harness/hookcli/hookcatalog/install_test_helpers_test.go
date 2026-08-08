package hookcatalog

import (
	installadapter "agent-harness/internal/adapter/install"
)

// production wiring과 같은 install reader를 설치한다.
func init() {
	NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
}
