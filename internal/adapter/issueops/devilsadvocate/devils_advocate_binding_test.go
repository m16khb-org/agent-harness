package devilsadvocate

import (
	"errors"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestValidateRequiresReviewerContext(t *testing.T) {
	base := model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", Findings: []string{"attacked gate 3: no second use case exists"}}
	if _, err := Validate(base); err == nil || !strings.Contains(err.Error(), "reviewer_context") {
		t.Fatalf("missing reviewer_context must fail closed, got %v", err)
	}
	base.ReviewerContext = "peer"
	if _, err := Validate(base); err == nil || !strings.Contains(err.Error(), "reviewer_context") {
		t.Fatalf("unknown reviewer_context must fail, got %v", err)
	}
	for _, ctx := range []string{"subagent", "inline", " Inline "} {
		base.ReviewerContext = ctx
		got, err := Validate(base)
		if err != nil {
			t.Fatalf("%q should validate: %v", ctx, err)
		}
		if got.ReviewerContext != strings.ToLower(strings.TrimSpace(ctx)) {
			t.Fatalf("reviewer_context should be normalized, got %q", got.ReviewerContext)
		}
	}
}

func TestValidatePassRequiresAFinding(t *testing.T) {
	_, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent"})
	if err == nil || !strings.Contains(err.Error(), "pass verdict requires") {
		t.Fatalf("pass without findings is a rubber stamp and must fail, got %v", err)
	}
}

func bindingStore(t *testing.T, initial model.IssueOpsRecord, digest func(string, model.IssueOpsRecord) (string, error)) (*Store, *model.IssueOpsRecord) {
	t.Helper()
	current := initial
	store := &Store{
		Read: func(_, _ string) (model.IssueOpsRecord, error) { return current, nil },
		TouchWrite: func(_ string, r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			current = r
			return r, nil
		},
		PlanDigest: digest,
	}
	return store, &current
}

func TestRecordPropagatesPlanIdentityError(t *testing.T) {
	boom := errors.New("durable plan is empty or unreadable")
	store, _ := bindingStore(t, model.IssueOpsRecord{OK: true, ID: "io-1"}, func(string, model.IssueOpsRecord) (string, error) {
		return "", boom
	})
	_, err := Record(*store, "root", "io-1", model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"f"}})
	if !errors.Is(err, boom) {
		t.Fatalf("plan digest failure (no linked or staged plan) must surface, got %v", err)
	}
}

func TestRecordBindsPlanIdentityAndKeepsRounds(t *testing.T) {
	digest := "d1"
	store, current := bindingStore(t, model.IssueOpsRecord{OK: true, ID: "io-1", PlanPath: "plans/x.md"}, func(root string, _ model.IssueOpsRecord) (string, error) {
		if root != "root" {
			t.Fatalf("state root must reach the resolver, got %q", root)
		}
		return digest, nil
	})
	first, err := Record(*store, "root", "io-1", model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "revise", ReviewerContext: "inline", Findings: []string{"gate 1: accidental layer"}})
	if err != nil {
		t.Fatal(err)
	}
	r1 := first.DevilsAdvocateReview
	if r1.ReviewedPlanDigest != "d1" || r1.ReviewerContext != "inline" || len(r1.History) != 0 {
		t.Fatalf("first round must bind the plan and carry no history: %+v", r1)
	}
	digest = "d2"
	second, err := Record(*store, "root", "io-1", model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"gate 1 resolved: layer removed"}})
	if err != nil {
		t.Fatal(err)
	}
	r2 := second.DevilsAdvocateReview
	if r2.ReviewedPlanDigest != "d2" || r2.Verdict != "pass" {
		t.Fatalf("second round must rebind the current plan: %+v", r2)
	}
	if len(r2.History) != 1 || r2.History[0].Verdict != "revise" || r2.History[0].ReviewedPlanDigest != "d1" || r2.History[0].ReviewerContext != "inline" || len(r2.History[0].Findings) != 1 {
		t.Fatalf("earlier round must be preserved oldest-first: %+v", r2.History)
	}
	third, err := Record(*store, "root", "io-1", model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass", ReviewerContext: "subagent", Findings: []string{"gate 2: one model"}})
	if err != nil {
		t.Fatal(err)
	}
	if h := third.DevilsAdvocateReview.History; len(h) != 2 || h[0].Verdict != "revise" || h[1].Verdict != "pass" || h[1].ReviewedPlanDigest != "d2" {
		t.Fatalf("history must accumulate oldest-first without nesting: %+v", h)
	}
	if current.DevilsAdvocateReview == nil || len(current.DevilsAdvocateReview.History) != 2 {
		t.Fatalf("touch-write must persist the latest round: %+v", current.DevilsAdvocateReview)
	}
}
