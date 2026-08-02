package issueopslease

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResumeReceiptJSONKeepsOnlySealedArtifactReferences(t *testing.T) {
	want := ResumeReceipt{
		Execution: Execution{Mode: "orca"},
		Artifacts: ResumeArtifacts{
			ClaimTokenPath:      "/worktree/lease-1.token",
			IssueBodySHA256:     "issue-sha",
			ContextPacketPath:   "/worktree/context.json",
			ContextPacketSHA256: "packet-sha",
			OwnerPromptPath:     "/worktree/owner-prompt.txt",
			OwnerPromptSHA256:   "prompt-sha",
		},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["execution"] == nil || fields["artifacts"] == nil {
		t.Fatalf("resume receipt fields=%s", data)
	}
	if bytes.Contains(data, []byte("\"claim_token\"")) {
		t.Fatalf("resume receipt leaked a claim token: %s", data)
	}
}

func TestResumeStageReceiptJSONPreservesRunIdentity(t *testing.T) {
	want := ResumeStageReceipt{RunID: "run-resume", RunBound: true}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got ResumeStageReceipt
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Run stage receipt round-trip=%#v want=%#v", got, want)
	}
}
