package issueops

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"agent-harness/internal/adapter/issueops/linking"
	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
)

func BeginIssueCreateIntent(
	stateRoot string,
	id string,
	request issueopscontract.IssueOpsIssueCreateIntentRequest,
) (issueopscontract.IssueOpsRecord, error) {
	var updated issueopscontract.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(record.IssueURL) != "" {
			return fmt.Errorf("issueops record already links issue %s", record.IssueURL)
		}
		if err := validateIssueCreateIntentRequest(request); err != nil {
			return err
		}
		if record.IssueCreateIntent != nil {
			intent := record.IssueCreateIntent
			if intent.Status != issueopscontract.IssueCreateIntentNotInvoked {
				return fmt.Errorf("issue create intent %s is %s; reconcile before retry", intent.OperationID, intent.Status)
			}
			if !sameIssueCreateRequest(*intent, request) {
				return fmt.Errorf("retry request does not match sealed issue create intent")
			}
			intent.Status = issueopscontract.IssueCreateIntentPending
			intent.Attempt++
			intent.Failure = ""
			intent.UpdatedAt = request.StartedAt
			updated, err = touchAndWriteIssueOps(stateRoot, record)
			return err
		}
		record.IssueCreateIntent = &issueopscontract.IssueOpsIssueCreateIntent{
			OperationID:      strings.TrimSpace(request.OperationID),
			Marker:           "<!-- agent-harness:issue-create:" + strings.TrimSpace(request.OperationID) + " -->",
			Provider:         strings.TrimSpace(request.Provider),
			ProjectAuthority: strings.TrimSpace(request.ProjectAuthority),
			Title:            strings.TrimSpace(request.Title),
			BodySHA256:       strings.ToLower(strings.TrimSpace(request.BodySHA256)),
			Labels:           append([]string(nil), request.Labels...),
			Assignees:        append([]string(nil), request.Assignees...),
			Status:           issueopscontract.IssueCreateIntentPending,
			Attempt:          1,
			StartedAt:        strings.TrimSpace(request.StartedAt),
			UpdatedAt:        strings.TrimSpace(request.StartedAt),
		}
		updated, err = touchAndWriteIssueOps(stateRoot, record)
		return err
	})
	return updated, err
}

func RecordIssueCreateOutcome(
	stateRoot string,
	id string,
	outcome issueopscontract.IssueOpsIssueCreateOutcome,
) (issueopscontract.IssueOpsRecord, error) {
	var updated issueopscontract.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.IssueCreateIntent == nil {
			return fmt.Errorf("no pending issue create intent")
		}
		if record.IssueCreateIntent.Status == issueopscontract.IssueCreateIntentCompleted {
			return fmt.Errorf("issue create intent is already completed")
		}
		if err := issueopscontract.ValidateIssueCreateTransition(record.IssueCreateIntent.Status, outcome.Status); err != nil {
			return err
		}
		next := *record.IssueCreateIntent
		next.Status = outcome.Status
		next.CanonicalURL = strings.TrimSpace(outcome.CanonicalURL)
		next.Failure = strings.TrimSpace(outcome.Failure)
		next.UpdatedAt = strings.TrimSpace(outcome.ObservedAt)
		if next.CanonicalURL != "" {
			projectAuthority := issueopsremote.ProjectKey(next.CanonicalURL, next.Provider, "issue")
			if projectAuthority == "" || projectAuthority != next.ProjectAuthority {
				return fmt.Errorf("observed issue project authority %q does not match sealed authority %q", projectAuthority, next.ProjectAuthority)
			}
		}
		if err := issueopscontract.ValidateIssueCreateIntent(next); err != nil {
			return err
		}
		record.IssueCreateIntent = &next
		updated, err = touchAndWriteIssueOps(stateRoot, record)
		return err
	})
	return updated, err
}

func CompleteIssueCreateIntent(
	stateRoot string,
	id string,
	issueURL string,
	completedAt string,
) (issueopscontract.IssueOpsRecord, error) {
	var updated issueopscontract.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.IssueCreateIntent == nil {
			return fmt.Errorf("no pending issue create intent")
		}
		if record.IssueCreateIntent.Status == issueopscontract.IssueCreateIntentNotInvoked {
			return fmt.Errorf("issue create intent was not invoked; begin a retry before completion")
		}
		if record.IssueCreateIntent.Status == issueopscontract.IssueCreateIntentCompleted {
			return fmt.Errorf("issue create intent is already completed")
		}
		if err := issueopscontract.ValidateIssueCreateTransition(record.IssueCreateIntent.Status, issueopscontract.IssueCreateIntentCompleted); err != nil {
			return err
		}
		canonicalURL := strings.TrimSpace(issueURL)
		if err := linking.ValidateIssueURL(canonicalURL); err != nil {
			return err
		}
		projectAuthority := issueopsremote.ProjectKey(canonicalURL, record.IssueCreateIntent.Provider, "issue")
		if projectAuthority == "" || projectAuthority != record.IssueCreateIntent.ProjectAuthority {
			return fmt.Errorf(
				"created issue project authority %q does not match sealed authority %q",
				projectAuthority,
				record.IssueCreateIntent.ProjectAuthority,
			)
		}
		record.IssueURL = canonicalURL
		record.IssueCreateIntent.Status = issueopscontract.IssueCreateIntentCompleted
		record.IssueCreateIntent.CanonicalURL = canonicalURL
		record.IssueCreateIntent.Failure = ""
		record.IssueCreateIntent.UpdatedAt = strings.TrimSpace(completedAt)
		if ready := IssueOpsPlanReadiness(record); ready.Ready && issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(issueopscontract.IssueOpsPhasePlan) {
			record.Phase = issueopscontract.IssueOpsPhasePlan
		}
		updated, err = touchAndWriteIssueOps(stateRoot, record)
		return err
	})
	return updated, err
}

func validateIssueCreateIntentRequest(request issueopscontract.IssueOpsIssueCreateIntentRequest) error {
	for field, value := range map[string]string{
		"operation_id":      request.OperationID,
		"provider":          request.Provider,
		"project_authority": request.ProjectAuthority,
		"title":             request.Title,
		"body_sha256":       request.BodySHA256,
		"started_at":        request.StartedAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	if len(strings.TrimSpace(request.BodySHA256)) != 64 {
		return fmt.Errorf("body_sha256 must be 64 hex characters")
	}
	return nil
}

func sameIssueCreateRequest(
	intent issueopscontract.IssueOpsIssueCreateIntent,
	request issueopscontract.IssueOpsIssueCreateIntentRequest,
) bool {
	return intent.OperationID == strings.TrimSpace(request.OperationID) &&
		intent.Provider == strings.TrimSpace(request.Provider) &&
		intent.ProjectAuthority == strings.TrimSpace(request.ProjectAuthority) &&
		intent.Title == strings.TrimSpace(request.Title) &&
		intent.BodySHA256 == strings.ToLower(strings.TrimSpace(request.BodySHA256)) &&
		slices.Equal(intent.Labels, request.Labels) &&
		slices.Equal(intent.Assignees, request.Assignees)
}
