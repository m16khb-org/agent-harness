package intentdesign

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
)

type Store struct {
	Read          func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite    func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanReadiness func(record model.IssueOpsRecord) model.IssueOpsReadiness
}

func RecordIntent(store Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	rawRequest := strings.TrimSpace(req.RawRequest)
	if rawRequest == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("raw_request is required")
	}
	interpretedIntent := strings.TrimSpace(req.InterpretedIntent)
	if interpretedIntent == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent is required")
	}
	successCriteria := CleanTextValues(req.SuccessCriteria)
	if len(successCriteria) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("success_criteria is required")
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &model.IssueOpsIntentContract{
		RawRequest:        policy.RedactFreeform(rawRequest),
		InterpretedIntent: policy.RedactFreeform(interpretedIntent),
		SuccessCriteria:   successCriteria,
		Constraints:       CleanTextValues(req.Constraints),
		Ambiguities:       CleanTextValues(req.Ambiguities),
		NonGoals:          CleanTextValues(req.NonGoals),
		RecordedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func RecordDesignReview(store Store, stateRoot, id string, req model.IssueOpsDesignReviewRequest) (model.IssueOpsRecord, error) {
	problemSummary := strings.TrimSpace(req.ProblemSummary)
	if problemSummary == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("problem_summary is required")
	}
	proposedDesign := strings.TrimSpace(req.ProposedDesign)
	if proposedDesign == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("proposed_design is required")
	}
	verification := CleanTextValues(req.Verification)
	if len(verification) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("verification is required")
	}
	openQuestions := CleanTextValues(req.OpenQuestions)
	if req.Approved && len(openQuestions) > 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review must not have open_questions")
	}
	refactorPlan := strings.TrimSpace(req.RefactorPlan)
	alternatives := CleanTextValues(req.Alternatives)
	risks := CleanTextValues(req.Risks)
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.PlanReadiness(record); !ready.Ready {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot record design review before intent contract: missing %s", strings.Join(ready.Missing, ", "))
	}
	if req.Approved && refactorPlan == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires refactor_plan")
	}
	if req.Approved && len(alternatives) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires alternatives")
	}
	if req.Approved && len(risks) == 0 {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("approved design review requires risks")
	}
	record.DesignReview = &model.IssueOpsDesignReview{
		ProblemSummary: policy.RedactFreeform(problemSummary),
		ProposedDesign: policy.RedactFreeform(proposedDesign),
		RefactorPlan:   policy.RedactFreeform(refactorPlan),
		Alternatives:   alternatives,
		Risks:          risks,
		Verification:   verification,
		OpenQuestions:  openQuestions,
		Approved:       req.Approved,
		ReviewedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func CleanTextValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
