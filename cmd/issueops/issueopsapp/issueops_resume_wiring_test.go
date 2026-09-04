package issueopsapp

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leaseinbound "issueops/internal/adapter/inbound/issueopslease"
	"issueops/internal/adapter/issueops"
	leaseoutbound "issueops/internal/adapter/outbound/issueopslease"
	"issueops/internal/adapter/outbound/sqlstore"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

const (
	resumeWiringRecordBucket = "issueops_v1"
	resumeWiringIntentBucket = "external_intent_v1"
)

func TestResumePlanIdentityFailureStopsBeforeOperationAndOrcaMutation(t *testing.T) {
	stateRoot, record, _, _, _ := seedOrcaClaimSnapshot(t)
	record.PlanPath = filepath.Join(record.Execution.Workspace.Root, "plans", "missing.md")
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	fake := &resumePlanMutationFake{}
	service, err := newIssueOpsResumeService(stateRoot, fake, fake)
	if err != nil {
		t.Fatal(err)
	}
	handler := leaseinbound.NewResumeHandler(service)
	result, err := handler(context.Background(), stateRoot, issueops.ExecutionResumeRequest{
		ID: record.ID, ExpectedGeneration: 1, Actor: claimWiringActor(t),
		CWD: record.Execution.Workspace.Root, Confirm: true,
	})
	if err == nil || result.OK {
		t.Fatalf("result=%#v err=%v, want plan identity failure", result, err)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "orca_plan_artifact_required" {
		t.Fatalf("error=%T %v want orca_plan_artifact_required", err, err)
	}
	if fake.ownerCalls != 0 || fake.probeCalls != 0 || fake.inspectCalls != 0 || fake.invokeCalls != 0 {
		t.Fatalf("owner=%d probe=%d inspect=%d invoke=%d, want all zero", fake.ownerCalls, fake.probeCalls, fake.inspectCalls, fake.invokeCalls)
	}
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := database.List(resumeWiringIntentBucket)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 0 {
		t.Fatalf("resume plan failure persisted operations: %v", operations)
	}
	persisted, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Pending != nil || persisted.Execution.Lease.Generation != 1 || persisted.Execution.Lease.Status != record.Execution.Lease.Status {
		t.Fatalf("resume plan failure mutated execution: %#v", persisted.Execution)
	}
}

type resumePlanMutationFake struct {
	ownerCalls   int
	probeCalls   int
	inspectCalls int
	invokeCalls  int
}

func (fake *resumePlanMutationFake) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	fake.probeCalls++
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true}, nil
}

func (fake *resumePlanMutationFake) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	fake.inspectCalls++
	return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
}

func (fake *resumePlanMutationFake) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	fake.invokeCalls++
	return port.ExecutionOrcaIntentReceipt{}, nil
}

func (fake *resumePlanMutationFake) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	fake.ownerCalls++
	return port.ExecutionOrcaOwnerInventory{}, nil
}

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
