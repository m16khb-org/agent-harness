package issueopscli

import (
	"errors"

	issueopscontract "issueops/internal/contract/issueops"
)

var errLoopGateNotConfigured = errors.New("issueops loop gate is not configured")

// 단계 전진과 PR 준비도 판정은 상태 저장소를 읽고 쓰는 I/O다. CLI는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var (
	advancePhaseWithActor = func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{}, errLoopGateNotConfigured
	}
	strictPRReadinessWithState = func(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
		return issueopscontract.IssueOpsReadiness{}
	}
)

// LoopGateDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type LoopGateDeps struct {
	AdvancePhaseWithActor      func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	StrictPRReadinessWithState func(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness
}

func ConfigureLoopGate(deps LoopGateDeps) {
	if deps.AdvancePhaseWithActor != nil {
		advancePhaseWithActor = deps.AdvancePhaseWithActor
	}
	if deps.StrictPRReadinessWithState != nil {
		strictPRReadinessWithState = deps.StrictPRReadinessWithState
	}
}
