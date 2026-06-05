package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsBenchmarkCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "none", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["fixture_count"].(float64) < 1 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}

func TestRunIssueOpsBenchmarkCLIAgyKeepsOneScorePerFixture(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
cat <<'EOF'
{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"judge ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "agy", "--agy-command", fakeAgy, "--json"})
	})
	var result struct {
		OK           bool             `json:"ok"`
		FixtureCount int              `json:"fixture_count"`
		Scores       []map[string]any `json:"scores"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if !result.OK || len(result.Scores) != result.FixtureCount {
		t.Fatalf("expected one merged score per fixture: %+v", result)
	}
}

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

func TestRunIssueOpsBenchmarkGateCLIKeepsCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	baseline := core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	}
	candidateRun := baseline
	candidateRun.ID = "candidate"
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(baseline)); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(candidateRun)); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Bounded IssueOps changes should pass the gate.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "skills/issueops/SKILL.md", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.KeepCandidate {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestRunIssueOpsBenchmarkGateCLIDiscardsOutsideEditSurface(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	run := core.FinalizeIssueOpsBenchmarkRunResult(core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	})
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, run); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Only skill changes are allowed.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "baseline", "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueops.go", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 {
		t.Fatalf("expected edit-surface discard: %+v", result)
	}
}

func writeIssueOpsRemoteScoreRequestForCLITest(t *testing.T, req core.IssueOpsRemoteScoringRequest) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "remote-score.json")
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func scoreForCLITest(score float64) *float64 {
	return &score
}

func writeIssueOpsCandidateForCLITest(t *testing.T, candidate core.IssueOpsAutoresearchCandidate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate.json")
	b, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
