package issueopspreparation

import (
	"encoding/json"
	"strings"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

// PlannerGate는 owner가 스스로 채울 수 없는 planner 소유 전제 하나다.
type PlannerGate struct {
	// Key는 owner가 나중에 받게 될 missing key와 같은 이름이다. 두 곳이 다른
	// 이름을 쓰면 사용자가 같은 사실을 두 번 조사하게 된다.
	Key string
	// Command는 coordinator가 그것을 기록하는 정확한 명령이다. 진단만 있고
	// 명령이 없으면 사용자는 무엇을 실행할지 추측하게 된다.
	Command string
}

// MissingPlannerGates는 Orca owner를 띄우기 전에 반드시 기록돼 있어야 하는
// planner 소유 전제 중 빠진 것을 돌려준다(#319).
//
// 이 검사가 필요한 이유는 owner가 이것들을 보충할 수 없기 때문이다.
// intent contract, design review, devil's advocate review는 planner의 판단이며
// owner packet의 commands map에 없다. 그런데 owner의 compatibility review와
// implement 진입 게이트는 이것들을 요구한다. 없는 상태로 띄우면 owner는
// claim까지 완주한 뒤 채울 수 없는 게이트에 부딪혀 반드시 실패한다.
//
// 실측: lifecycle io-cb83a79e1bfd에서 owner가 claim, await-link,
// link-verified까지 완주한 뒤 "missing design_review, intent_contract"로
// fail-closed 했고, 그 셋이 planner 소유라 보충할 수 없다고 보고했다.
//
// prepare가 이것을 미리 막으면 owner를 헛되이 띄우지 않고, coordinator는
// 무엇을 먼저 해야 하는지 정확한 명령으로 알게 된다.
func MissingPlannerGates(record leasecontract.Record) []PlannerGate {
	id := strings.TrimSpace(record.ID)
	var missing []PlannerGate
	if !plannerIntentRecorded(record.Intent) {
		missing = append(missing, PlannerGate{
			Key:     "intent_contract",
			Command: "agent-harness issueops intent record --id " + id + " --raw-request <TEXT> --interpreted-intent <TEXT> --success-criteria <TEXT> ...",
		})
	}
	if !plannerDesignReviewApproved(record.DesignReview) {
		missing = append(missing, PlannerGate{
			Key:     "design_review",
			Command: "agent-harness issueops design review --id " + id + " --problem-summary <TEXT> --proposed-design <TEXT> --verification <TEXT> --approved ...",
		})
	}
	if !plannerDevilsAdvocateCleared(record.DevilsAdvocateReview) {
		missing = append(missing, PlannerGate{
			Key:     "devils_advocate_review",
			Command: "agent-harness issueops devils-advocate review --id " + id + " --verdict <VERDICT> ...",
		})
	}
	return missing
}

// PlannerGateKeys는 진단 문구에 넣을 키 목록이다.
func PlannerGateKeys(gates []PlannerGate) []string {
	keys := make([]string, 0, len(gates))
	for _, gate := range gates {
		keys = append(keys, gate.Key)
	}
	return keys
}

func plannerIntentRecorded(raw json.RawMessage) bool {
	var intent struct {
		RawRequest        string   `json:"raw_request"`
		InterpretedIntent string   `json:"interpreted_intent"`
		SuccessCriteria   []string `json:"success_criteria"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &intent) != nil {
		return false
	}
	return strings.TrimSpace(intent.RawRequest) != "" &&
		strings.TrimSpace(intent.InterpretedIntent) != "" &&
		len(nonEmptyValues(intent.SuccessCriteria)) > 0
}

func plannerDesignReviewApproved(raw json.RawMessage) bool {
	var review struct {
		ProblemSummary string   `json:"problem_summary"`
		ProposedDesign string   `json:"proposed_design"`
		Verification   []string `json:"verification"`
		Approved       bool     `json:"approved"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &review) != nil {
		return false
	}
	return review.Approved && strings.TrimSpace(review.ProblemSummary) != "" &&
		strings.TrimSpace(review.ProposedDesign) != "" && len(nonEmptyValues(review.Verification)) > 0
}

// plannerDevilsAdvocateCleared는 review가 기록됐고, stop/revise 판정이면
// 명시적으로 waive됐는지 본다. adapter의 implement-entry 게이트와 같은 규칙이다.
func plannerDevilsAdvocateCleared(raw json.RawMessage) bool {
	var review struct {
		Verdict    string `json:"verdict"`
		Waived     bool   `json:"waived"`
		RecordedAt string `json:"recorded_at"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &review) != nil {
		return false
	}
	if strings.TrimSpace(review.RecordedAt) == "" {
		return false
	}
	verdict := strings.TrimSpace(review.Verdict)
	return !((verdict == "stop" || verdict == "revise") && !review.Waived)
}

func nonEmptyValues(values []string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
