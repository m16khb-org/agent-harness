package issueops

import (
	"agent-harness/internal/core/policy"
	"fmt"
	"strings"
	"time"
)

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	rawRequest := strings.TrimSpace(req.RawRequest)
	if rawRequest == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("raw_request is required")
	}
	interpretedIntent := strings.TrimSpace(req.InterpretedIntent)
	if interpretedIntent == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("interpreted_intent is required")
	}
	successCriteria := cleanIssueOpsTextValues(req.SuccessCriteria)
	if len(successCriteria) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("success_criteria is required")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.Intent = &IssueOpsIntentContract{
		RawRequest:        policy.RedactFreeform(rawRequest),
		InterpretedIntent: policy.RedactFreeform(interpretedIntent),
		SuccessCriteria:   successCriteria,
		Constraints:       cleanIssueOpsTextValues(req.Constraints),
		Ambiguities:       cleanIssueOpsTextValues(req.Ambiguities),
		NonGoals:          cleanIssueOpsTextValues(req.NonGoals),
		RecordedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	problemSummary := strings.TrimSpace(req.ProblemSummary)
	if problemSummary == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("problem_summary is required")
	}
	proposedDesign := strings.TrimSpace(req.ProposedDesign)
	if proposedDesign == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("proposed_design is required")
	}
	verification := cleanIssueOpsTextValues(req.Verification)
	if len(verification) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("verification is required")
	}
	openQuestions := cleanIssueOpsTextValues(req.OpenQuestions)
	if req.Approved && len(openQuestions) > 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("approved design review must not have open_questions")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := IssueOpsPlanReadiness(record); !ready.Ready {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot record design review before intent contract: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.DesignReview = &IssueOpsDesignReview{
		ProblemSummary: policy.RedactFreeform(problemSummary),
		ProposedDesign: policy.RedactFreeform(proposedDesign),
		RefactorPlan:   policy.RedactFreeform(strings.TrimSpace(req.RefactorPlan)),
		Alternatives:   cleanIssueOpsTextValues(req.Alternatives),
		Risks:          cleanIssueOpsTextValues(req.Risks),
		Verification:   verification,
		OpenQuestions:  openQuestions,
		Approved:       req.Approved,
		ReviewedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

func cleanIssueOpsTextValues(values []string) []string {
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
