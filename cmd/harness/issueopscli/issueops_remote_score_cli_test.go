package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunIssueOpsRemoteScoreCLIDeterministic(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, issueopscore.IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue: issueopscore.IssueOpsRemoteArtifact{
			Title: "IssueOps related issue and label scoring",
			Body:  "Score related issues and enhancement labels before creating an issue.",
		},
		IssueCandidates: []issueopscore.IssueOpsRemoteIssueCandidate{
			{ID: "#11", Title: "IssueOps related issue and label scoring", Score: scoreForCLITest(0.93)},
			{ID: "#8", Title: "Unrelated docs cleanup", Score: scoreForCLITest(0.30)},
		},
		LabelCandidates: []issueopscore.IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: scoreForCLITest(0.90)},
			{Name: "documentation", Score: scoreForCLITest(0.20)},
		},
	})
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "none", "--json"})
	})
	var result issueopscore.IssueOpsRemoteScoringResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("remote score should return JSON: %v\n%s", err, out)
	}
	if !result.OK || len(result.SelectedRelatedIssues) != 1 || result.SelectedRelatedIssues[0].ID != "#11" {
		t.Fatalf("expected threshold-selected issue: %+v", result)
	}
	if len(result.SelectedLabels) != 1 || result.SelectedLabels[0].Name != "enhancement" {
		t.Fatalf("expected threshold-selected label: %+v", result)
	}
}

func TestRunIssueOpsRemoteScoreCLIAcceptsCandidateAliases(t *testing.T) {
	input := filepath.Join(t.TempDir(), "remote-score-alias.json")
	if err := os.WriteFile(input, []byte(`{
		"provider": "github",
		"threshold": 0.7,
		"issue": {"title": "IssueOps feedback gate", "body": "Feedback contract gate should block PR readiness."},
		"related_issues": [{"id": "#11", "title": "IssueOps feedback gate", "score": 0.93}],
		"labels": [{"name": "bug", "score": 0.91}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "none", "--json"})
	})
	var result issueopscore.IssueOpsRemoteScoringResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("remote score should return JSON: %v\n%s", err, out)
	}
	if len(result.SelectedRelatedIssues) != 1 || result.SelectedRelatedIssues[0].ID != "#11" {
		t.Fatalf("expected alias related issue to be selected: %+v", result)
	}
	if len(result.SelectedLabels) != 1 || result.SelectedLabels[0].Name != "bug" {
		t.Fatalf("expected alias label to be selected: %+v", result)
	}
}

func TestRunIssueOpsRemoteScoreFailureWithJSONEmitsStructuredError(t *testing.T) {
	input := filepath.Join(t.TempDir(), "remote-score-invalid.json")
	if err := os.WriteFile(input, []byte(`{
		"provider": "jira",
		"issue": {"title": "Bad provider", "body": "body"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "none", "--json"})
	})
	if err == nil {
		t.Fatalf("remote score with invalid provider should still return an error")
	}
	var failure map[string]any
	if unmarshalErr := json.Unmarshal([]byte(out), &failure); unmarshalErr != nil {
		t.Fatalf("remote score failure with --json should emit JSON stdout: %v\n%s", unmarshalErr, out)
	}
	errorText, _ := failure["error"].(string)
	if failure["ok"] != false || !strings.Contains(errorText, "unsupported issueops remote provider") {
		t.Fatalf("unexpected structured failure payload: %#v", failure)
	}
}

func TestRunIssueOpsRemoteScoreCLIUsesJudgeFile(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, issueopscore.IssueOpsRemoteScoringRequest{
		Provider: "gitlab",
		Issue:    issueopscore.IssueOpsRemoteArtifact{Title: "IssueOps GitLab remote scoring"},
	})
	judgeFile := filepath.Join(t.TempDir(), "judge.json")
	if err := os.WriteFile(judgeFile, []byte(`{"ok":true,"provider":"gitlab","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[{"id":"#11","score":0.91,"threshold":0.7,"selected":true,"evidence":["same IssueOps workflow"],"apply_hint":"link in issue body: #11"}],"rejected_related_issues":[],"selected_labels":[{"name":"enhancement","score":0.94,"threshold":0.7,"selected":true,"evidence":["feature request"],"apply_hint":"apply GitLab label: enhancement"}],"rejected_labels":[],"apply_instructions":["apply selected labels with the GitLab issue labels field or glab issue create --label: enhancement"],"warnings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "file", "--judge-file", judgeFile, "--json"})
	})
	var result issueopscore.IssueOpsRemoteScoringResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("remote judge-file score should return JSON: %v\n%s", err, out)
	}
	if result.Provider != "gitlab" || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected GitLab judge-file score result: %+v", result)
	}
}

func TestRunIssueOpsRemoteScoreCLIRendersHostAgentPrompt(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, issueopscore.IssueOpsRemoteScoringRequest{
		Provider: "github",
		Issue:    issueopscore.IssueOpsRemoteArtifact{Title: "IssueOps host-agent scoring prompt"},
	})
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "prompt", "--json"})
	})
	var result struct {
		OK             bool   `json:"ok"`
		ExecutionClass string `json:"execution_class"`
		ReadOnly       bool   `json:"read_only"`
		JoinBefore     string `json:"join_before"`
		Prompt         string `json:"prompt"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("remote prompt should return JSON: %v\n%s", err, out)
	}
	if !result.OK || result.ExecutionClass != "background_join" || !result.ReadOnly || result.JoinBefore != "remote_artifact_write" {
		t.Fatalf("expected read-only background prompt envelope: %+v", result)
	}
	if !strings.Contains(result.Prompt, "IssueOps host-agent scoring prompt") ||
		!strings.Contains(result.Prompt, "selected_related_issues") {
		t.Fatalf("prompt should contain the request and output contract:\n%s", result.Prompt)
	}
}

func TestRunIssueOpsRemoteScoreCLITextShowsIssueTitleWithReference(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, issueopscore.IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue:     issueopscore.IssueOpsRemoteArtifact{Title: "IssueOps related issue scoring"},
		IssueCandidates: []issueopscore.IssueOpsRemoteIssueCandidate{
			{ID: "#11", Title: "IssueOps related issue and label scoring", Score: scoreForCLITest(0.93)},
		},
	})
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "none"})
	})
	if !strings.Contains(out, "#11 (IssueOps related issue and label scoring)") {
		t.Fatalf("text output should include issue reference and title, got:\n%s", out)
	}
}
