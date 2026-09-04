package issueopspreparation

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func plannerReadyRecord() leasecontract.Record {
	return leasecontract.Record{
		SchemaVersion: 1, ID: "io-planner", Repo: "/repo",
		Intent:               []byte(`{"raw_request":"r","interpreted_intent":"i","success_criteria":["c"]}`),
		DesignReview:         []byte(`{"problem_summary":"p","proposed_design":"d","verification":["v"],"approved":true}`),
		DevilsAdvocateReview: []byte(`{"verdict":"pass","findings":["f"],"reviewer_context":"subagent","reviewed_plan_digest":"abc","recorded_at":"2026-08-09T00:00:00Z"}`),
	}
}

// TestMissingPlannerGatesIsQuietOnACompleteRecord는 정상 상태가 조용한지
// 고정한다. 이 게이트가 시끄러우면 정상 사이클이 prepare에서 막힌다.
func TestMissingPlannerGatesIsQuietOnACompleteRecord(t *testing.T) {
	if gates := MissingPlannerGates(plannerReadyRecord()); len(gates) != 0 {
		t.Fatalf("완비된 record는 게이트를 남기지 않아야 한다: %v", PlannerGateKeys(gates))
	}
}

// TestMissingPlannerGatesNamesEachOwnerUnfillablePrerequisite는 #319의 세
// 번째 결함을 고정한다.
//
// owner는 intent contract, design review, devil's advocate review를 보충할 수
// 없다 — planner의 판단이고 owner packet의 commands map에 없다. 그런데 owner의
// compatibility review와 implement 진입 게이트는 이것들을 요구한다. 없는 채로
// 띄우면 owner는 claim까지 완주한 뒤 반드시 실패한다.
//
// 실측: lifecycle io-cb83a79e1bfd에서 owner가 claim, await-link,
// link-verified까지 완주한 뒤 "missing design_review, intent_contract"로
// fail-closed 했고 그것들이 planner 소유라 보충할 수 없다고 보고했다.
func TestMissingPlannerGatesNamesEachOwnerUnfillablePrerequisite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*leasecontract.Record)
		wantKey string
		wantCmd string
	}{
		{"intent 없음", func(r *leasecontract.Record) { r.Intent = nil }, "intent_contract", "issueops intent record --id io-planner"},
		{"intent 미완", func(r *leasecontract.Record) {
			r.Intent = []byte(`{"raw_request":"r","interpreted_intent":"","success_criteria":["c"]}`)
		}, "intent_contract", "intent record"},
		{"success criteria 공백뿐", func(r *leasecontract.Record) {
			r.Intent = []byte(`{"raw_request":"r","interpreted_intent":"i","success_criteria":["   "]}`)
		}, "intent_contract", "intent record"},
		{"design review 없음", func(r *leasecontract.Record) { r.DesignReview = nil }, "design_review", "issueops design review --id io-planner"},
		{"design 미승인", func(r *leasecontract.Record) {
			r.DesignReview = []byte(`{"problem_summary":"p","proposed_design":"d","verification":["v"],"approved":false}`)
		}, "design_review", "design review"},
		{"devils advocate 없음", func(r *leasecontract.Record) { r.DevilsAdvocateReview = nil }, "devils_advocate_review", "devils-advocate review --id io-planner"},
		{"stop 판정 미waive", func(r *leasecontract.Record) {
			r.DevilsAdvocateReview = []byte(`{"verdict":"stop","recorded_at":"2026-08-09T00:00:00Z"}`)
		}, "devils_advocate_review", "devils-advocate review"},
		{"revise 판정 미waive", func(r *leasecontract.Record) {
			r.DevilsAdvocateReview = []byte(`{"verdict":"revise","recorded_at":"2026-08-09T00:00:00Z"}`)
		}, "devils_advocate_review", "devils-advocate review"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := plannerReadyRecord()
			tc.mutate(&record)
			gates := MissingPlannerGates(record)
			keys := PlannerGateKeys(gates)
			if len(gates) != 1 || keys[0] != tc.wantKey {
				t.Fatalf("정확히 빠진 것만 지목해야 한다: %v", keys)
			}
			if !strings.Contains(gates[0].Command, tc.wantCmd) {
				t.Fatalf("진단이 실행할 명령을 담아야 한다: %q", gates[0].Command)
			}
		})
	}
}

// TestMissingPlannerGatesAcceptsAWaivedAdverseVerdict는 완화 경로를 고정한다.
// stop/revise라도 명시적으로 waive됐으면 planner가 판단을 내린 것이다.
func TestMissingPlannerGatesAcceptsAWaivedAdverseVerdict(t *testing.T) {
	record := plannerReadyRecord()
	record.DevilsAdvocateReview = []byte(`{"verdict":"stop","waived":true,"reviewed_plan_digest":"abc","recorded_at":"2026-08-09T00:00:00Z"}`)
	if gates := MissingPlannerGates(record); len(gates) != 0 {
		t.Fatalf("waive된 판정은 통과해야 한다: %v", PlannerGateKeys(gates))
	}
}

// TestMissingPlannerGatesReportsEveryGapAtOnce는 한 번에 다 알려주는지
// 고정한다. 하나씩 알려주면 coordinator는 prepare를 세 번 실패시켜야 한다.
func TestMissingPlannerGatesReportsEveryGapAtOnce(t *testing.T) {
	gates := MissingPlannerGates(leasecontract.Record{ID: "io-empty"})
	keys := PlannerGateKeys(gates)
	if len(keys) != 3 {
		t.Fatalf("빠진 것을 모두 보고해야 한다: %v", keys)
	}
	for _, want := range []string{"intent_contract", "design_review", "devils_advocate_review"} {
		if !strings.Contains(strings.Join(keys, ","), want) {
			t.Fatalf("%q가 빠졌다: %v", want, keys)
		}
	}
}

// TestMissingPlannerGatesTreatsUnparseableRecordsAsMissing는 fail-closed를
// 고정한다. 읽을 수 없는 기록을 있는 것으로 세면 owner가 다시 헛되이 뜬다.
func TestMissingPlannerGatesTreatsUnparseableRecordsAsMissing(t *testing.T) {
	record := plannerReadyRecord()
	record.DesignReview = []byte(`not json`)
	keys := PlannerGateKeys(MissingPlannerGates(record))
	if len(keys) != 1 || keys[0] != "design_review" {
		t.Fatalf("파싱 실패는 부재로 다뤄야 한다: %v", keys)
	}
}

func TestMissingPlannerGatesRequiresAPlanBoundDevilsAdvocateReview(t *testing.T) {
	record := plannerReadyRecord()
	record.DevilsAdvocateReview = []byte(`{"verdict":"pass","findings":["f"],"reviewer_context":"subagent","recorded_at":"2026-08-28T00:00:00Z"}`)
	gates := MissingPlannerGates(record)
	if keys := PlannerGateKeys(gates); len(keys) != 1 || keys[0] != "devils_advocate_review" {
		t.Fatalf("an unbound review (legacy record) must gate before an owner is launched: %v", keys)
	}
	if !strings.Contains(gates[0].Command, "--reviewer-context subagent") {
		t.Fatalf("command hint must show the binding flag: %q", gates[0].Command)
	}
	record.DevilsAdvocateReview = []byte(`{"verdict":"pass","findings":["f"],"reviewer_context":"subagent","reviewed_plan_digest":"abc","recorded_at":"2026-08-28T00:00:00Z"}`)
	if gates := MissingPlannerGates(record); len(gates) != 0 {
		t.Fatalf("a bound review must be quiet: %v", PlannerGateKeys(gates))
	}
	record.DevilsAdvocateReview = []byte(`{"verdict":"pass","waived":true,"waiver_rationale":"delegated:io-parent parent DA verdict pass","reviewer_pattern":"delegated-parent-review","recorded_at":"2026-08-28T00:00:00Z"}`)
	if gates := MissingPlannerGates(record); len(gates) != 0 {
		t.Fatalf("delegated child reviews inherit the parent verdict and must be quiet: %v", PlannerGateKeys(gates))
	}
}
