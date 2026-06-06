package issueops

import (
	"strings"
	"testing"
)

func TestValidateIssueOpsRemoteArtifactVerificationChecksWithoutPersisting(t *testing.T) {
	stateRoot := t.TempDir()
	record := IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    IssueOpsPhasePR,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	got, err := ValidateIssueOpsRemoteArtifactVerification(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pull_request",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{" enhancement ", "issueops"},
		Assignees: []string{" habin "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != record.ID || got.RemoteArtifact != nil {
		t.Fatalf("validate should return original record without persisting artifact: %+v", got)
	}
	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.RemoteArtifact != nil {
		t.Fatalf("validate should not persist artifact, got %+v", reloaded.RemoteArtifact)
	}
}

func TestValidateIssueOpsRemoteArtifactVerificationRejectsInvalidRecord(t *testing.T) {
	stateRoot := t.TempDir()
	record := IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    IssueOpsPhaseFeedback,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	got, err := ValidateIssueOpsRemoteArtifactVerification(stateRoot, record.ID, IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{"issueops"},
		Assignees: []string{"habin"},
	})
	if err == nil || !strings.Contains(err.Error(), "before pr phase") {
		t.Fatalf("expected phase validation error, got record %+v err %v", got, err)
	}
	if got.OK {
		t.Fatalf("invalid validation should return ok=false record: %+v", got)
	}
}

func TestScoreIssueOpsRemoteCandidatesRequiresExplicitLabelDecisionWhenAllLabelsRejected(t *testing.T) {
	labelScore := 0.40
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue: IssueOpsRemoteArtifact{
			Title: "IssueOps MR 라벨 누락 방지",
			Body:  "원격 이슈와 MR 생성 전에 라벨 결정을 검증한다.",
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "documentation", Score: &labelScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedLabels) != 0 {
		t.Fatalf("expected all labels rejected: %+v", result.SelectedLabels)
	}
	joined := strings.Join(result.ApplyInstructions, "\n")
	if !containsFold(joined, "stop before remote artifact writes") || !containsFold(joined, "manual label") {
		t.Fatalf("expected explicit label-decision instruction before remote writes: %+v", result.ApplyInstructions)
	}
	if !containsFold(strings.Join(result.Warnings, "\n"), "no label candidates met threshold") {
		t.Fatalf("expected no-label warning: %+v", result.Warnings)
	}
}
