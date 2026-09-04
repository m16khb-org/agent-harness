package gatescli

import (
	"issueops/internal/adapter/gates"
	policyadapter "issueops/internal/adapter/policy"
)

// 실제 adapter 구현을 쓴다(policy 실행 포함). CLI flag 해석과 출력만 여기서
// 검증한다. gates adapter의 policy 실행기는 composition root가 설치하는
// 함수 변수이므로 테스트에서도 프로덕션 배선과 같은 구현을 넣는다.
var (
	adapterCheck   = gates.Check
	adapterInit    = gates.Init
	adapterAbandon = gates.Abandon
)

func init() {
	gates.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	gates.RunCommand = policyadapter.RunCommand
}
