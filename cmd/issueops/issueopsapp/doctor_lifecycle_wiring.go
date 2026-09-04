package issueopsapp

import (
	"issueops/internal/adapter/doctor"
	lifecycle "issueops/internal/adapter/lifecycle"
)

// doctor는 lifecycle 어댑터를 직접 알지 않는다. 실제 구현을 아는 곳은
// composition root 하나뿐이다.
func configureDoctorLifecycle() {
	doctor.ConfigureLifecycle(lifecycle.ValidateProjectLifecycleState)
}
