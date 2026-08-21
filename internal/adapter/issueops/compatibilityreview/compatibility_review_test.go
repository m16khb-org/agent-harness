package compatibilityreview

import (
	"errors"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func reviewStore(readinessMissing []string) (Store, *model.IssueOpsRecord) {
	record := &model.IssueOpsRecord{
		OK: true, ID: "io-c", Repo: "/repo", Branch: "12-c", Phase: model.IssueOpsPhasePlan,
	}
	return Store{
		Read: func(string, string) (model.IssueOpsRecord, error) { return *record, nil },
		TouchWrite: func(_ string, rec model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			*record = rec
			return rec, nil
		},
		Ready: func(model.IssueOpsRecord) model.IssueOpsReadiness {
			return model.IssueOpsReadiness{Ready: len(readinessMissing) == 0, Missing: readinessMissing}
		},
		PhaseRank: func(phase model.IssueOpsPhase) int {
			order := map[model.IssueOpsPhase]int{
				model.IssueOpsPhaseProblem: 1, model.IssueOpsPhaseGrill: 2, model.IssueOpsPhasePlan: 3,
				model.IssueOpsPhaseCompatibilityReview: 4, model.IssueOpsPhaseImplement: 5,
			}
			return order[phase]
		},
	}, record
}

// Validate의 필수 목록/rollback/blocker 규칙과 RedactFreeform 적용을 잠근다.
func TestValidateRules(t *testing.T) {
	base := model.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{" additive field ", "additive field", ""},
		SideEffects:           []string{"none"},
		Verification:          []string{"go test ./..."},
		RollbackPlan:          "git revert",
	}
	review, err := Validate(base)
	if err != nil {
		t.Fatal(err)
	}
	// 중복/빈 항목은 정리되고 redaction이 적용된다.
	if len(review.BackwardCompatibility) != 1 || review.SideEffects[0] != "none" {
		t.Fatalf("cleaned lists wrong: %#v", review)
	}
	if review.RollbackPlan == "" || !review.Approved == false && review.Blockers != nil && len(review.Blockers) > 0 {
		t.Fatalf("blocker state wrong: %#v", review)
	}
	approved := base
	approved.Approved = true
	if _, err := Validate(approved); err != nil {
		t.Fatalf("clean approval must pass: %v", err)
	}
	blocker := base
	blocker.Blockers = []string{" schema drift "}
	if review, err := Validate(blocker); err != nil || len(review.Blockers) != 1 || review.Approved {
		t.Fatalf("blocker projection wrong: %#v err=%v", review, err)
	}
	blocked := base
	blocked.Approved = true
	blocked.Blockers = []string{"drift"}
	if _, err := Validate(blocked); err == nil || !strings.Contains(err.Error(), "must not have blockers") {
		t.Fatalf("approved-with-blockers must fail: %v", err)
	}
	for _, missing := range []struct {
		field string
		req   model.IssueOpsCompatibilityReviewRequest
	}{
		{"backward_compatibility", model.IssueOpsCompatibilityReviewRequest{SideEffects: []string{"n"}, Verification: []string{"v"}, RollbackPlan: "r"}},
		{"side_effects", model.IssueOpsCompatibilityReviewRequest{BackwardCompatibility: []string{"b"}, Verification: []string{"v"}, RollbackPlan: "r"}},
		{"verification", model.IssueOpsCompatibilityReviewRequest{BackwardCompatibility: []string{"b"}, SideEffects: []string{"s"}, RollbackPlan: "r"}},
	} {
		if _, err := Validate(missing.req); err == nil || !strings.Contains(err.Error(), missing.field) {
			t.Fatalf("missing %s must fail: %v", missing.field, err)
		}
	}
	if _, err := Validate(model.IssueOpsCompatibilityReviewRequest{BackwardCompatibility: []string{"b"}, SideEffects: []string{"s"}, Verification: []string{"v"}}); err == nil || !strings.Contains(err.Error(), "rollback_plan is required") {
		t.Fatalf("missing rollback must fail: %v", err)
	}
}

// Record는 readiness 게이트를 통과할 때만 기록하고 phase를 compatibility-review로
// 진행시킨다. 준비 부족은 구체적 missing 목록과 함께 거부한다.
func TestRecordGatesAndPhaseAdvance(t *testing.T) {
	req := model.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"ok"},
		SideEffects:           []string{"none"},
		Verification:          []string{"tests"},
		RollbackPlan:          "revert",
		Approved:              true,
	}
	store, record := reviewStore(nil)
	updated, err := Record(store, "state", record.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CompatibilityReview == nil || !updated.CompatibilityReview.Approved {
		t.Fatalf("review not recorded: %#v", updated)
	}
	if record.Phase != model.IssueOpsPhaseCompatibilityReview {
		t.Fatalf("phase must advance from plan: %v", record.Phase)
	}
	gated, gatedRecord := reviewStore([]string{"plan_path", "worktree_path"})
	if _, err := Record(gated, "state", gatedRecord.ID, req); err == nil ||
		!strings.Contains(err.Error(), "missing plan_path, worktree_path") {
		t.Fatalf("unready record must list missing gates: %v", err)
	}
	// read 실패는 원본 에러로 전파된다.
	failing := Store{Read: func(string, string) (model.IssueOpsRecord, error) {
		return model.IssueOpsRecord{}, errors.New("state unreadable")
	}}
	if _, err := Record(failing, "state", "io-x", req); err == nil || !strings.Contains(err.Error(), "state unreadable") {
		t.Fatalf("read failure must propagate: %v", err)
	}
}
