package harnessapp

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	"agent-harness/internal/adapter/outbound/sqlstore"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

const (
	resumeWiringRecordBucket = "issueops_v1"
	resumeWiringIntentBucket = "external_intent_v1"
)

func TestCoreResumeEffectsRejectsRawSnapshotDriftWithoutAdditionalMutation(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, stateRoot string, effects *coreResumeEffects, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts)
	}{
		{
			name: "begin_record",
			run: func(t *testing.T, stateRoot string, effects *coreResumeEffects, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts) {
				resumeWiringDriftRecord(t, stateRoot, record.ID)
				beforeRecord := resumeWiringRawRow(t, stateRoot, resumeWiringRecordBucket, record.ID)
				_, err := effects.Begin(context.Background(), record, raw, artifacts, leasedomain.ResumePlan{RuntimeID: "runtime"}, strings.Repeat("a", 32))
				resumeWiringRequireStale(t, err, "stale raw record snapshot")
				resumeWiringAssertRows(t, stateRoot, record.ID, strings.Repeat("a", 32), beforeRecord, nil)
			},
		},
		{
			name: "mark_intent",
			run: func(t *testing.T, stateRoot string, effects *coreResumeEffects, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts) {
				state := resumeWiringBegin(t, stateRoot, effects, record, raw, artifacts, strings.Repeat("b", 32))
				resumeWiringDriftIntent(t, stateRoot, state.OperationID)
				beforeRecord := resumeWiringRawRow(t, stateRoot, resumeWiringRecordBucket, record.ID)
				beforeIntent := resumeWiringRawRow(t, stateRoot, resumeWiringIntentBucket, state.OperationID)
				_, err := effects.MarkInvoking(context.Background(), state)
				resumeWiringRequireStale(t, err, "stale raw intent snapshot")
				resumeWiringAssertRows(t, stateRoot, record.ID, state.OperationID, beforeRecord, beforeIntent)
			},
		},
		{
			name: "apply_record",
			run: func(t *testing.T, stateRoot string, effects *coreResumeEffects, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts) {
				state := resumeWiringBegin(t, stateRoot, effects, record, raw, artifacts, strings.Repeat("c", 32))
				resumeWiringDriftRecord(t, stateRoot, record.ID)
				beforeRecord := resumeWiringRawRow(t, stateRoot, resumeWiringRecordBucket, record.ID)
				beforeIntent := resumeWiringRawRow(t, stateRoot, resumeWiringIntentBucket, state.OperationID)
				_, err := effects.ApplyReceipt(context.Background(), state, leasecontract.ResumeStageReceipt{TerminalPTYID: "pty-resume"})
				resumeWiringRequireStale(t, err, "stale raw record snapshot")
				resumeWiringAssertRows(t, stateRoot, record.ID, state.OperationID, beforeRecord, beforeIntent)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, coreRecord, tokenPath, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
			canonicalRoot := coreRecord.Repo + ".worktrees/" + coreRecord.Branch
			coreRecord.WorktreePath = canonicalRoot
			coreRecord.Execution.Workspace.Root = canonicalRoot
			if _, err := issueops.WriteIssueOps(stateRoot, coreRecord); err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			raw, ok, err := db.Get(resumeWiringRecordBucket, coreRecord.ID)
			if err != nil || !ok {
				t.Fatalf("read raw record ok=%t err=%v", ok, err)
			}
			record, err := leasecontract.Decode(coreRecord.ID, raw)
			if err != nil {
				t.Fatal(err)
			}
			artifacts := leasecontract.ResumeArtifacts{
				ClaimTokenPath: tokenPath, IssueBodySHA256: issueDigest,
				ContextPacketPath: issueops.SealedOwnerContextPacketPath(coreRecord), ContextPacketSHA256: packetDigest,
				OwnerPromptPath:   strings.TrimSuffix(issueops.SealedOwnerContextPacketPath(coreRecord), "context.json") + "owner-prompt.txt",
				OwnerPromptSHA256: strings.Repeat("d", 64),
			}
			effects := &coreResumeEffects{stateRoot: stateRoot, now: func() time.Time { return time.Date(2026, time.July, 31, 2, 20, 0, 0, time.UTC) }}
			tt.run(t, stateRoot, effects, record, raw, artifacts)
		})
	}
}

func TestResumePortReceiptPreservesRunStages(t *testing.T) {
	created := resumePortReceipt(string(port.ExecutionOrcaIntentRun), leasecontract.ResumeStageReceipt{RunID: "run-resume"})
	if created.RunID != "run-resume" || created.RunBound {
		t.Fatalf("Run create receipt=%#v", created)
	}
	bound := resumePortReceipt(string(port.ExecutionOrcaIntentRunBind), leasecontract.ResumeStageReceipt{RunID: "run-resume", RunBound: true})
	if bound.RunID != "run-resume" || !bound.RunBound {
		t.Fatalf("Run bind receipt=%#v", bound)
	}
}

func resumeWiringBegin(t *testing.T, stateRoot string, effects *coreResumeEffects, record leasecontract.Record, raw []byte, artifacts leasecontract.ResumeArtifacts, operationID string) leaseoutbound.ResumeEffectState {
	t.Helper()
	state, err := effects.Begin(context.Background(), record, raw, artifacts, leasedomain.ResumePlan{RuntimeID: "runtime"}, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func resumeWiringDriftRecord(t *testing.T, stateRoot, id string) {
	t.Helper()
	record, err := issueops.ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	record.UpdatedAt += "-drift"
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
}

func resumeWiringDriftIntent(t *testing.T, stateRoot, operationID string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(resumeWiringIntentBucket, operationID)
	if err != nil || !ok {
		t.Fatalf("read raw intent ok=%t err=%v", ok, err)
	}
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: resumeWiringIntentBucket, ID: operationID, Data: append(append([]byte(nil), raw...), ' ')}}); err != nil {
		t.Fatal(err)
	}
}

func resumeWiringRequireStale(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("stale error=%v want=%q", err, want)
	}
}

func resumeWiringAssertRows(t *testing.T, stateRoot, id, operationID string, wantRecord, wantIntent []byte) {
	t.Helper()
	if got := resumeWiringRawRow(t, stateRoot, resumeWiringRecordBucket, id); !bytes.Equal(got, wantRecord) {
		t.Fatal("record raw bytes changed after rejected CAS")
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Get(resumeWiringIntentBucket, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if wantIntent == nil {
		if ok {
			t.Fatal("rejected begin created an external intent")
		}
		return
	}
	if !ok || !bytes.Equal(got, wantIntent) {
		t.Fatal("external intent raw bytes changed after rejected CAS")
	}
}

func resumeWiringRawRow(t *testing.T, stateRoot, bucket, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil || !ok {
		t.Fatalf("read %s/%s ok=%t err=%v", bucket, id, ok, err)
	}
	return raw
}
