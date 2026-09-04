package doctor

import doctorcontract "issueops/internal/contract/doctor"

// 진단 요청·결과는 계약 DTO다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	HarnessDoctorRequest         = doctorcontract.HarnessDoctorRequest
	HarnessDoctorDaemonAdmission = doctorcontract.HarnessDoctorDaemonAdmission
	HarnessDoctorResult          = doctorcontract.HarnessDoctorResult
	HarnessDoctorCheck           = doctorcontract.HarnessDoctorCheck
	HarnessDoctorIssue           = doctorcontract.HarnessDoctorIssue
	HarnessDoctorFix             = doctorcontract.HarnessDoctorFix
)
