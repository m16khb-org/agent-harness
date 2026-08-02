package artifactverify

import (
	"fmt"
	"strings"
	"time"

	model "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops/remote"
)

type Store struct {
	Read       func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

func Verify(store Store, stateRoot, id string, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRecord, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	artifact, err := verificationFromRequest(record, req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record.RemoteArtifact = &artifact
	return store.TouchWrite(stateRoot, record)
}

func Validate(store Store, stateRoot, id string, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRecord, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	_, err = verificationFromRequest(record, req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

func Projection(record model.IssueOpsRecord, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRemoteArtifactVerification, error) {
	return verificationFromRequest(record, req)
}

func verificationFromRequest(record model.IssueOpsRecord, req model.IssueOpsRemoteArtifactVerificationRequest) (model.IssueOpsRemoteArtifactVerification, error) {
	if record.Phase != model.IssueOpsPhasePR {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("cannot verify remote artifact before pr phase")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "github" && provider != "gitlab" {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if issueProvider := remote.ProviderFromURL(record.IssueURL); issueProvider != "" && provider != issueProvider {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact provider must match linked issue provider")
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	if kind != "pr" && kind != "mr" {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact kind must be pr or mr")
	}
	if provider == "github" && kind != "pr" {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("github remote artifact kind must be pr")
	}
	if provider == "gitlab" && kind != "mr" {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("gitlab remote artifact kind must be mr")
	}
	artifactURL := strings.TrimSpace(req.URL)
	if err := remote.ValidateArtifactURL(artifactURL, provider, kind); err != nil {
		return model.IssueOpsRemoteArtifactVerification{}, err
	}
	if err := remote.ValidateArtifactMatchesIssue(record.IssueURL, artifactURL, provider, kind); err != nil {
		return model.IssueOpsRemoteArtifactVerification{}, err
	}
	labels := remote.CleanValues(req.Labels)
	if len(labels) == 0 {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact labels are required")
	}
	assignees := remote.CleanValues(req.Assignees)
	if len(assignees) == 0 {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact assignees are required")
	}
	if invalid := remote.InvalidAssignee(assignees); invalid != "" {
		return model.IssueOpsRemoteArtifactVerification{}, fmt.Errorf("remote artifact assignee must be a verified provider user, not placeholder %q", invalid)
	}
	return model.IssueOpsRemoteArtifactVerification{
		Provider:     provider,
		Kind:         kind,
		URL:          artifactURL,
		Labels:       labels,
		Assignees:    assignees,
		TargetBranch: strings.TrimSpace(req.TargetBranch),
		VerifiedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}
