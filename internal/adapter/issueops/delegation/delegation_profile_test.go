package delegation

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// BuildDelegatedProfile은 위임 자식이 부모의 검토 상태를 상속받는 방식을
// 정의한다. dogfooding에서 확인한 그대로: 자식 issue URL은 명시값 우선,
// 부모 intent 원문이 scope 앞에 붙고, plan-prep는 위임 사유로 waived,
// 부모 design/compatibility 리뷰는 승인 상태로 복제된다.
func TestBuildDelegatedProfileInheritsParentState(t *testing.T) {
	now := "2026-08-21T12:00:00Z"
	parent := model.IssueOpsRecord{
		OK: true, ID: "io-parent", Repo: "/repo", Branch: "6000-parent", Phase: model.IssueOpsPhaseImplement,
		IssueURL: "https://example.com/i/6000",
		Intent:   &model.IssueOpsIntentContract{RawRequest: "원 요청", InterpretedIntent: "i", SuccessCriteria: []string{"s"}},
		DomainReview: &model.IssueOpsDomainReview{
			ModelFit: "go", Terminology: []string{"term"}, Risks: []string{"risk"}, OpenUncertainties: []string{"u"},
		},
		DesignReview:        &model.IssueOpsDesignReview{Approved: false, Verification: []string{"v"}},
		CompatibilityReview: &model.IssueOpsCompatibilityReview{Approved: false, Blockers: []string{"b"}},
	}
	child := model.IssueOpsRecord{OK: true, ID: "io-child", Repo: "/repo", Branch: "6001-child", Phase: model.IssueOpsPhaseProblem}
	req := model.IssueOpsChildStartRequest{
		Branch: "6001-child", Title: "child one", TaskScope: "  scope  ",
		AcceptanceCriteria: []string{" a1 ", "", "a2"},
	}
	built := BuildDelegatedProfile(parent, child, req, now)

	if built.Delegation == nil || built.Delegation.ParentCycleID != "io-parent" ||
		built.Delegation.TaskScope != "scope" || len(built.Delegation.AcceptanceCriteria) != 2 {
		t.Fatalf("delegation contract wrong: %#v", built.Delegation)
	}
	if !strings.HasPrefix(built.Intent.RawRequest, "원 요청\n\nDelegated task: scope") {
		t.Fatalf("raw request inheritance wrong: %q", built.Intent.RawRequest)
	}
	if built.Intent.IntentClass != "delegated-child" || len(built.Intent.SuccessCriteria) != 2 {
		t.Fatalf("delegated intent wrong: %#v", built.Intent)
	}
	if !strings.Contains(built.DomainReview.ModelFit, "io-parent") ||
		len(built.DomainReview.Terminology) != 1 || len(built.DomainReview.Risks) != 1 {
		t.Fatalf("domain review inheritance wrong: %#v", built.DomainReview)
	}
	if built.PlanPrep == nil || built.PlanPrep.PriorDecisions.Status != "waived" ||
		built.PlanPrep.WebResearch.Status != "waived" {
		t.Fatalf("plan prep waiver wrong: %#v", built.PlanPrep)
	}
	if len(built.Decisions) != 1 || built.Decisions[0].Kind != "scope" {
		t.Fatalf("scope decision wrong: %#v", built.Decisions)
	}
	// 자식 issue URL: 명시값이 없으면 부모 issue URL을 상속한다.
	if built.IssueURL != "https://example.com/i/6000" {
		t.Fatalf("issue url inheritance wrong: %q", built.IssueURL)
	}
	if built.DesignReview == nil || !built.DesignReview.Approved {
		t.Fatalf("design review must be inherited as approved: %#v", built.DesignReview)
	}
	if built.CompatibilityReview == nil || !built.CompatibilityReview.Approved || built.CompatibilityReview.Blockers != nil {
		t.Fatalf("compatibility review must inherit approved and cleared blockers: %#v", built.CompatibilityReview)
	}
	if built.DevilsAdvocateReview == nil || built.DevilsAdvocateReview.Verdict != "pass" || !built.DevilsAdvocateReview.Waived {
		t.Fatalf("devils advocate waiver wrong: %#v", built.DevilsAdvocateReview)
	}

	// 명시 child URL과 부모 plan path 지정이 우선한다.
	req.ChildIssueURL = "https://example.com/i/6001"
	req.ParentPlanPath = "/repo.worktrees/6000/plan.md"
	built2 := BuildDelegatedProfile(parent, child, req, now)
	if built2.IssueURL != "https://example.com/i/6001" || built2.Delegation.ChildIssueURL != "https://example.com/i/6001" {
		t.Fatalf("explicit child issue url must win: %q", built2.IssueURL)
	}
	if built2.Delegation.ParentPlanPath != "/repo.worktrees/6000/plan.md" {
		t.Fatalf("explicit parent plan path must win: %q", built2.Delegation.ParentPlanPath)
	}
	// 부모에 intent가 없으면 scope가 원문이 된다.
	noIntent := parent
	noIntent.Intent = nil
	built3 := BuildDelegatedProfile(noIntent, child, req, now)
	if built3.Intent.RawRequest != "scope" {
		t.Fatalf("scope-only raw request wrong: %q", built3.Intent.RawRequest)
	}
}

func TestParentRefProjectsChildReference(t *testing.T) {
	child := model.IssueOpsRecord{ID: "io-child", Branch: "6001-child"}
	req := model.IssueOpsChildStartRequest{Branch: "6001-child", Title: "  child one  ", ChildIssueURL: " https://example.com/i/6001 "}
	ref := ParentRef(child, req, "now")
	if ref.CycleID != "io-child" || ref.Branch != "6001-child" || ref.Title != "child one" ||
		ref.ChildIssueURL != "https://example.com/i/6001" || ref.CreatedAt != "now" {
		t.Fatalf("parent ref wrong: %#v", ref)
	}
}
