package remote

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScoreIssueOpsRemoteCandidatesSelectsThresholdMatches(t *testing.T) {
	relatedScore := 0.92
	unrelatedScore := 0.42
	labelScore := 0.88
	docScore := 0.40
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue: IssueOpsRemoteArtifact{
			Title: "IssueOps 이슈 생성 시 관련 이슈와 라벨 점수화",
			Body:  "관련 이슈 링크와 enhancement 라벨을 threshold 기반으로 적용한다.",
		},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#9", Title: "IssueOps autoresearch quality gate", URL: "https://example.com/issues/9", Score: &relatedScore},
			{ID: "#7", Title: "Template cleanup", URL: "https://example.com/issues/7", Score: &unrelatedScore},
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: &labelScore},
			{Name: "documentation", Score: &docScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Provider != "github" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.SelectedRelatedIssues) != 1 || result.SelectedRelatedIssues[0].ID != "#9" {
		t.Fatalf("expected only #9 selected: %+v", result.SelectedRelatedIssues)
	}
	if len(result.SelectedLabels) != 1 || result.SelectedLabels[0].Name != "enhancement" {
		t.Fatalf("expected only enhancement selected: %+v", result.SelectedLabels)
	}
	if !containsFold(strings.Join(result.ApplyInstructions, "\n"), "gh issue") {
		t.Fatalf("expected GitHub apply instruction: %+v", result.ApplyInstructions)
	}
	if result.ExecutionClass != "background_join" || !result.ReadOnly {
		t.Fatalf("expected background join read-only classification: %+v", result)
	}
}

func TestScoreIssueOpsRemoteCandidatesSupportsGitLabInstructions(t *testing.T) {
	score := 0.95
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue:     IssueOpsRemoteArtifact{Title: "IssueOps GitLab label support"},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: &score},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedLabels) != 1 {
		t.Fatalf("expected selected GitLab label: %+v", result)
	}
	if !containsFold(strings.Join(result.ApplyInstructions, "\n"), "glab issue create") {
		t.Fatalf("expected GitLab apply instruction: %+v", result.ApplyInstructions)
	}
}

func TestScoreIssueOpsRemoteCandidatesGitLabRelatedIssuesUseLinkedItems(t *testing.T) {
	relatedScore := 0.92
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue:     IssueOpsRemoteArtifact{Title: "IssueOps GitLab linked item 연결"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#9", Title: "IssueOps autoresearch quality gate", URL: "https://gitlab.example.com/group/repo/-/issues/9", Score: &relatedScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 {
		t.Fatalf("expected one selected GitLab related issue: %+v", result.SelectedRelatedIssues)
	}
	joined := strings.Join(result.ApplyInstructions, "\n")
	if !containsFold(joined, "linked item") || !containsFold(joined, "/links") {
		t.Fatalf("expected GitLab linked-item apply instruction: %+v", result.ApplyInstructions)
	}
	if containsFold(joined, "in the issue body") {
		t.Fatalf("GitLab related issues must not be instructed into the issue body: %+v", result.ApplyInstructions)
	}
	if hint := result.SelectedRelatedIssues[0].ApplyHint; !containsFold(hint, "linked item") {
		t.Fatalf("expected GitLab linked-item apply hint: %q", hint)
	}
}

func TestScoreIssueOpsRemoteCandidatesDoesNotWarnWhenNoCandidatesExist(t *testing.T) {
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider: "github",
		Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote score with no candidate data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("empty candidate lists should not produce threshold warnings: %+v", result.Warnings)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		`"selected_related_issues":[]`,
		`"rejected_related_issues":[]`,
		`"selected_labels":[]`,
		`"rejected_labels":[]`,
		`"apply_instructions":[]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected empty array %s in JSON, got %s", want, text)
		}
	}
	if strings.Contains(text, "null") {
		t.Fatalf("remote score JSON must use [] instead of null: %s", text)
	}
}

func TestDecodeIssueOpsRemoteScoringRequestAcceptsCandidateAliases(t *testing.T) {
	req, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"provider": "github",
		"issue": {"title": "IssueOps feedback gate"},
		"related_issues": [{"id": "#1", "title": "IssueOps feedback gate"}],
		"labels": [{"name": "bug", "score": 0.91}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.IssueCandidates) != 1 || req.IssueCandidates[0].ID != "#1" {
		t.Fatalf("expected related_issues alias to populate issue candidates: %+v", req.IssueCandidates)
	}
	if len(req.LabelCandidates) != 1 || req.LabelCandidates[0].Name != "bug" {
		t.Fatalf("expected labels alias to populate label candidates: %+v", req.LabelCandidates)
	}
}

func TestDecodeIssueOpsRemoteScoringRequestRejectsDuplicateCandidateFields(t *testing.T) {
	if _, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"issue": {"title": "IssueOps feedback gate"},
		"issue_candidates": [],
		"related_issues": []
	}`)); err == nil || !strings.Contains(err.Error(), "issue_candidates") {
		t.Fatalf("expected duplicate issue candidate field error, got %v", err)
	}
	if _, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"issue": {"title": "IssueOps feedback gate"},
		"label_candidates": [],
		"labels": []
	}`)); err == nil || !strings.Contains(err.Error(), "label_candidates") {
		t.Fatalf("expected duplicate label candidate field error, got %v", err)
	}
}
