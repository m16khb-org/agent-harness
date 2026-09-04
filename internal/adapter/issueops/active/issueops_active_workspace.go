package active

import (
	"issueops/internal/adapter/issueops/pathutil"
	model "issueops/internal/contract/issueops"
)

// CycleForWorkspace는 주어진 경로에서 작업 중인 non-done 사이클을 돌려준다.
//
// 경로는 연결된 워크트리일 수도 있고 소스 체크아웃일 수도 있다. 원격 쓰기
// 명령은 둘 중 어디서든 실행되므로 두 경우를 모두 받는다. 워크트리를 먼저
// 보는 이유는 한 소스 repo에 여러 자식 사이클이 붙을 수 있어서, 워크트리가
// 더 좁은 식별자이기 때문이다.
//
// done 사이클은 제외한다. 이미 정리된 사이클의 base_branch로 지금 열리는
// MR을 판정하면 사라진 브랜치를 요구하게 된다.
func CycleForWorkspace(store Store, workspace string) (model.IssueOpsRecord, bool) {
	workspace = pathutil.CleanAbsPath(workspace)
	if workspace == "" {
		return model.IssueOpsRecord{}, false
	}
	stateRoot := store.StateRoot()
	records, err := scanActiveRecords(store, stateRoot)
	if err != nil {
		return model.IssueOpsRecord{}, false
	}

	var repoMatch model.IssueOpsRecord
	repoMatched := false
	for _, record := range records {
		if record.Phase == model.IssueOpsPhaseDone {
			continue
		}
		if pathutil.CleanAbsPath(record.WorktreePath) == workspace {
			return record, true
		}
		if !repoMatched && pathutil.CleanAbsPath(record.Repo) == workspace {
			repoMatch, repoMatched = record, true
		}
	}
	return repoMatch, repoMatched
}
