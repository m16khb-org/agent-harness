package issueops

import (
	"fmt"
	"strings"
	"time"
)

func VerifyIssueOpsRemoteArtifact(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	artifact, err := issueOpsRemoteArtifactVerificationFromRequest(record, req)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	record.RemoteArtifact = &artifact
	return touchAndWriteIssueOps(stateRoot, record)
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	_, err = issueOpsRemoteArtifactVerificationFromRequest(record, req)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

func issueOpsRemoteArtifactVerificationFromRequest(record IssueOpsRecord, req IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRemoteArtifactVerification, error) {
	if record.Phase != IssueOpsPhasePR {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("cannot verify remote artifact before pr phase")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "github" && provider != "gitlab" {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if issueProvider := issueOpsProviderFromURL(record.IssueURL); issueProvider != "" && provider != issueProvider {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact provider must match linked issue provider")
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	if kind != "pr" && kind != "mr" {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact kind must be pr or mr")
	}
	if provider == "github" && kind != "pr" {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("github remote artifact kind must be pr")
	}
	if provider == "gitlab" && kind != "mr" {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("gitlab remote artifact kind must be mr")
	}
	artifactURL := strings.TrimSpace(req.URL)
	if err := validateRemoteArtifactURL(artifactURL, provider, kind); err != nil {
		return IssueOpsRemoteArtifactVerification{}, err
	}
	if err := validateRemoteArtifactMatchesIssue(record.IssueURL, artifactURL, provider, kind); err != nil {
		return IssueOpsRemoteArtifactVerification{}, err
	}
	labels := cleanIssueOpsRemoteValues(req.Labels)
	if len(labels) == 0 {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact labels are required")
	}
	assignees := cleanIssueOpsRemoteValues(req.Assignees)
	if len(assignees) == 0 {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact assignees are required")
	}
	if invalid := invalidIssueOpsRemoteAssignee(assignees); invalid != "" {
		return IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact assignee must be a verified provider user, not placeholder %q", invalid)
	}
	return IssueOpsRemoteArtifactVerification{
		Provider:   provider,
		Kind:       kind,
		URL:        artifactURL,
		Labels:     labels,
		Assignees:  assignees,
		VerifiedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}
