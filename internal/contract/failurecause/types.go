// Package failurecause는 실패 원인 분류의 DTO를 소유한다.
//
// 분류 자체는 진단 문자열 redaction이 필요해 adapter에 남지만, 결과를 읽고
// 전달하는 쪽은 분류 구현을 알 필요가 없다.
package failurecause

type Cause string

const (
	None               Cause = "none"
	Model              Cause = "model"
	HarnessEnvironment Cause = "harness_environment"
	Transport          Cause = "transport"
	ContractInput      Cause = "contract_input"
	Unknown            Cause = "unknown"
)

type Evidence struct {
	Cause  Cause  `json:"cause"`
	Code   string `json:"code"`
	Source string `json:"source"`
}

type Result struct {
	Cause    Cause      `json:"cause"`
	Reason   string     `json:"reason"`
	Evidence []Evidence `json:"evidence"`
}
