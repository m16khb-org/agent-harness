package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunIssueOpsRemoteScoreTextOutputCoversLabelsAndAlias(t *testing.T) {
	input := writeIssueOpsRemoteScoreRequestForCLITest(t, core.IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue:     core.IssueOpsRemoteArtifact{Title: "IssueOps GitLab score text"},
		IssueCandidates: []core.IssueOpsRemoteIssueCandidate{
			{URL: "https://gitlab.example/group/project/-/issues/17", Title: "Reuse related IssueOps report", Score: scoreForCLITest(0.92)},
		},
		LabelCandidates: []core.IssueOpsRemoteLabelCandidate{
			{Name: "refactor", Score: scoreForCLITest(0.88)},
		},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote-score", "--input", input, "--judge", "none"})
	})
	for _, want := range []string{
		"provider=gitlab threshold=0.70 related_issues=1 labels=1",
		"- related issue: https://gitlab.example/group/project/-/issues/17 (Reuse related IssueOps report) score=0.92",
		"- label: refactor score=0.88",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("remote score text output missing %q:\n%s", want, out)
		}
	}
}

func TestRunIssueOpsRemoteScoreErrorBranches(t *testing.T) {
	help := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"remote", "help"})
	})
	if !strings.Contains(help, "Usage: agent-harness issueops remote score --input PATH [--judge none|agy] [--json]") {
		t.Fatalf("remote help missing score usage:\n%s", help)
	}

	missingOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"remote", "score", "--judge", "none", "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "input is required") {
		t.Fatalf("missing input error = %v", err)
	}
	var missing map[string]any
	if jsonErr := json.Unmarshal([]byte(missingOut), &missing); jsonErr != nil {
		t.Fatalf("missing input should emit JSON error: %v\n%s", jsonErr, missingOut)
	}
	if missing["ok"] != false || !strings.Contains(missing["error"].(string), "input is required") {
		t.Fatalf("unexpected missing input payload: %#v", missing)
	}

	input := writeIssueOpsRemoteScoreRequestForCLITest(t, core.IssueOpsRemoteScoringRequest{
		Provider: "github",
		Issue:    core.IssueOpsRemoteArtifact{Title: "Unsupported judge"},
	})
	unsupportedOut, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"remote", "score", "--input", input, "--judge", "bogus", "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported issueops remote score judge "bogus"`) {
		t.Fatalf("unsupported judge error = %v", err)
	}
	var unsupported map[string]any
	if jsonErr := json.Unmarshal([]byte(unsupportedOut), &unsupported); jsonErr != nil {
		t.Fatalf("unsupported judge should emit JSON error: %v\n%s", jsonErr, unsupportedOut)
	}
	if unsupported["ok"] != false || !strings.Contains(unsupported["error"].(string), `unsupported issueops remote score judge "bogus"`) {
		t.Fatalf("unexpected unsupported judge payload: %#v", unsupported)
	}
}
