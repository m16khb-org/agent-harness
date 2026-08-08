package port

import "context"

// 실행 준비가 이슈 스냅샷을 읽는 경로다. port 인터페이스만 참조하므로 port
// 계층이 소유한다 — 계약 계층은 port를 참조할 수 없다.
type ExecutionIssueSnapshotReadFunc func(context.Context, string, ExecutionIssueSnapshotRequest) (ExecutionIssueSnapshot, error)
type ExecutionPrepareInvocation struct {
	ReadIssue ExecutionIssueSnapshotReadFunc
}
