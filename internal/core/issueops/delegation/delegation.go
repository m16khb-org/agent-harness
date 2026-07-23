package delegation

import (
	"strings"

	"agent-harness/internal/core/issueops/model"
)

func MissingPreconditions(parent model.IssueOpsRecord, req model.IssueOpsChildStartRequest) []string {
	var missing []string
	if parent.Phase != model.IssueOpsPhaseImplement {
		missing = append(missing, "parent_phase_not_implement")
	}
	if parent.DesignReview == nil || !parent.DesignReview.Approved {
		missing = append(missing, "parent_design_review_unapproved")
	}
	if parent.CompatibilityReview == nil || !parent.CompatibilityReview.Approved || len(clean(parent.CompatibilityReview.Blockers)) > 0 {
		missing = append(missing, "parent_compatibility_unapproved")
	}
	if parent.DevilsAdvocateReview == nil || strings.TrimSpace(parent.DevilsAdvocateReview.RecordedAt) == "" || ((parent.DevilsAdvocateReview.Verdict == "stop" || parent.DevilsAdvocateReview.Verdict == "revise") && !parent.DevilsAdvocateReview.Waived) {
		missing = append(missing, "parent_devils_advocate_missing")
	}
	if parent.Delegation != nil {
		missing = append(missing, "delegation_depth_exceeded")
	}
	if strings.TrimSpace(req.Branch) == strings.TrimSpace(parent.Branch) {
		missing = append(missing, "child_branch_equals_parent")
	}
	return missing
}

func BuildDelegatedProfile(parent, child model.IssueOpsRecord, req model.IssueOpsChildStartRequest, now string) model.IssueOpsRecord {
	taskScope := strings.TrimSpace(req.TaskScope)
	acceptance := clean(req.AcceptanceCriteria)
	parentPlanPath := strings.TrimSpace(req.ParentPlanPath)
	if parentPlanPath == "" {
		parentPlanPath = strings.TrimSpace(parent.PlanPath)
	}
	childIssueURL := strings.TrimSpace(req.ChildIssueURL)
	if childIssueURL == "" {
		childIssueURL = strings.TrimSpace(parent.IssueURL)
	}

	child.Delegation = &model.IssueOpsDelegationContract{
		ParentCycleID:      parent.ID,
		TaskScope:          taskScope,
		AcceptanceCriteria: acceptance,
		ParentPlanPath:     parentPlanPath,
		ChildIssueURL:      strings.TrimSpace(req.ChildIssueURL),
		DelegatedAt:        now,
	}
	rawRequest := taskScope
	if parent.Intent != nil && strings.TrimSpace(parent.Intent.RawRequest) != "" {
		rawRequest = strings.TrimSpace(parent.Intent.RawRequest) + "\n\nDelegated task: " + taskScope
	}
	child.Intent = &model.IssueOpsIntentContract{
		RawRequest:        rawRequest,
		InterpretedIntent: taskScope,
		SuccessCriteria:   acceptance,
		IntentClass:       "delegated-child",
		RecordedAt:        now,
	}
	child.DomainReview = &model.IssueOpsDomainReview{
		ModelFit:   "delegated: inherits parent " + parent.ID + " domain review",
		ReviewedAt: now,
	}
	if parent.DomainReview != nil {
		child.DomainReview.Terminology = append([]string{}, parent.DomainReview.Terminology...)
		child.DomainReview.Risks = append([]string{}, parent.DomainReview.Risks...)
		child.DomainReview.OpenUncertainties = append([]string{}, parent.DomainReview.OpenUncertainties...)
	}
	waived := model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "delegated:" + parent.ID}
	child.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
		CodebaseSurvey: waived,
		RecordedAt:     now,
	}
	child.Decisions = []model.IssueOpsDecision{{
		Title:     "delegated child of " + parent.ID,
		Body:      taskScope,
		Kind:      "scope",
		Rationale: "delegated child of " + parent.ID,
		CreatedAt: now,
	}}
	child.IssueURL = childIssueURL
	if parent.DesignReview != nil {
		dr := *parent.DesignReview
		dr.Approved = true
		dr.ReviewedAt = now
		child.DesignReview = &dr
	}
	if parent.CompatibilityReview != nil {
		cr := *parent.CompatibilityReview
		cr.Approved = true
		cr.Blockers = nil
		cr.ReviewedAt = now
		child.CompatibilityReview = &cr
	}
	child.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{
		Verdict:          "pass",
		Waived:           true,
		WaiverRationale:  "delegated:" + parent.ID + " parent DA verdict pass",
		ReviewerPattern:  "delegated-parent-review",
		RecordedAt:       now,
		IssueReflectedAt: "",
	}
	return child
}

func ParentRef(child model.IssueOpsRecord, req model.IssueOpsChildStartRequest, now string) model.IssueOpsChildCycleRef {
	return model.IssueOpsChildCycleRef{
		CycleID:       child.ID,
		Branch:        child.Branch,
		Title:         strings.TrimSpace(req.Title),
		ChildIssueURL: strings.TrimSpace(req.ChildIssueURL),
		CreatedAt:     now,
	}
}

func clean(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
