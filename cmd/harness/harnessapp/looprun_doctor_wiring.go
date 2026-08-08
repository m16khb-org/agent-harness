package harnessapp

import (
	"agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/looprun"
)

// configureDoctorLoopGate는 doctor가 읽는 loop gate 조회를 설치한다.
func configureDoctorLoopGate() {
	doctor.RepoGateSummaryFor = looprun.RepoGateSummaryFor
	doctor.LoopStateRoot = looprun.StateRoot
}
