package intentdesign

import (
	"fmt"
	"strings"
	"time"

	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/policy"
)

// RecordPlanPrep stores the pre-plan evidence gate: prior-decision lookup,
// related-issue scoring, web research, and the codebase survey. Each item must
// carry either evidence or a waive reason (mutually exclusive). The plan
// readiness gate then checks these for non-trivial intent classes.
func RecordPlanPrep(store Store, stateRoot, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
	decisions, err := buildPlanPrepItem("decisions", req.PriorDecisions)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	related, err := buildPlanPrepItem("related_issues", req.RelatedIssues)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	research, err := buildPlanPrepItem("web_research", req.WebResearch)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	survey, err := buildPlanPrepItem("codebase_survey", req.CodebaseSurvey)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: decisions,
		RelatedIssues:  related,
		WebResearch:    research,
		CodebaseSurvey: survey,
		RecordedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func buildPlanPrepItem(name string, req model.IssueOpsPlanPrepItemRequest) (model.IssueOpsPlanPrepItem, error) {
	evidence := CleanTextValues(req.Evidence)
	waive := strings.TrimSpace(req.WaiveReason)
	if len(evidence) > 0 && waive != "" {
		return model.IssueOpsPlanPrepItem{}, fmt.Errorf("plan_prep %s: evidence and waive_reason are mutually exclusive", name)
	}
	if len(evidence) == 0 && waive == "" {
		return model.IssueOpsPlanPrepItem{}, fmt.Errorf("plan_prep %s: provide evidence or a waive reason", name)
	}
	if waive != "" {
		return model.IssueOpsPlanPrepItem{Status: "waived", WaiveReason: policy.RedactFreeform(waive)}, nil
	}
	return model.IssueOpsPlanPrepItem{Status: "evidence", Evidence: evidence}, nil
}
