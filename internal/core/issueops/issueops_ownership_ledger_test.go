package issueops

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func TestCycleStateSchemaV9Matrix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IssueOpsRecord)
		valid  bool
	}{
		{name: "active before handoff", mutate: func(*IssueOpsRecord) {}, valid: true},
		{name: "active current attempt", mutate: func(r *IssueOpsRecord) { r.Ownership = ownershipLedgerForTest(t, false) }, valid: true},
		{name: "paused historical attempts", mutate: func(r *IssueOpsRecord) {
			r.CycleState = IssueOpsCyclePaused
			r.Ownership = ownershipLedgerForTest(t, true)
		}, valid: true},
		{name: "closed done", mutate: func(r *IssueOpsRecord) { r.CycleState = IssueOpsCycleClosed; r.Phase = IssueOpsPhaseDone }, valid: true},
		{name: "active done", mutate: func(r *IssueOpsRecord) { r.Phase = IssueOpsPhaseDone }, valid: false},
		{name: "paused done", mutate: func(r *IssueOpsRecord) { r.CycleState = IssueOpsCyclePaused; r.Phase = IssueOpsPhaseDone }, valid: false},
		{name: "paused active pointer", mutate: func(r *IssueOpsRecord) {
			r.CycleState = IssueOpsCyclePaused
			r.Ownership = ownershipLedgerForTest(t, true)
			r.Ownership.ActiveAttempt = 1
		}, valid: false},
		{name: "closed non-done", mutate: func(r *IssueOpsRecord) { r.CycleState = IssueOpsCycleClosed }, valid: false},
		{name: "closed active pointer", mutate: func(r *IssueOpsRecord) {
			r.CycleState = IssueOpsCycleClosed
			r.Phase = IssueOpsPhaseDone
			r.Ownership = ownershipLedgerForTest(t, true)
			r.Ownership.ActiveAttempt = 1
		}, valid: false},
		{name: "unknown state", mutate: func(r *IssueOpsRecord) { r.CycleState = "retired" }, valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := schemaV9RecordForTest(t)
			tt.mutate(&record)
			err := ValidateIssueOpsOwnershipLedger(record)
			if tt.valid && err != nil {
				t.Fatalf("valid record rejected: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatalf("invalid record accepted: %+v", record)
			}
		})
	}
}

func TestOwnershipLedgerSchemaV9RejectsInvalidAttemptShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IssueOpsOwnershipLedger)
	}{
		{name: "empty ledger", mutate: func(l *IssueOpsOwnershipLedger) { l.Attempts = nil; l.ActiveAttempt = 0 }},
		{name: "duplicate number", mutate: func(l *IssueOpsOwnershipLedger) {
			l.Attempts = append(l.Attempts, l.Attempts[0])
			l.Attempts[1].ClosedAt = "2026-07-22T01:00:00Z"
		}},
		{name: "non-monotonic number", mutate: func(l *IssueOpsOwnershipLedger) {
			second := ownershipAttemptForTest(t, 2, false)
			second.Number = 0
			l.Attempts = append(l.Attempts, second)
		}},
		{name: "missing active attempt", mutate: func(l *IssueOpsOwnershipLedger) { l.ActiveAttempt = 9 }},
		{name: "active points to closed", mutate: func(l *IssueOpsOwnershipLedger) {
			l.Attempts[0].Handoff.State = handoff.StateClosed
			l.Attempts[0].Handoff.ClosedDisposition = handoff.DispositionCancelled
			l.Attempts[0].Handoff.Failure = cancelledFailureForTest()
			l.Attempts[0].ClosedAt = "2026-07-22T01:00:00Z"
		}},
		{name: "historical attempt remains active", mutate: func(l *IssueOpsOwnershipLedger) {
			historical := ownershipAttemptForTest(t, 1, false)
			current := ownershipAttemptForTest(t, 2, false)
			current.RestartedFrom = 1
			l.Attempts = []IssueOpsOwnershipAttempt{historical, current}
			l.ActiveAttempt = 2
		}},
		{name: "workspace epoch reuse", mutate: func(l *IssueOpsOwnershipLedger) {
			first := ownershipAttemptForTest(t, 1, true)
			second := ownershipAttemptForTest(t, 2, false)
			second.RestartedFrom = 1
			second.Workspace.WorkspaceEpoch = first.Workspace.WorkspaceEpoch
			second.Handoff.WorkspaceEpoch = first.Workspace.WorkspaceEpoch
			l.Attempts = []IssueOpsOwnershipAttempt{first, second}
			l.ActiveAttempt = 2
		}},
		{name: "ownership epoch reuse", mutate: func(l *IssueOpsOwnershipLedger) {
			first := ownershipAttemptForTest(t, 1, true)
			second := ownershipAttemptForTest(t, 2, false)
			second.RestartedFrom = 1
			second.Handoff.OwnershipEpoch = first.Handoff.OwnershipEpoch
			l.Attempts = []IssueOpsOwnershipAttempt{first, second}
			l.ActiveAttempt = 2
		}},
		{name: "attempt and handoff number mismatch", mutate: func(l *IssueOpsOwnershipLedger) { l.Attempts[0].Handoff.Attempt = 2 }},
		{name: "workspace and handoff epoch mismatch", mutate: func(l *IssueOpsOwnershipLedger) { l.Attempts[0].Handoff.WorkspaceEpoch = "workspace-other" }},
		{name: "active attempt has closed_at", mutate: func(l *IssueOpsOwnershipLedger) { l.Attempts[0].ClosedAt = "2026-07-22T01:00:00Z" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := schemaV9RecordForTest(t)
			record.Ownership = ownershipLedgerForTest(t, false)
			tt.mutate(record.Ownership)
			if err := ValidateIssueOpsOwnershipLedger(record); err == nil {
				t.Fatalf("invalid ledger accepted: %+v", record.Ownership)
			}
		})
	}
}

func TestOwnershipLedgerSchemaV9RejectsMalformedInheritedWIPSeal(t *testing.T) {
	validSeal := IssueOpsOwnershipWIPSeal{
		Ref: "refs/agent-harness/issueops/io-ledger/attempts/2/wip", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), BaseHead: strings.Repeat("c", 40),
		StatusSHA256: strings.Repeat("d", 64), PathManifestSHA256: strings.Repeat("e", 64), PathCount: 2, CreatedAt: "2026-07-22T01:00:00Z",
	}
	mutations := []func(*IssueOpsOwnershipWIPSeal){
		func(s *IssueOpsOwnershipWIPSeal) { s.Ref = "refs/heads/unsafe" },
		func(s *IssueOpsOwnershipWIPSeal) { s.Commit = "short" },
		func(s *IssueOpsOwnershipWIPSeal) { s.Tree = "short" },
		func(s *IssueOpsOwnershipWIPSeal) { s.BaseHead = "short" },
		func(s *IssueOpsOwnershipWIPSeal) { s.StatusSHA256 = "short" },
		func(s *IssueOpsOwnershipWIPSeal) { s.PathManifestSHA256 = "short" },
		func(s *IssueOpsOwnershipWIPSeal) { s.PathCount = 0 },
		func(s *IssueOpsOwnershipWIPSeal) { s.CreatedAt = "not-a-time" },
	}
	for index, mutate := range mutations {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			record := schemaV9RecordForTest(t)
			first := ownershipAttemptForTest(t, 1, true)
			second := ownershipAttemptForTest(t, 2, false)
			second.RestartedFrom = 1
			seal := validSeal
			mutate(&seal)
			second.InheritedWIPSeal = &seal
			record.Ownership = &IssueOpsOwnershipLedger{ActiveAttempt: 2, Attempts: []IssueOpsOwnershipAttempt{first, second}}
			if err := ValidateIssueOpsOwnershipLedger(record); err == nil {
				t.Fatalf("malformed seal accepted: %+v", seal)
			}
		})
	}
}

func TestOwnershipLedgerAccessorsUseOnlyActivePointer(t *testing.T) {
	record := schemaV9RecordForTest(t)
	first := ownershipAttemptForTest(t, 1, true)
	second := ownershipAttemptForTest(t, 2, false)
	second.RestartedFrom = 1
	record.Ownership = &IssueOpsOwnershipLedger{ActiveAttempt: 2, Attempts: []IssueOpsOwnershipAttempt{first, second}}

	current := CurrentOwnershipAttempt(record)
	last := LastOwnershipAttempt(record)
	if current == nil || current.Number != 2 || last == nil || last.Number != 2 {
		t.Fatalf("unexpected accessors current=%+v last=%+v", current, last)
	}
	record.Ownership.ActiveAttempt = 0
	if CurrentOwnershipAttempt(record) != nil || LastOwnershipAttempt(record).Number != 2 {
		t.Fatalf("historical last attempt became live authority")
	}
}

func TestOwnershipLedgerAppendCASPreservesHistoricalBytesAndDeepCopies(t *testing.T) {
	record := schemaV9RecordForTest(t)
	first := ownershipAttemptForTest(t, 1, true)
	record.CycleState = IssueOpsCyclePaused
	record.Ownership = &IssueOpsOwnershipLedger{Attempts: []IssueOpsOwnershipAttempt{first}}
	expected := CloneIssueOpsOwnershipLedger(record.Ownership)
	before, err := json.Marshal(record.Ownership.Attempts[0])
	if err != nil {
		t.Fatal(err)
	}
	successor := ownershipAttemptForTest(t, 2, false)
	successor.RestartedFrom = 1

	updated, err := AppendIssueOpsOwnershipAttempt(record, expected, successor)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(updated.Ownership.Attempts[0])
	if string(before) != string(after) {
		t.Fatalf("historical attempt changed\nbefore=%s\nafter=%s", before, after)
	}
	successor.Workspace.WorkerRoot = "/mutated-by-caller"
	if updated.Ownership.Attempts[1].Workspace.WorkerRoot == successor.Workspace.WorkerRoot {
		t.Fatal("append retained caller-owned nested pointer")
	}

	staleExpected := CloneIssueOpsOwnershipLedger(expected)
	staleExpected.Attempts[0].Handoff.OwnershipEpoch = "stale-history"
	if _, err := AppendIssueOpsOwnershipAttempt(record, staleExpected, ownershipAttemptForTest(t, 2, false)); err == nil {
		t.Fatal("historical attempt mutation bypassed ledger CAS")
	}
}

func TestOwnershipLedgerPendingRestartHasNoDispatcherReceipts(t *testing.T) {
	restartType := reflect.TypeOf(IssueOpsOwnerRestartIntent{})
	for _, forbidden := range []string{"Terminal", "Task", "Dispatch", "TerminalID", "TaskID", "DispatchID"} {
		if _, ok := restartType.FieldByName(forbidden); ok {
			t.Fatalf("pending restart duplicates dispatcher receipt field %s", forbidden)
		}
	}
	record := schemaV9RecordForTest(t)
	second := ownershipAttemptForTest(t, 2, false)
	second.Handoff.PendingOperation = &model.IssueOpsExecutionHandoffPendingOperation{Kind: handoff.OperationTaskCreate, StartedAt: "2026-07-22T01:00:00Z"}
	second.Handoff.Orca.TaskID = "task-successor"
	record.Ownership = &IssueOpsOwnershipLedger{ActiveAttempt: 2, Attempts: []IssueOpsOwnershipAttempt{ownershipAttemptForTest(t, 1, true), second}}
	if CurrentOwnershipAttempt(record).Handoff.Orca.TaskID != "task-successor" {
		t.Fatal("successor handoff does not own dispatcher receipt")
	}
}

func schemaV9RecordForTest(t *testing.T) IssueOpsRecord {
	t.Helper()
	return IssueOpsRecord{SchemaVersion: 9, ID: "io-ledger", Repo: ownerRestartLedgerRepo, Branch: "68-demo", Phase: IssueOpsPhaseImplement, CycleState: IssueOpsCycleActive}
}

func ownershipLedgerForTest(t *testing.T, closed bool) *IssueOpsOwnershipLedger {
	t.Helper()
	attempt := ownershipAttemptForTest(t, 1, closed)
	active := 1
	if closed {
		active = 0
	}
	return &IssueOpsOwnershipLedger{ActiveAttempt: active, Attempts: []IssueOpsOwnershipAttempt{attempt}}
}

func ownershipAttemptForTest(t *testing.T, number int, closed bool) IssueOpsOwnershipAttempt {
	t.Helper()
	repo := ownerRestartLedgerRepo
	worker := ownerRestartLedgerWorker
	workspaceEpoch := "workspace-" + string(rune('0'+number))
	h := &IssueOpsExecutionHandoff{
		State: handoff.StateOwnershipDispatching, Attempt: number, OwnershipEpoch: "ownership-" + string(rune('0'+number)), WorkspaceEpoch: workspaceEpoch,
		WorkspaceSHA256: strings.Repeat("a", 64), AttemptBaseHead: strings.Repeat("b", 40), Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worker,
		Orca: &model.IssueOpsOrcaIdentity{},
	}
	attempt := IssueOpsOwnershipAttempt{
		Number:    number,
		Workspace: &IssueOpsExecutionWorkspace{State: "ready", WorkspaceEpoch: workspaceEpoch, Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worker, PreparationSession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "prep"}, BaseHead: strings.Repeat("b", 40)},
		Handoff:   h, StartedAt: "2026-07-22T00:00:00Z",
	}
	if closed {
		h.State = handoff.StateClosed
		h.ClosedDisposition = handoff.DispositionCancelled
		h.Failure = cancelledFailureForTest()
		attempt.ClosedAt = "2026-07-22T01:00:00Z"
	}
	return attempt
}

const (
	ownerRestartLedgerRepo   = "/tmp/agent-harness-owner-restart-ledger-repo"
	ownerRestartLedgerWorker = "/tmp/agent-harness-owner-restart-ledger-worker"
)

func cancelledFailureForTest() *model.IssueOpsExecutionHandoffFailure {
	return &model.IssueOpsExecutionHandoffFailure{Code: "cancellation_finalized", Message: "owner released", At: "2026-07-22T01:00:00Z"}
}
