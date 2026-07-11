package issueops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func TestWriteIssueOpsRejectsInvalidHandoffBeforeReplacingStoredBytes(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	workerRoot := handoffPrepareWorktreePath(record)
	valid, err := handoff.Prepare(record, handoff.PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-write-guard", AttemptBaseHead: record.BranchPrepare.BaseSHA,
		CoordinatorRoot: record.Repo, WorkerRoot: workerRoot, Agent: "codex", Now: "2026-07-11T01:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteIssueOps(stateRoot, valid); err != nil {
		t.Fatalf("store valid envelope: %v", err)
	}
	wantBytes := rawIssueOpsBytesForTest(t, stateRoot, record.ID)

	tests := []struct {
		name   string
		mutate func(*IssueOpsRecord)
	}{
		{name: "invalid state", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff.State = "future_state" }},
		{name: "padded enum", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff.Driver = " orca " }},
		{name: "mismatched coordinator root", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff.CoordinatorRoot = r.Repo + "-other" }},
		{name: "malformed attempt sha", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff.AttemptBaseHead = "not-a-commit" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid := valid
			h := *valid.ExecutionHandoff
			invalid.ExecutionHandoff = &h
			tt.mutate(&invalid)
			if _, err := WriteIssueOps(stateRoot, invalid); err == nil {
				t.Fatal("invalid execution_handoff write must fail before Put")
			}
			if got := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(got, wantBytes) {
				t.Fatalf("rejected write changed stored bytes\nwant=%s\n got=%s", wantBytes, got)
			}
		})
	}
}

func TestReadIssueOpsPreservesIdentifiableInvalidEnvelopeWithoutLeakingValues(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	invalid, err := handoff.Prepare(record, handoff.PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-invalid", AttemptBaseHead: record.BranchPrepare.BaseSHA,
		CoordinatorRoot: record.Repo, WorkerRoot: handoffPrepareWorktreePath(record), Agent: "codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := "super-secret-token"
	apiSecret := "super-secret-value"
	invalid.ExecutionHandoff.State = "Authorization: Bearer " + secret + strings.Repeat("x", 16*1024)
	invalid.ExecutionHandoff.PendingOperation = &IssueOpsExecutionHandoffPendingOperation{
		Kind: "api_key=" + apiSecret + strings.Repeat("y", 16*1024),
	}
	putRawIssueOpsRecordForTest(t, stateRoot, invalid)

	got, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr == nil {
		t.Fatal("corrupted handoff must fail validation")
	}
	if got.ID != record.ID || got.Repo != record.Repo || got.ExecutionHandoff == nil || got.ExecutionHandoff.WorkerRoot != invalid.ExecutionHandoff.WorkerRoot {
		t.Fatalf("identifiable invalid record was discarded: %#v", got)
	}
	diagnostic := readErr.Error()
	if strings.Contains(diagnostic, secret) || strings.Contains(diagnostic, apiSecret) || len(diagnostic) > 512 {
		t.Fatalf("invalid-envelope diagnostic leaked or exceeded its bound: len=%d", len(diagnostic))
	}
}

func TestWriteIssueOpsStillAllowsLegacyInlineRecordWithoutHandoff(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	record.ExecutionHandoff = nil
	record.UpdatedAt = "2026-07-11T02:00:00Z"
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatalf("legacy inline write: %v", err)
	}
	if got.ExecutionHandoff != nil || !strings.Contains(string(rawIssueOpsBytesForTest(t, stateRoot, record.ID)), "2026-07-11T02:00:00Z") {
		t.Fatalf("legacy inline record was not stored: %#v", got)
	}
}

func TestReadIssueOpsRejectsOversizedPriorAttemptHistory(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	valid, err := handoff.Prepare(record, handoff.PrepareRequest{
		Attempt: 1, OwnershipEpoch: "epoch-prior-bounds", AttemptBaseHead: record.BranchPrepare.BaseSHA,
		CoordinatorRoot: record.Repo, WorkerRoot: handoffPrepareWorktreePath(record), Agent: "codex", Now: "2026-07-11T01:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteIssueOps(stateRoot, valid); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(rawIssueOpsBytesForTest(t, stateRoot, record.ID), &raw); err != nil {
		t.Fatal(err)
	}
	h := raw["execution_handoff"].(map[string]any)
	h["prior_attempts"] = make([]any, 17)
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err == nil {
		t.Fatal("oversized prior-attempt history must fail closed")
	}
}

func TestReadIssueOpsRejectsOversizedPriorAttemptBytes(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	current, err := handoff.Prepare(record, handoff.PrepareRequest{
		Attempt: 17, OwnershipEpoch: "epoch-current", AttemptBaseHead: record.BranchPrepare.BaseSHA,
		CoordinatorRoot: record.Repo, WorkerRoot: handoffPrepareWorktreePath(record), Agent: "codex", Now: "2026-07-11T01:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", 4096)
	for attempt := 1; attempt <= 16; attempt++ {
		current.ExecutionHandoff.PriorAttempts = append(current.ExecutionHandoff.PriorAttempts, IssueOpsExecutionHandoffPriorAttempt{
			ProtocolVersion: handoff.ProtocolVersion,
			State:           handoff.StateClosed, ClosedDisposition: handoff.DispositionCancelled,
			Attempt: attempt, OwnershipEpoch: fmt.Sprintf("epoch-%d", attempt), AttemptBaseHead: record.BranchPrepare.BaseSHA,
			ContextSHA256: strings.Repeat("a", 64), ContextSourceSHA256: strings.Repeat("b", 64), ContextVersion: handoff.ContextVersion,
			ContextOptions: &model.IssueOpsExecutionHandoffContextOptions{WorkerScope: large, HeartbeatCadence: large, ResultFormat: large},
			Driver:         "orca", Agent: "codex", CoordinatorRoot: record.Repo, WorkerRoot: handoffPrepareWorktreePath(record),
			Failure:    &IssueOpsExecutionHandoffFailure{Code: "forced_claimed_cancel", Message: strings.Repeat("c", 8192), At: "2026-07-11T00:30:00Z"},
			PreparedAt: "2026-07-11T00:00:00Z", UpdatedAt: "2026-07-11T00:30:00Z",
		})
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "prior attempts exceed 262144 bytes") {
		t.Fatalf("oversized prior-attempt bytes must fail closed at the aggregate bound, got %v", err)
	}
}

func rawIssueOpsBytesForTest(t *testing.T, stateRoot, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("missing IssueOps row %q", id)
	}
	return append([]byte(nil), b...)
}

func putRawIssueOpsRecordForTest(t *testing.T, stateRoot string, record IssueOpsRecord) {
	t.Helper()
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, b); err != nil {
		t.Fatal(err)
	}
}
