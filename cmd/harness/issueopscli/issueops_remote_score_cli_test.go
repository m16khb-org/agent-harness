package issueopscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsRemoteScoreCLIDeterministic(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, core.IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue: core.IssueOpsRemoteArtifact{
			Title: "IssueOps related issue and label scoring",
			Body:  "Score related issues and enhancement labels before creating an issue.",
		},
		IssueCandidates: []core.IssueOpsRemoteIssueCandidate{
			{ID: "#11", Title: "IssueOps related issue and label scoring", Score: scoreForCLITest(0.93)},
			{ID: "#8", Title: "Unrelated docs cleanup", Score: scoreForCLITest(0.30)},
		},
		LabelCandidates: []core.IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: scoreForCLITest(0.90)},
			{Name: "documentation", Score: scoreForCLITest(0.20)},
		},
	})
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "none", "--json"})
	})
	var result core.IssueOpsRemoteScoringResult
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
	var result core.IssueOpsRemoteScoringResult
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

func TestRunIssueOpsRemoteScoreCLIAgyUsesExternalLLMWrapper(t *testing.T) {
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo missing agy flags >&2
  exit 2
fi
cat <<'EOF'
{"ok":true,"provider":"gitlab","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[{"id":"#11","score":0.91,"threshold":0.7,"selected":true,"evidence":["same IssueOps workflow"],"apply_hint":"link in issue body: #11"}],"rejected_related_issues":[],"selected_labels":[{"name":"enhancement","score":0.94,"threshold":0.7,"selected":true,"evidence":["feature request"],"apply_hint":"apply GitLab label: enhancement"}],"rejected_labels":[],"apply_instructions":["apply selected labels with the GitLab issue labels field or glab issue create --label: enhancement"],"warnings":[]}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, core.IssueOpsRemoteScoringRequest{
		Provider: "gitlab",
		Issue:    core.IssueOpsRemoteArtifact{Title: "IssueOps GitLab remote scoring"},
	})
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "agy", "--agy-command", fakeAgy, "--json"})
	})
	var result core.IssueOpsRemoteScoringResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("remote agy score should return JSON: %v\n%s", err, out)
	}
	if result.Provider != "gitlab" || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected GitLab agy score result: %+v", result)
	}
}

func TestRunIssueOpsRemoteScoreCLITextShowsIssueTitleWithReference(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, core.IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue:     core.IssueOpsRemoteArtifact{Title: "IssueOps related issue scoring"},
		IssueCandidates: []core.IssueOpsRemoteIssueCandidate{
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
