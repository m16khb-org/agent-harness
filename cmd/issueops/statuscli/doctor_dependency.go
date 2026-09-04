package statuscli

import (
	"errors"
	doctorcontract "issueops/internal/contract/doctor"
)

// 진단 실행은 파일시스템과 런타임을 관찰하는 I/O다. CLI는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var harnessDoctor = func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
	return doctorcontract.HarnessDoctorResult{}, errors.New("doctor is not configured")
}

// ConfigureDoctor는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureDoctor(run func(doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error)) {
	if run != nil {
		harnessDoctor = run
	}
}
