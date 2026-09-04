package issueopsnext

import (
	"context"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

// Ports는 `issueops next`가 필요로 하는 관측이다. 전부 읽기 전용이며, 함수
// 타입으로 두어 composition root가 현행 어댑터 함수를 그대로 꽂는다. 이
// 패키지는 어댑터를 import하지 않는다.
type Ports struct {
	ListCycles func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error)
	ReadRecord func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	Completion func(record issueopscontract.IssueOpsRecord, phase issueopscontract.IssueOpsPhase) issueopscontract.IssueOpsReadiness
	// LocalReadiness는 fetch 없는 PR readiness다. 네트워크를 건드리지 않는다.
	LocalReadiness func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness
	// WriterlessCommand는 writer 없는 lease의 회복 명령이다.
	WriterlessCommand func(record issueopscontract.IssueOpsRecord) string
	PlannerDefaults   func(host string) (model, effort string, ok bool)
	StagedArtifacts   func(stateRoot, id string) ([]string, error)
	Actor             func() (host, sessionID string, err error)
	// ProcessLive는 홀더 프로세스 관측이다. 관측하지 못하면 nil을 돌려주고,
	// 분류기는 그것을 "살아 있다"로 본다.
	ProcessLive func(receipt issueopscontract.NativeProcessReceipt) *bool
	SourceRoot  func(cwd string) string
	// CleanPath는 경로 비교 전 정규화다. 파일시스템 어휘는 composition root가
	// 소유하므로 application 계층은 filepath를 직접 import하지 않는다.
	CleanPath     func(path string) string
	WorktreeState func(root string) (present bool, branch, head string)
	CurrentBranch func(cwd string) string
	Env           func(key string) string
	Now           func() time.Time
}
