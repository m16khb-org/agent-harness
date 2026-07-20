package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"context"
)

// validIssueOpsFeedbackResolutions are the recognised feedback resolution
// outcomes recorded for the phase-ledger feedback_resolution artifact.
var validIssueOpsFeedbackResolutions = map[string]bool{
	"valid-defect":      true,
	"question-answered": true,
	"noise-dismissed":   true,
}

// RecordIssueOpsDomainReview persists the grill-phase domain review
// (terminology, model fit, risks, uncertainties) — the source of truth backing
// the grill domain_review artifact.
func RecordIssueOpsDomainReview(stateRoot, id string, req IssueOpsDomainReviewRequest) (IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDomainReviewWithActor(stateRoot, id string, req IssueOpsDomainReviewRequest, actor IssueOpsActor) (IssueOpsRecord, error) {
	return recordIssueOpsDomainReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDomainReview(stateRoot, id string, req IssueOpsDomainReviewRequest, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = recordIssueOpsDomainReviewLocked(stateRoot, id, req)
		return e
	})
	return rec, err
}

func recordIssueOpsDomainReviewLocked(stateRoot, id string, req IssueOpsDomainReviewRequest) (IssueOpsRecord, error) {
	modelFit := strings.TrimSpace(req.ModelFit)
	terminology := cleanIssueOpsTextValues(req.Terminology)
	if modelFit == "" && len(terminology) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("domain review requires model_fit or terminology")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.DomainReview = &model.IssueOpsDomainReview{
		Terminology:       terminology,
		ModelFit:          modelFit,
		Risks:             cleanIssueOpsTextValues(req.Risks),
		OpenUncertainties: cleanIssueOpsTextValues(req.OpenUncertainties),
		ReviewedAt:        now,
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

// RecordIssueOpsAISlopCleanEvidence persists which cleanup categories were
// checked/cleaned and which verifications were rerun — the source of truth
// backing the ai-slop-clean cleanup_evidence and verification_evidence artifacts.
func RecordIssueOpsAISlopCleanEvidence(stateRoot, id string, categories, verification []string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = recordIssueOpsAISlopCleanEvidenceLocked(stateRoot, id, categories, verification)
		return e
	})
	return rec, err
}

func recordIssueOpsAISlopCleanEvidenceLocked(stateRoot, id string, categories, verification []string) (IssueOpsRecord, error) {
	cats := cleanIssueOpsTextValues(categories)
	ver := cleanIssueOpsTextValues(verification)
	if len(cats) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("ai-slop-clean evidence requires at least one cleanup category")
	}
	if len(ver) == 0 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("ai-slop-clean evidence requires at least one verification entry")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.AISlopCleanCategories = cats
	record.AISlopCleanVerification = ver
	if strings.TrimSpace(record.AISlopCleanAt) != "" && issueOpsPhaseRank(record.Phase) >= issueOpsPhaseRank(IssueOpsPhaseAISlopClean) {
		return refreshIssueOpsAISlopClean(stateRoot, record)
	}
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

// ResolveIssueOpsFeedback records the outcome of a feedback item by index — the
// source of truth backing the feedback feedback_resolution artifact.
func ResolveIssueOpsFeedback(stateRoot, id string, index int, resolution string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = resolveIssueOpsFeedbackLocked(stateRoot, id, index, resolution)
		return e
	})
	return rec, err
}

func resolveIssueOpsFeedbackLocked(stateRoot, id string, index int, resolution string) (IssueOpsRecord, error) {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if !validIssueOpsFeedbackResolutions[resolution] {
		return IssueOpsRecord{OK: false}, fmt.Errorf("unknown feedback resolution %q; use valid-defect, question-answered, or noise-dismissed", resolution)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if index < 0 || index >= len(record.Feedback) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback index %d out of range (have %d items)", index, len(record.Feedback))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback[index].Resolution = resolution
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}
