package projectbootstrap

import (
	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

// lifecycle state 초기화는 파일시스템에 쓰는 I/O다. bootstrap은 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var initProjectLifecycleState = func(repoRoot string, confirm bool, metadata ...projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return lifecyclecontract.ProjectLifecycleStatePlan{}, nil
}

// ConfigureLifecycle는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureLifecycle(init func(string, bool, ...projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error)) {
	if init != nil {
		initProjectLifecycleState = init
	}
}
