package issueops

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
)

func TestSchemaV9MigrationClassifiesDeterministicV8Shapes(t *testing.T) {
	tests := []struct {
		name   string
		record IssueOpsRecord
		state  IssueOpsCycleState
		phase  IssueOpsPhase
		active int
	}{
		{name: "no handoff active", record: v8MigrationRecordForTest(t), state: IssueOpsCycleActive, phase: IssueOpsPhaseImplement},
		{name: "no handoff done", record: func() IssueOpsRecord { r := v8MigrationRecordForTest(t); r.Phase = IssueOpsPhaseDone; return r }(), state: IssueOpsCycleClosed, phase: IssueOpsPhaseDone},
		{name: "active owner", record: v8MigrationOwnershipRecordForTest(t, false), state: IssueOpsCycleActive, phase: IssueOpsPhaseImplement, active: 1},
		{name: "completed owner with remote artifact", record: func() IssueOpsRecord {
			r := v8MigrationOwnershipRecordForTest(t, true)
			r.Phase = IssueOpsPhaseDone
			r.IssueURL = "https://github.com/acme/repo/issues/68"
			r.ExecutionHandoff.ClosedDisposition = ""
			r.ExecutionHandoff.Failure = nil
			r.ExecutionHandoff.OwnerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "owner-session"}
			r.ExecutionHandoff.Orientation = &IssueOpsOwnershipOrientation{IssueURL: r.IssueURL, PlanSHA256: strings.Repeat("b", 64), Understanding: "understood", ScopeConfirmation: "scoped", RecordedAt: "2026-07-22T01:00:00Z"}
			r.ExecutionHandoff.Completion = &IssueOpsOwnershipCompletion{FinalHead: strings.Repeat("a", 40), CompletedAt: "2026-07-22T02:00:00Z"}
			r.RemoteArtifact = &IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pull_request", URL: "https://github.com/acme/repo/pull/68", VerifiedAt: "2026-07-22T02:00:00Z"}
			return r
		}(), state: IssueOpsCycleClosed, phase: IssueOpsPhaseDone},
		{name: "cancelled force released", record: func() IssueOpsRecord {
			r := v8MigrationOwnershipRecordForTest(t, true)
			r.Phase = IssueOpsPhaseDone
			r.ForceReleasedAt = "2026-07-22T02:00:00Z"
			r.PhaseLedger = IssueOpsPhaseLedger{IssueOpsPhaseImplement: {Phase: IssueOpsPhaseImplement, EnteredAt: "2026-07-22T00:30:00Z"}, IssueOpsPhaseDone: {Phase: IssueOpsPhaseDone}}
			return r
		}(), state: IssueOpsCyclePaused, phase: IssueOpsPhaseImplement},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.record)
			before := append([]byte(nil), raw...)
			classification, err := ClassifyIssueOpsV8Migration(raw)
			if err != nil {
				t.Fatal(err)
			}
			migrated, err := MigrateIssueOpsV8Record(tt.record)
			if err != nil {
				t.Fatal(err)
			}
			if classification.Classification == "" || migrated.SchemaVersion != 9 || migrated.CycleState != tt.state || migrated.Phase != tt.phase {
				t.Fatalf("unexpected migration classification=%+v record=%+v", classification, migrated)
			}
			if migrated.Ownership != nil && migrated.Ownership.ActiveAttempt != tt.active {
				t.Fatalf("active attempt=%d, want %d", migrated.Ownership.ActiveAttempt, tt.active)
			}
			if string(raw) != string(before) {
				t.Fatal("classification mutated v8 input bytes")
			}
		})
	}
}

func TestSchemaV9MigrationRejectsAmbiguousPausedPhaseAndInvalidEpochs(t *testing.T) {
	ambiguous := v8MigrationOwnershipRecordForTest(t, true)
	ambiguous.Phase = IssueOpsPhaseDone
	ambiguous.ForceReleasedAt = "2026-07-22T02:00:00Z"
	ambiguousRaw, _ := json.Marshal(ambiguous)
	if _, err := MigrateIssueOpsV8Record(ambiguous); err == nil || !strings.Contains(err.Error(), "phase_ledger") {
		t.Fatalf("expected phase-ledger blocker, got %v", err)
	}
	ambiguousAfter, _ := json.Marshal(ambiguous)
	if !bytes.Equal(ambiguousRaw, ambiguousAfter) {
		t.Fatal("failed migration mutated ambiguous v8 input")
	}
	invalid := v8MigrationOwnershipRecordForTest(t, false)
	invalid.ExecutionHandoff.WorkspaceEpoch = "other"
	invalidRaw, _ := json.Marshal(invalid)
	if _, err := MigrateIssueOpsV8Record(invalid); err == nil || !strings.Contains(err.Error(), "epoch") {
		t.Fatalf("expected epoch blocker, got %v", err)
	}
	invalidAfter, _ := json.Marshal(invalid)
	if !bytes.Equal(invalidRaw, invalidAfter) {
		t.Fatal("failed migration mutated invalid-epoch v8 input")
	}
}

func TestFutureSchemaNeverMigratesOrWrites(t *testing.T) {
	record := v8MigrationRecordForTest(t)
	record.SchemaVersion = 10
	raw, _ := json.Marshal(record)
	if _, err := ClassifyIssueOpsV8Migration(raw); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("future schema classification must fail closed: %v", err)
	}
	if _, err := MigrateIssueOpsV8Record(record); err == nil || !strings.Contains(err.Error(), "schema_version 8") {
		t.Fatalf("future schema migration must fail closed: %v", err)
	}
}

func v8MigrationRecordForTest(t *testing.T) IssueOpsRecord {
	t.Helper()
	return IssueOpsRecord{SchemaVersion: 8, ID: "io-migrate", Repo: "/tmp/agent-harness-v8-migrate", Branch: "68-demo", Phase: IssueOpsPhaseImplement, CreatedAt: "2026-07-22T00:00:00Z", UpdatedAt: "2026-07-22T00:00:00Z"}
}

func v8MigrationOwnershipRecordForTest(t *testing.T, closed bool) IssueOpsRecord {
	t.Helper()
	record := v8MigrationRecordForTest(t)
	attempt := ownershipAttemptForTest(t, 1, closed)
	record.Repo = attempt.Workspace.CoordinatorRoot
	record.ExecutionWorkspace = attempt.Workspace
	record.ExecutionHandoff = attempt.Handoff
	record.ExecutionHandoff.PreparedAt = attempt.StartedAt
	if closed {
		record.ExecutionHandoff.UpdatedAt = attempt.ClosedAt
		record.ExecutionHandoff.ClosedDisposition = handoff.DispositionCancelled
	}
	return record
}
