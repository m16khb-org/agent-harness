package harnessapp

import (
	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/statuscli"
	"agent-harness/internal/adapter/doctor"
)

// CLI는 진단 구현을 알지 않는다. 어댑터를 아는 곳은 composition root 하나뿐이다.
func configureDoctorRunner() {
	basiccli.ConfigureDoctor(doctor.HarnessDoctor)
	statuscli.ConfigureDoctor(doctor.HarnessDoctor)
}
