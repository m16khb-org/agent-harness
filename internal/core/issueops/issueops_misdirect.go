package issueops

import (
	"context"
	"time"
)

// IncrementIssueOpsSourceMisdirect는 소스 체크아웃 오편집 경고 관측을
// 사이클 record에 누적한다. 훅은 관측을 전달만 하고 판단하지 않는다 —
// 이 카운터는 strict readiness의 비차단 경고 키로만 쓰인다(설계 v5 WS8).
func IncrementIssueOpsSourceMisdirect(stateRoot, id string) (int, error) {
	count := 0
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		rec.SourceMisdirectWarnings++
		count = rec.SourceMisdirectWarnings
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
