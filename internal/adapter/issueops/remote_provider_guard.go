package issueops

import (
	"context"
	"fmt"

	"agent-harness/internal/port"
)

// 아래 세 함수는 provider 호출 전에 사용 가능 여부를 확인하는 얇은 가드다.
// facade가 갖고 있었으나 provider와 IssueOps 워크플로 사이의 규칙이므로 여기가
// 소유다. provider가 없거나 필요한 능력을 갖추지 못한 경우를 호출부마다 반복해서
// 검사하지 않도록 한곳에 모은다.

// CreateRemoteIssue는 provider가 구성되어 있을 때만 원격 이슈를 만든다.
func CreateRemoteIssue(req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error) {
	if prov == nil {
		return port.IssueProviderCreateIssueResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateIssue(req)
}

// CreateRemoteChild는 provider가 구성되어 있을 때만 원격 child work item을 만든다.
func CreateRemoteChild(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error) {
	if prov == nil {
		return port.IssueProviderCreateChildResult{OK: false}, fmt.Errorf("no issue provider configured")
	}
	return prov.CreateChild(req)
}

// ReadRemoteIssueSnapshot은 provider가 snapshot 읽기를 지원할 때만 이슈 본문을
// 읽는다. 모든 provider가 이 능력을 갖추지는 않으므로 타입 단언으로 확인한다.
func ReadRemoteIssueSnapshot(ctx context.Context, prov port.IssueProvider, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	reader, ok := prov.(port.ExecutionIssueSnapshotReader)
	if !ok {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("issue provider does not support issue snapshot reads")
	}
	return reader.ReadIssueSnapshot(ctx, req)
}
