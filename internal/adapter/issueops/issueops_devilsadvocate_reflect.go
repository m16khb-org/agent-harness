package issueops

import (
	"fmt"
	"strings"
	"time"

	"context"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// ReflectDevilsAdvocateFindings writes the recorded devil's-advocate findings
// into the linked remote issue's managed body section through the supplied
// provider. On a confirmed successful update it stamps IssueReflectedAt, which
// the regress precondition requires so a stop's findings reach the issue before
// the cycle re-plans. Without confirm it returns the provider's dry-run preview
// and does not mutate state.
func ReflectDevilsAdvocateFindings(stateRoot, id string, confirm bool, prov port.IssueProvider) (issueops.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	return reflectDevilsAdvocateFindings(stateRoot, id, confirm, prov, nil)
}

func ReflectDevilsAdvocateFindingsWithActor(stateRoot, id string, confirm bool, prov port.IssueProvider, actor IssueOpsActor) (issueops.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	return reflectDevilsAdvocateFindings(stateRoot, id, confirm, prov, &actor)
}

func reflectDevilsAdvocateFindings(stateRoot, id string, confirm bool, prov port.IssueProvider, actor *IssueOpsActor) (issueops.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("no issue provider configured")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, port.IssueProviderUpdateIssueBodySectionResult{}, err
	}
	if err := validateExecutionMutation(record, actor); err != nil {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, err
	}
	review := record.DevilsAdvocateReview
	if review == nil || len(review.Findings) == 0 {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("no devil's-advocate findings to reflect")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return issueops.IssueOpsRecord{OK: false}, port.IssueProviderUpdateIssueBodySectionResult{}, fmt.Errorf("cannot reflect findings before a linked issue")
	}
	result, err := prov.UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{
		Repo:     record.Repo,
		IssueURL: record.IssueURL,
		Section:  port.IssueBodySectionDevilsAdvocate,
		Findings: review.Findings,
		Confirm:  confirm,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
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
		if err := validateExecutionMutation(rec, actor); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.DevilsAdvocateReview.IssueReflectedAt = now
		rec.UpdatedAt = now
		record, e = writeIssueOps(stateRoot, rec)
		return e
	})
	if lockErr != nil {
		return issueops.IssueOpsRecord{OK: false}, result, lockErr
	}
	return record, result, nil
}

// issueOpsLinkedPlanDigest는 링크된 플랜 파일의 sha256이다. owner preflight와
// 같은 resolver(readLinkedPlanIdentity)를 써서 한 플랜에 digest가 둘이 되지 않게 한다.
func issueOpsLinkedPlanDigest(record issueops.IssueOpsRecord) (string, error) {
	identity, err := readLinkedPlanIdentity(record)
	if err != nil {
		return "", err
	}
	return identity.Digest, nil
}

// issueOpsReviewedPlanDigest는 devil's-advocate 판정을 묶을 플랜 digest다. 링크된
// 플랜 파일이 우선이고, 파일 없이 staged plan artifact만 있는 사이클(fresh staged
// plan)은 그 artifact에 묶는다. 둘 다 없으면 검토할 플랜이 없으므로 기록을 거부한다.
func issueOpsReviewedPlanDigest(stateRoot string, record issueops.IssueOpsRecord) (string, error) {
	if strings.TrimSpace(record.PlanPath) != "" {
		return issueOpsLinkedPlanDigest(record)
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err == nil {
		if plan, ok := staged["plan"]; ok && strings.TrimSpace(plan) != "" {
			return digestExecutionOwnerBytes([]byte(plan)), nil
		}
	}
	return "", fmt.Errorf("link the plan (issueops link-plan) or stage it (issueops artifact stage --name plan) before recording the devil's-advocate review")
}
