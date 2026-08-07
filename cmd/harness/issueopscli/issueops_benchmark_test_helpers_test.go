package issueopscli

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeIssueOpsRemoteScoreRequestForCLITest(t *testing.T, req issueopscore.IssueOpsRemoteScoringRequest) string {
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

func writeIssueOpsCandidateForCLITest(t *testing.T, candidate issueopscore.IssueOpsAutoresearchCandidate) string {
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
