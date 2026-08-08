package issueops

import (
	"agent-harness/internal/port"
)

// 실행 의존 묶음과 그 핸들러는 port가 소유한다. 어댑터는 같은 이름으로 재노출만
// 한다.
type (
	ExecutionActionDependencies    = port.ExecutionActionDependencies
	ExecutionReconcileDependencies = port.ExecutionReconcileDependencies
	ExecutionReconcileHandler      = port.ExecutionReconcileHandler
	ExecutionOrcaProvisioner       = port.ExecutionOrcaProvisioner
	ExecutionOrcaOwnerInspector    = port.ExecutionOrcaOwnerInspector
)
