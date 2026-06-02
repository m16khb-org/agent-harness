package core

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestRunIssueOpsRemoteAgyJudgeUsesStrictJSONWrapper(t *testing.T) {
	dir := t.TempDir()
	fakeAgy := filepath.Join(dir, "fake-agy.sh")
	out := IssueOpsRemoteScoringResult{
		OK:        true,
		Provider:  "github",
		Threshold: 0.70,
		SelectedRelatedIssues: []IssueOpsRemoteScoredItem{{
			ID:        "#9",
			Score:     0.91,
			Threshold: 0.70,
			Selected:  true,
			Evidence:  []string{"shared IssueOps workflow"},
			ApplyHint: "link in issue body: #9",
		}},
		SelectedLabels: []IssueOpsRemoteScoredItem{{
			Name:      "enhancement",
			Score:     0.93,
			Threshold: 0.70,
			Selected:  true,
			Evidence:  []string{"feature request"},
			ApplyHint: "apply GitHub label: enhancement",
		}},
		ApplyInstructions: []string{"apply selected labels with gh issue create --label or gh issue edit --add-label: enhancement"},
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\nprintf '%s' '"+string(b)+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   dir,
		AgyCommand: fakeAgy,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps related issue scoring"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected strict JSON result from fake agy: %+v", result)
	}
}
