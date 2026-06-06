package issueops

import (
	"fmt"
	"strings"
	"time"
)

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (IssueOpsRecord, error) {
	source = strings.TrimSpace(source)
	body = strings.TrimSpace(body)
	classification = strings.ToLower(strings.TrimSpace(classification))
	if source == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback source is required")
	}
	if body == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback body is required")
	}
	if !knownIssueOpsFeedbackClassification(classification) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops feedback classification %q; use contract_change, defect, question, noise, valid_review, stale_review, rollout_evidence_missing, or environment_debt", classification)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot add feedback after %s phase", record.Phase)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback = append(record.Feedback, IssueOpsFeedbackItem{Source: source, Body: body, Classification: classification, CreatedAt: now})
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		record.Phase = IssueOpsPhaseFeedback
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func knownIssueOpsFeedbackClassification(classification string) bool {
	switch classification {
	case "", "contract_change", "defect", "question", "noise", "valid_review", "stale_review", "rollout_evidence_missing", "environment_debt":
		return true
	default:
		return false
	}
}

func MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	marked := false
	for i := range record.Feedback {
		if issueOpsFeedbackRequiresIssueUpdate(record.Feedback[i]) {
			record.Feedback[i].IssueUpdatedAt = now
			marked = true
		}
	}
	if !marked {
		return IssueOpsRecord{OK: false}, fmt.Errorf("no unresolved contract_change feedback requires a remote issue update")
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}
