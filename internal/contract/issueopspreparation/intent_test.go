package issueopspreparation

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

const (
	prepareOperationID = "0123456789abcdef0123456789abcdef"
	resumeOperationID  = "fedcba9876543210fedcba9876543210"
	digestA            = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB            = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	prepareIntentJSON = `{"schema_version":1,"purpose":"prepare","operation_id":"0123456789abcdef0123456789abcdef","lifecycle_id":"io-codec-prepare","generation":1,"stage":"worktree_create","marker":"agent-harness issueops-v1 lifecycle=io-codec-prepare operation=0123456789abcdef0123456789abcdef provider=github issue=199","started_at":"2026-08-02T00:00:00Z","invocation_state":"not_invoked_proven","invocation_attempts":0,"workspace":{"lifecycle_id":"io-codec-prepare","source_root":"/repo","root":"/repo.worktrees/199-prepare","branch":"199-prepare","base_branch":"117-parent","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm":true},"probe":{"repo":"/repo","host":"codex","model":"gpt-5.6","effort":"high","provider":"github","issue":199,"marker":"agent-harness issueops-v1 lifecycle=io-codec-prepare operation=0123456789abcdef0123456789abcdef provider=github issue=199"},"issue_body_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`

	resumeIntentJSON = `{"schema_version":1,"purpose":"resume","operation_id":"fedcba9876543210fedcba9876543210","lifecycle_id":"io-codec-resume","generation":2,"stage":"terminal_create","marker":"agent-harness issueops-v1 resume lifecycle=io-codec-resume generation=2 operation=fedcba9876543210fedcba9876543210 provider=github issue=199","started_at":"2026-08-02T00:01:00Z","invocation_state":"not_invoked_proven","invocation_attempts":0,"workspace":{"lifecycle_id":"io-codec-resume","source_root":"/repo","root":"/repo.worktrees/199-resume","branch":"199-resume","base_branch":"117-parent","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm":true},"probe":{"repo":"/repo","host":"claude","model":"claude-opus-4-1","effort":"high","provider":"github","issue":199,"marker":"agent-harness issueops-v1 resume lifecycle=io-codec-resume generation=2 operation=fedcba9876543210fedcba9876543210 provider=github issue=199"},"prepared":{"workspace":{"source_root":"/repo","root":"/repo.worktrees/199-resume","branch":"199-resume","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","driver":"orca"},"runtime_id":"runtime","repo_id":"repo","worktree_id":"worktree","worktree_instance_id":"instance"},"launch":{"prompt_path":"/repo.worktrees/199-resume/.agent-harness/owner.md","prompt_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","context_packet_path":"/repo.worktrees/199-resume/.agent-harness/context.json","context_packet_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"issue_body_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","claim_token_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","prior_binding":{"runtime_id":"runtime","repo_id":"repo","worktree_id":"worktree","worktree_instance_id":"instance","lease_generation":1,"owner_host":"claude","owner_model":"claude-opus-4-1","owner_effort":"high","run_id":"run-old","task_id":"task-old","dispatch_id":"dispatch-old","terminal_pty_id":"terminal-old"},"resume_lease":{"generation":2,"status":"claimable","claim_token_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
)

func TestIntentCodecRoundTripsPrepareAndResumeBytes(t *testing.T) {
	codec := IntentCodec{}
	for _, test := range []struct {
		name        string
		operationID string
		raw         string
	}{
		{name: "prepare", operationID: prepareOperationID, raw: prepareIntentJSON},
		{name: "resume", operationID: resumeOperationID, raw: resumeIntentJSON},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, err := codec.Decode(test.operationID, []byte(test.raw))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			encoded, err := codec.Encode(decoded)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(encoded, []byte(test.raw)) {
				t.Fatalf("intent bytes changed\nwant=%s\n got=%s", test.raw, encoded)
			}
		})
	}
}

func TestIntentCodecAcceptsOmoOwner(t *testing.T) {
	var intent Intent
	if err := json.Unmarshal([]byte(prepareIntentJSON), &intent); err != nil {
		t.Fatal(err)
	}
	intent.Probe.Host = "omo"
	intent.Probe.Model = ImplementerModelOmo
	intent.Probe.Effort = ImplementerEffortOmo
	if err := (IntentCodec{}).Validate(intent, prepareOperationID); err != nil {
		t.Fatalf("Omo owner intent must be valid: %v", err)
	}
}

func TestIntentCodecRejectsIdentityAndAuthorityDrift(t *testing.T) {
	codec := IntentCodec{}
	tests := []struct {
		name        string
		operationID string
		raw         string
		want        string
	}{
		{name: "operation", operationID: resumeOperationID, raw: prepareIntentJSON, want: "Orca external intent payload is invalid"},
		{name: "prepare generation", operationID: prepareOperationID, raw: strings.Replace(prepareIntentJSON, `"generation":1`, `"generation":2`, 1), want: "Orca prepare intent payload is invalid"},
		{name: "resume authority", operationID: resumeOperationID, raw: strings.Replace(resumeIntentJSON, `,"resume_lease":{"generation":2,"status":"claimable","claim_token_sha256":"`+digestB+`"}`, "", 1), want: "Orca resume intent payload is invalid"},
		{name: "marker issue", operationID: prepareOperationID, raw: strings.ReplaceAll(prepareIntentJSON, "provider=github issue=199", "provider=github issue=200"), want: "intent_identity_mismatch"},
		{name: "attempt bound", operationID: prepareOperationID, raw: strings.Replace(prepareIntentJSON, `"invocation_attempts":0`, `"invocation_attempts":3`, 1), want: "Orca external intent payload is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := codec.Decode(test.operationID, []byte(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want substring %q", err, test.want)
			}
		})
	}
}

func TestIntentCodecRejectsNonCanonicalMarkerTokensAndPrepareGeneration(t *testing.T) {
	codec := IntentCodec{}
	for _, identity := range []MarkerIdentity{
		{Purpose: PurposePrepare, LifecycleID: "io-prepare", Generation: 2, OperationID: prepareOperationID, Provider: "github", Issue: 199},
		{Purpose: PurposePrepare, LifecycleID: "io-prepare\u00a0suffix", Generation: 1, OperationID: prepareOperationID, Provider: "github", Issue: 199},
	} {
		if marker, err := codec.RenderMarker(identity); err == nil {
			t.Fatalf("non-canonical identity rendered marker %q", marker)
		}
	}
}

func TestIntentCodecCanonicalizeRejectsRetiredMarkerWithoutMutation(t *testing.T) {
	codec := IntentCodec{}
	raw := retiredPrepareIntentBytes(t)
	record := prepareIntentRecord(t, "github", "https://github.com/m16khb/agent-harness/issues/199", raw)
	beforeRaw := append([]byte(nil), raw...)
	beforeRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}

	_, encoded, err := codec.Canonicalize(record, raw)
	if err == nil || encoded != nil || !strings.Contains(err.Error(), "intent_marker_invalid") {
		t.Fatalf("canonicalize = encoded:%s err:%v", encoded, err)
	}
	if !bytes.Equal(raw, beforeRaw) {
		t.Fatal("retired marker rejection mutated the input bytes")
	}
	afterRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterRecord, beforeRecord) {
		t.Fatal("retired marker rejection mutated the record")
	}
}

func TestPrepareIssueIdentityAndReadinessMarker(t *testing.T) {
	record := leasecontract.Record{
		ID: "io-prepare", IssueURL: "https://github.com/example/repo/issues/199",
		BranchPrepare: []byte(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/199","link_verified":false}`),
	}
	codec := IntentCodec{}
	issue, err := codec.PrepareIssueIdentity(record)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Provider != "github" || issue.Issue != 199 {
		t.Fatalf("issue=%+v", issue)
	}
	marker, err := codec.RenderReadinessMarker(record.ID, issue)
	if err != nil {
		t.Fatal(err)
	}
	if marker != "agent-harness issueops-v1 lifecycle=io-prepare provider=github issue=199" {
		t.Fatalf("marker=%q", marker)
	}

	record.IssueURL = "https://gitlab.com/example/repo/-/work_items/199"
	record.BranchPrepare = []byte(`{"provider":"gitlab","issue_url":"https://gitlab.com/example/repo/-/work_items/199","link_verified":false}`)
	if _, err := codec.PrepareIssueIdentity(record); err == nil {
		t.Fatal("GitLab preparation identity accepted without a verified link")
	}
}

func retiredPrepareIntentBytes(t *testing.T) []byte {
	t.Helper()
	canonical := "agent-harness issueops-v1 lifecycle=io-codec-prepare operation=" + prepareOperationID + " provider=github issue=199"
	retired := "agent-harness issueops-v1 lifecycle=io-codec-prepare operation=" + prepareOperationID
	return []byte(strings.ReplaceAll(prepareIntentJSON, canonical, retired))
}

func prepareIntentRecord(t *testing.T, provider, issueURL string, raw []byte) leasecontract.Record {
	t.Helper()
	var intent Intent
	if err := json.Unmarshal(raw, &intent); err != nil {
		t.Fatal(err)
	}
	branchPrepare, err := json.Marshal(map[string]any{
		"provider": provider, "issue_url": issueURL, "link_verified": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return leasecontract.Record{
		ID: intent.LifecycleID, IssueURL: issueURL, BranchPrepare: branchPrepare,
		Execution: &leasecontract.Execution{
			Lease: leasecontract.Lease{Generation: intent.Generation, Status: "released"},
			Pending: &leasecontract.ExternalIntent{
				OperationID: intent.OperationID, Kind: "worktree_create", Marker: intent.Marker,
			},
		},
	}
}
