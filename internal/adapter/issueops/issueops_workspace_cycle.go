package issueops

import (
	"strings"

	"agent-harness/internal/adapter/issueops/active"
)

// PreparedBaseBranchForWorkspace는 주어진 워크스페이스에서 진행 중인 사이클이
// 준비해 둔 부모 작업 브랜치를 돌려준다.
//
// `issueops branch prepare`가 자식 사이클의 base_branch를 우산 브랜치로
// 고정하므로(umbrellaBaseBranchMismatch), 이 값은 그 사이클의 PR/MR이
// 향해야 하는 유일한 타겟이다. 원격 쓰기 전에 타겟을 판정하려는 호출자가
// 레코드 전체를 알 필요 없이 비교 기준만 받도록 문자열로 좁혀 돌려준다.
//
// 진행 중인 사이클이 없거나 아직 브랜치를 준비하지 않았으면 false다. 그 경우
// 비교할 기준이 없으므로 호출자는 판정을 하지 않아야 한다.
func PreparedBaseBranchForWorkspace(workspace string) (string, bool) {
	record, ok := active.CycleForWorkspace(issueOpsActiveStore(), workspace)
	if !ok || record.BranchPrepare == nil {
		return "", false
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return "", false
	}
	return base, true
}
