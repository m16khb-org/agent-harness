package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/contract/issueops"
)

func AddIssueOpsFeedback(stateRoot, id, source, body, classification string) (issueops.IssueOpsRecord, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, nil)
}

func AddIssueOpsFeedbackWithActor(stateRoot, id, source, body, classification string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return addIssueOpsFeedback(stateRoot, id, source, body, classification, &actor)
}

func addIssueOpsFeedback(stateRoot, id, source, body, classification string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := validatePostTransferMutation(record, actor); err != nil {
			return err
		}
		var e error
		rec, e = addIssueOpsFeedbackLocked(stateRoot, id, source, body, classification)
		return e
	})
	return rec, err
}

func addIssueOpsFeedbackLocked(stateRoot, id, source, body, classification string) (issueops.IssueOpsRecord, error) {
	source = strings.TrimSpace(source)
	body = strings.TrimSpace(body)
	classification = strings.ToLower(strings.TrimSpace(classification))
	if source == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("feedback source is required")
	}
	if body == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("feedback body is required")
	}
	if !knownIssueOpsFeedbackClassification(classification) {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops feedback classification %q; use contract_change, defect, question, noise, valid_review, stale_review, rollout_evidence_missing, or environment_debt", classification)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == IssueOpsPhaseDone {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("cannot add feedback after %s phase", record.Phase)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback = append(record.Feedback, issueops.IssueOpsFeedbackItem{Source: source, Body: body, Classification: classification, CreatedAt: now})
	if strings.TrimSpace(record.AISlopCleanAt) != "" {
		record.Phase = IssueOpsPhaseFeedback
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func knownIssueOpsFeedbackClassification(classification string) bool {
	return issueops.KnownFeedbackClassification(classification)
}

func MarkIssueOpsContractFeedbackIssueUpdated(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	return markIssueOpsContractFeedbackIssueUpdated(stateRoot, id, nil)
}

func MarkIssueOpsContractFeedbackIssueUpdatedWithActor(stateRoot, id string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return markIssueOpsContractFeedbackIssueUpdated(stateRoot, id, &actor)
}

func markIssueOpsContractFeedbackIssueUpdated(stateRoot, id string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := validatePostTransferMutation(record, actor); err != nil {
			return err
		}
		var e error
		rec, e = markIssueOpsContractFeedbackIssueUpdatedLocked(stateRoot, id)
		return e
	})
	return rec, err
}

func markIssueOpsContractFeedbackIssueUpdatedLocked(stateRoot, id string) (issueops.IssueOpsRecord, error) {
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
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("no unresolved contract_change feedback requires a remote issue update")
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}
