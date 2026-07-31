package issueops

import (
	"strconv"
	"strings"
	"unicode"

	"agent-harness/internal/core/issueops/remote"
)

const orcaIntentMarkerPrefix = "agent-harness issueops-v1"

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

func authoritativeOrcaIssueIdentity(record IssueOpsRecord) (orcaIssueIdentity, error) {
	prepared := record.BranchPrepare
	if prepared == nil || !prepared.LinkVerified {
		return orcaIssueIdentity{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent requires verified branch issue identity",
		)
	}
	return preparedOrcaIssueIdentity(record)
}

func preparedOrcaIssueIdentity(record IssueOpsRecord) (orcaIssueIdentity, error) {
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

// GitHub의 최초 prepare만 로컬 브랜치 생성 뒤 linked branch를 붙이는 순서를
// 사용한다. resume와 GitLab은 기존 verified-link 전제를 그대로 유지한다.
func orcaPrepareIssueIdentity(record IssueOpsRecord) (orcaIssueIdentity, error) {
	issue, err := preparedOrcaIssueIdentity(record)
	if err != nil {
		return orcaIssueIdentity{}, err
	}
	if issue.Provider != "github" {
		return authoritativeOrcaIssueIdentity(record)
	}
	return issue, nil
}

func sealExternalOrcaIntentPayload(record IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	issue, err := authoritativeOrcaIssueIdentity(record)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return sealExternalOrcaIntentPayloadWithIdentity(record, payload, issue)
}

func sealExternalOrcaPrepareIntentPayload(record IssueOpsRecord, payload externalOrcaIntentPayload) (externalOrcaIntentPayload, error) {
	issue, err := orcaPrepareIssueIdentity(record)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return sealExternalOrcaIntentPayloadWithIdentity(record, payload, issue)
}

func sealExternalOrcaIntentPayloadWithIdentity(record IssueOpsRecord, payload externalOrcaIntentPayload, issue orcaIssueIdentity) (externalOrcaIntentPayload, error) {
	if record.ID != payload.LifecycleID {
		return externalOrcaIntentPayload{}, newOrcaIntentContractError(
			"intent_identity_mismatch",
			"Orca intent lifecycle does not match the verified record",
		)
	}
	payload.Probe.Provider = issue.Provider
	payload.Probe.Issue = issue.Issue
	marker, err := renderOrcaIntentMarker(orcaIntentMarkerIdentity{
		Purpose: normalizedOrcaIntentPurpose(payload), LifecycleID: payload.LifecycleID,
		Generation: payload.Generation, OperationID: payload.OperationID,
		Provider: issue.Provider, Issue: issue.Issue,
	})
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	payload.Marker = marker
	payload.Probe.Marker = payload.Marker
	if err := validateExternalOrcaIntentPayload(payload, payload.OperationID); err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return payload, nil
}

func validateOrcaIntentIssueIdentity(record IssueOpsRecord, payload externalOrcaIntentPayload) error {
	var (
		issue orcaIssueIdentity
		err   error
	)
	if normalizedOrcaIntentPurpose(payload) == orcaIntentPurposePrepare {
		issue, err = orcaPrepareIssueIdentity(record)
	} else {
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
	if err := validateOrcaMarkerIdentity(identity); err != nil {
		return "", err
	}
	fields := []string{orcaIntentMarkerPrefix}
	if identity.Purpose == orcaIntentPurposeResume {
		fields = append(fields, "resume")
	}
	fields = append(fields, "lifecycle="+identity.LifecycleID)
	if identity.Purpose == orcaIntentPurposeResume {
		fields = append(fields, "generation="+strconv.FormatUint(identity.Generation, 10))
	}
	fields = append(fields,
		"operation="+identity.OperationID,
		"provider="+identity.Provider,
		"issue="+strconv.Itoa(identity.Issue),
	)
	return strings.Join(fields, " "), nil
}

func renderOrcaReadinessMarker(lifecycleID string, issue orcaIssueIdentity) (string, error) {
	if !validOrcaMarkerToken(lifecycleID) || !validOrcaProvider(issue.Provider) || issue.Issue <= 0 {
		return "", newOrcaIntentContractError(
			"intent_marker_invalid",
			"Orca readiness marker identity is invalid",
		)
	}
	return strings.Join([]string{
		orcaIntentMarkerPrefix,
		"lifecycle=" + lifecycleID,
		"provider=" + issue.Provider,
		"issue=" + strconv.Itoa(issue.Issue),
	}, " "), nil
}

func parseOrcaIntentMarker(marker string) (orcaIntentMarkerIdentity, error) {
	fields := strings.Fields(marker)
	var identity orcaIntentMarkerIdentity
	switch {
	case len(fields) == 6 && fields[0] == "agent-harness" && fields[1] == "issueops-v1":
		identity.Purpose = orcaIntentPurposePrepare
		identity.Generation = 1
		var err error
		if identity.LifecycleID, err = orcaMarkerField(fields[2], "lifecycle"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		if identity.OperationID, err = orcaMarkerField(fields[3], "operation"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		if identity.Provider, err = orcaMarkerField(fields[4], "provider"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		rawIssue, err := orcaMarkerField(fields[5], "issue")
		if err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		identity.Issue, err = strconv.Atoi(rawIssue)
		if err != nil {
			return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
		}
	case len(fields) == 8 && fields[0] == "agent-harness" && fields[1] == "issueops-v1" && fields[2] == "resume":
		identity.Purpose = orcaIntentPurposeResume
		var err error
		if identity.LifecycleID, err = orcaMarkerField(fields[3], "lifecycle"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		rawGeneration, err := orcaMarkerField(fields[4], "generation")
		if err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		identity.Generation, err = strconv.ParseUint(rawGeneration, 10, 64)
		if err != nil {
			return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
		}
		if identity.OperationID, err = orcaMarkerField(fields[5], "operation"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		if identity.Provider, err = orcaMarkerField(fields[6], "provider"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		rawIssue, err := orcaMarkerField(fields[7], "issue")
		if err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		identity.Issue, err = strconv.Atoi(rawIssue)
		if err != nil {
			return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
		}
	default:
		return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
	}
	rendered, err := renderOrcaIntentMarker(identity)
	if err != nil || rendered != marker {
		return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
	}
	return identity, nil
}

func parseLegacyOrcaIntentMarker(marker string) (orcaIntentMarkerIdentity, error) {
	fields := strings.Fields(marker)
	var identity orcaIntentMarkerIdentity
	switch {
	case len(fields) == 4 && fields[0] == "agent-harness" && fields[1] == "issueops-v1":
		identity.Purpose = orcaIntentPurposePrepare
		identity.Generation = 1
		var err error
		if identity.LifecycleID, err = orcaMarkerField(fields[2], "lifecycle"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		if identity.OperationID, err = orcaMarkerField(fields[3], "operation"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
	case len(fields) == 6 && fields[0] == "agent-harness" && fields[1] == "issueops-v1" && fields[2] == "resume":
		identity.Purpose = orcaIntentPurposeResume
		var err error
		if identity.LifecycleID, err = orcaMarkerField(fields[3], "lifecycle"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		rawGeneration, err := orcaMarkerField(fields[4], "generation")
		if err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
		identity.Generation, err = strconv.ParseUint(rawGeneration, 10, 64)
		if err != nil {
			return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
		}
		if identity.OperationID, err = orcaMarkerField(fields[5], "operation"); err != nil {
			return orcaIntentMarkerIdentity{}, err
		}
	default:
		return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
	}
	rendered, err := renderLegacyOrcaIntentMarker(identity)
	if err != nil || rendered != marker {
		return orcaIntentMarkerIdentity{}, invalidOrcaMarker()
	}
	return identity, nil
}

func renderLegacyOrcaIntentMarker(identity orcaIntentMarkerIdentity) (string, error) {
	if err := validateOrcaMarkerCoreIdentity(identity); err != nil {
		return "", err
	}
	fields := []string{orcaIntentMarkerPrefix}
	if identity.Purpose == orcaIntentPurposeResume {
		fields = append(fields, "resume")
	}
	fields = append(fields, "lifecycle="+identity.LifecycleID)
	if identity.Purpose == orcaIntentPurposeResume {
		fields = append(fields, "generation="+strconv.FormatUint(identity.Generation, 10))
	}
	fields = append(fields, "operation="+identity.OperationID)
	return strings.Join(fields, " "), nil
}

func validateOrcaMarkerIdentity(identity orcaIntentMarkerIdentity) error {
	if err := validateOrcaMarkerCoreIdentity(identity); err != nil {
		return err
	}
	if !validOrcaProvider(identity.Provider) || identity.Issue <= 0 {
		return invalidOrcaMarker()
	}
	return nil
}

func validateOrcaMarkerCoreIdentity(identity orcaIntentMarkerIdentity) error {
	if identity.Purpose != orcaIntentPurposePrepare && identity.Purpose != orcaIntentPurposeResume {
		return invalidOrcaMarker()
	}
	if !validOrcaMarkerToken(identity.LifecycleID) || !validOrcaMarkerToken(identity.OperationID) {
		return invalidOrcaMarker()
	}
	if identity.Purpose == orcaIntentPurposePrepare && identity.Generation != 1 ||
		identity.Purpose == orcaIntentPurposeResume && identity.Generation == 0 {
		return invalidOrcaMarker()
	}
	return nil
}

func validOrcaProvider(provider string) bool {
	return provider == "github" || provider == "gitlab"
}

func validOrcaMarkerToken(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsRune(value, '=') {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsSpace)
}

func orcaMarkerField(field, name string) (string, error) {
	value, ok := strings.CutPrefix(field, name+"=")
	if !ok || !validOrcaMarkerToken(value) {
		return "", invalidOrcaMarker()
	}
	return value, nil
}

func invalidOrcaMarker() error {
	return newOrcaIntentContractError("intent_marker_invalid", "Orca intent marker is not canonical")
}
