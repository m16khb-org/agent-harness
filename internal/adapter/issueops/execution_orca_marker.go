package issueops

import (
	"strconv"
	"strings"

	"agent-harness/internal/adapter/issueops/remote"
	"agent-harness/internal/contract/issueops"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

type orcaIssueIdentity struct {
	Provider string
	Issue    int
}

type orcaIntentMarkerIdentity struct {
	Purpose     string
	LifecycleID string
	Generation  uint64
	OperationID string
	Provider    string
	Issue       int
}

type orcaIntentContractError struct {
	Code   string
	Detail string
}

func (e *orcaIntentContractError) Error() string {
	if strings.TrimSpace(e.Detail) == "" {
		return e.Code
	}
	return e.Code + ": " + strings.TrimSpace(e.Detail)
}

func (e *orcaIntentContractError) IssueOpsErrorFields() map[string]any {
	return map[string]any{"code": e.Code}
}

func newOrcaIntentContractError(code, detail string) error {
	return &orcaIntentContractError{Code: code, Detail: detail}
}

func authoritativeOrcaIssueIdentity(record issueops.IssueOpsRecord) (orcaIssueIdentity, error) {
	prepared := record.BranchPrepare
	if prepared == nil || !prepared.LinkVerified {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent requires verified branch issue identity",
		)
	}
	return preparedOrcaIssueIdentity(record)
}

func preparedOrcaIssueIdentity(record issueops.IssueOpsRecord) (orcaIssueIdentity, error) {
	prepared := record.BranchPrepare
	if prepared == nil {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent requires prepared branch issue identity",
		)
	}
	provider := strings.ToLower(strings.TrimSpace(prepared.Provider))
	if provider != "github" && provider != "gitlab" {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent provider must be github or gitlab",
		)
	}
	issueURL := strings.TrimSpace(prepared.IssueURL)
	if issueURL == "" || strings.TrimSpace(record.IssueURL) != issueURL {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent issue URL does not match the verified branch identity",
		)
	}
	if detected := remote.ProviderFromURL(issueURL); detected != provider {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent provider does not match the verified issue URL",
		)
	}
	rawIssue := remote.IssueNumber(issueURL)
	issue, err := strconv.Atoi(rawIssue)
	if err != nil || issue <= 0 {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent requires a positive issue number",
		)
	}
	return orcaIssueIdentity{Provider: provider, Issue: issue}, nil
}

// GitHub owner launch는 로컬 브랜치를 만든 뒤 linked branch를 검증하는 순서를
// 사용한다. prepare와 resume는 owner prompt의 post-claim 검증에 도달해야 하므로
// prepared identity를 허용하고, GitLab launch는 verified-link 전제를 유지한다.
func orcaLaunchIssueIdentity(record issueops.IssueOpsRecord) (orcaIssueIdentity, error) {
	issue, err := preparedOrcaIssueIdentity(record)
	if err != nil {
		return orcaIssueIdentity{}, err
	}
	if issue.Provider != "github" {
		return authoritativeOrcaIssueIdentity(record)
	}
	return issue, nil
}

func sealExternalOrcaIntentPayload(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	issue, err := authoritativeOrcaIssueIdentity(record)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return sealExternalOrcaIntentPayloadWithIdentity(record, payload, issue)
}

func sealExternalOrcaPrepareIntentPayload(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	return sealExternalOrcaLaunchIntentPayload(record, payload)
}

func sealExternalOrcaResumeIntentPayload(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	return sealExternalOrcaLaunchIntentPayload(record, payload)
}

func sealExternalOrcaLaunchIntentPayload(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	issue, err := orcaLaunchIssueIdentity(record)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return sealExternalOrcaIntentPayloadWithIdentity(record, payload, issue)
}

func sealExternalOrcaIntentPayloadWithIdentity(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload, issue orcaIssueIdentity) (externalOrcaIntentPayload, error) {
	if record.ID != payload.LifecycleID {
		return externalOrcaIntentPayload{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent lifecycle does not match the verified record",
		)
	}
	return preparationIntentCodec.Seal(payload, preparationcontract.IssueIdentity{Provider: issue.Provider, Issue: issue.Issue})
}

func validateOrcaIntentIssueIdentity(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) error {
	var (
		issue orcaIssueIdentity
		err   error
	)
	switch normalizedOrcaIntentPurpose(payload) {
	case orcaIntentPurposePrepare, orcaIntentPurposeResume:
		issue, err = orcaLaunchIssueIdentity(record)
	default:
		issue, err = authoritativeOrcaIssueIdentity(record)
	}
	if err != nil {
		return err
	}
	if record.ID != payload.LifecycleID ||
		payload.Probe.Provider != issue.Provider || payload.Probe.Issue != issue.Issue {
		return newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent issue identity changed before persistence",
		)
	}
	return nil
}

func renderOrcaIntentMarker(identity orcaIntentMarkerIdentity) (string, error) {
	return preparationIntentCodec.RenderMarker(preparationcontract.MarkerIdentity{
		Purpose: identity.Purpose, LifecycleID: identity.LifecycleID, Generation: identity.Generation,
		OperationID: identity.OperationID, Provider: identity.Provider, Issue: identity.Issue,
	})
}

func parseOrcaIntentMarker(marker string) (orcaIntentMarkerIdentity, error) {
	identity, err := preparationIntentCodec.ParseMarker(marker)
	if err != nil {
		return orcaIntentMarkerIdentity{}, err
	}
	return orcaIntentMarkerIdentity{
		Purpose: identity.Purpose, LifecycleID: identity.LifecycleID, Generation: identity.Generation,
		OperationID: identity.OperationID, Provider: identity.Provider, Issue: identity.Issue,
	}, nil
}
