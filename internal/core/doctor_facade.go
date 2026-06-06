package core

import "agent-harness/internal/core/doctor"

type HarnessDoctorRequest = doctor.HarnessDoctorRequest
type HarnessDoctorResult = doctor.HarnessDoctorResult
type HarnessDoctorCheck = doctor.HarnessDoctorCheck
type HarnessDoctorIssue = doctor.HarnessDoctorIssue
type HarnessDoctorFix = doctor.HarnessDoctorFix

func HarnessDoctor(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	return doctor.HarnessDoctor(req)
}

func shellQuote(s string) string {
	return doctor.ShellQuote(s)
}
