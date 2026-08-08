// 실행이 참조하는 이슈 스냅샷 DTO다. 스냅샷을 읽어오는 쪽은 I/O를 하지만,
// 결과를 필드로 담아 전달하는 쪽은 그 구현을 알 필요가 없다.
package executionissue

import "context"

type ExecutionIssueSnapshotRequest struct {
	Repo string `json:"repo"`
	URL  string `json:"url"`
}
type ExecutionIssueSnapshot struct {
	URL    string `json:"url"`
	Body   string `json:"body"`
	State  string `json:"state,omitempty"`
	Source string `json:"source,omitempty"`
}
type ExecutionIssueSnapshotEvidence struct {
	Provider string `json:"provider"`
	Source   string `json:"source"`
	WebURL   string `json:"web_url"`
	Body     string `json:"body"`
	State    string `json:"state"`
}

// IssueProviderCreatePullRequestResult reports the outcome.
type IssueProviderCreatePullRequestResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}

// 실행 준비가 이슈 스냅샷을 읽는 경로다. port 인터페이스만 참조하므로 port
// 계층이 소유한다 — 계약 계층은 port를 참조할 수 없다.
type ExecutionIssueSnapshotReadFunc func(context.Context, string, ExecutionIssueSnapshotRequest) (ExecutionIssueSnapshot, error)
type ExecutionPrepareInvocation struct {
	ReadIssue ExecutionIssueSnapshotReadFunc
}
