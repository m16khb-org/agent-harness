package issueops

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/port"
	"context"
)

// ReflectDevilsAdvocateFindings writes the recorded devil's-advocate findings
// into the linked remote issue's managed body section through the supplied
// provider. On a confirmed successful update it stamps IssueReflectedAt, which
// the regress precondition requires so a stop's findings reach the issue before
// the cycle re-plans. Without confirm it returns the provider's dry-run preview
// and does not mutate state.
func ReflectDevilsAdvocateFindings(stateRoot, id string, confirm bool, prov port.IssueProvider) (IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	if prov == nil {
		return IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, port.IssueProviderUpdateIssueBodySectionResult{}, err
	}
	review := record.DevilsAdvocateReview
	if review == nil || len(review.Findings) == 0 {
		return IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("no devil's-advocate findings to reflect")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("cannot reflect findings before a linked issue")
	}
	result, err := prov.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:     record.Repo,
		IssueURL: record.IssueURL,
		Findings: review.Findings,
		Confirm:  confirm,
	})
	if err != nil {
		return IssueOpsRecord{OK: false}, result, err
	}
	if !confirm || !result.Updated {
		return record, result, nil
	}
	lockErr := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if rec.DevilsAdvocateReview == nil {
			return fmt.Errorf("devil's-advocate review disappeared before reflect stamp")
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.DevilsAdvocateReview.IssueReflectedAt = now
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if lockErr != nil {
		return IssueOpsRecord{OK: false}, result, lockErr
	}
	return record, result, nil
}
