package issueops

import (
	"fmt"
	"strings"
)

// UmbrellaBranchGateReason은 우산 사이클이 provider-native 자식 작업 항목을 만들
// 준비가 되지 않은 이유를 돌려준다. 빈 문자열이면 통과다.
//
// 왜 이 게이트가 필요한가: 자식 PR의 base를 검사하는 가드
// (issueOpsPRTargetBranchBlockReason)는 이미 존재하지만, 우산 사이클이 자기
// 브랜치를 갖도록 요구하는 게이트가 없었다. 그 결과 #78에서 자식들이 main과
// 서로에게 뒤엉킨 체인으로 머지됐고, 부모에 원격 artifact가 없어 abandon /
// close-children / finish 세 정리 경로가 순환 차단됐다(#129).
//
// 판정을 CLI가 아니라 core에 두는 이유는 계약을 테스트로 고정하기 위해서다.
func UmbrellaBranchGateReason(record IssueOpsRecord) string {
	branch := strings.TrimSpace(record.Branch)
	if branch == "" {
		return "IssueOps 우산 사이클은 자체 브랜치를 가져야 자식 작업 항목을 만들 수 있다; " +
			"`agent-harness issueops start --branch NAME`으로 브랜치를 정한 뒤 `agent-harness issueops branch prepare`를 실행하라"
	}
	if record.BranchPrepare == nil {
		return fmt.Sprintf("IssueOps 우산 사이클 %s는 자식 작업 항목을 만들기 전에 우산 브랜치 %s를 준비해야 한다; "+
			"`agent-harness issueops branch prepare --id %s --branch %s --base-branch REF`를 먼저 실행하라",
			record.ID, branch, record.ID, branch)
	}
	if prepared := strings.TrimSpace(record.BranchPrepare.Branch); prepared != branch {
		return fmt.Sprintf("IssueOps 우산 사이클 %s의 branch_prepare가 다른 브랜치 %s를 가리킨다; "+
			"우산 브랜치 %s로 다시 준비한 뒤 자식 작업 항목을 만들라", record.ID, prepared, branch)
	}
	return ""
}
