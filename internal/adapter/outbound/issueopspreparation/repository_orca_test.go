package issueopspreparation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func TestOrcaIntentRepositoryBeginsBeforeEffectAtomically(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Pending || state.Intent.Stage != preparationcontract.IntentStageWorktree || len(state.IntentRaw) == 0 {
		t.Fatalf("state=%+v", state)
	}
	if len(store.applies) != 1 || len(store.applies[0]) != 2 {
		t.Fatalf("applies=%+v", store.applies)
	}
	if store.applies[0][0].Bucket != recordBucket || store.applies[0][1].Bucket != intentBucket || !store.applies[0][1].RequireAbsent {
		t.Fatalf("mutations=%+v", store.applies[0])
	}
	persisted, err := leasecontract.Decode(begin.Snapshot.Record.ID, store.mustGet(recordBucket, begin.Snapshot.Record.ID))
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution == nil || persisted.Execution.Mode != preparationcontract.ModeOrca || persisted.Execution.Lease.Status != "released" || persisted.Execution.Pending == nil || persisted.Execution.Pending.Marker != state.Intent.Marker {
		t.Fatalf("execution=%+v", persisted.Execution)
	}
	if persisted.Execution.Selection == nil || persisted.Execution.Selection.ReadinessFingerprint != begin.Selection.ReadinessFingerprint {
		t.Fatalf("selection=%+v", persisted.Execution.Selection)
	}
	decoded, err := (preparationcontract.IntentCodec{}).Decode(begin.OperationID, store.mustGet(intentBucket, begin.OperationID))
	if err != nil || decoded.Marker != state.Intent.Marker {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
}

func TestOrcaIntentRepositoryCASCompletesClaimableAuthority(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []preparationcontract.IntentStage{
		preparationcontract.IntentStageWorktree, preparationcontract.IntentStageTerminal,
		preparationcontract.IntentStageRun, preparationcontract.IntentStageRunBind,
		preparationcontract.IntentStageTask, preparationcontract.IntentStageDispatch,
	} {
		if state.Intent.Stage != stage {
			t.Fatalf("stage=%s want=%s", state.Intent.Stage, stage)
		}
		state, err = repository.MarkInvoking(context.Background(), state)
		if err != nil {
			t.Fatal(err)
		}
		if stage == preparationcontract.IntentStageWorktree {
			state.OwnerArtifacts = preparationcontract.OwnerArtifacts{
				PlanPath:       "/repo.worktrees/199-orca/.issueops/artifact/plan.md",
				ClaimTokenPath: "/repo.worktrees/199-orca/.issueops/state/claim", ClaimTokenSHA256: strings.Repeat("d", 64),
				ContextPacketPath: "/repo.worktrees/199-orca/.issueops/context.json", ContextPacketSHA256: strings.Repeat("c", 64),
				OwnerPromptPath: "/repo.worktrees/199-orca/.issueops/owner.md", OwnerPromptSHA256: strings.Repeat("b", 64),
			}
		}
		progress, applyErr := repository.ApplyReceipt(context.Background(), state, repositoryOrcaReceipt(stage))
		if applyErr != nil {
			t.Fatal(applyErr)
		}
		state = progress.State
		if stage == preparationcontract.IntentStageWorktree {
			persisted, decodeErr := leasecontract.Decode(begin.Snapshot.Record.ID, store.mustGet(recordBucket, begin.Snapshot.Record.ID))
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if persisted.PlanPath != state.OwnerArtifacts.PlanPath || state.Snapshot.Record.PlanPath != state.OwnerArtifacts.PlanPath {
				t.Fatalf("worktree receipt did not atomically persist plan path: persisted=%q snapshot=%q want=%q", persisted.PlanPath, state.Snapshot.Record.PlanPath, state.OwnerArtifacts.PlanPath)
			}
		}
		if stage != preparationcontract.IntentStageDispatch && !progress.Pending {
			t.Fatalf("stage %s unexpectedly terminal", stage)
		}
		if stage == preparationcontract.IntentStageDispatch {
			if progress.Pending || !progress.Result.OK || progress.Result.Execution == nil {
				t.Fatalf("progress=%+v", progress)
			}
			if progress.Result.ClaimTokenPath != state.OwnerArtifacts.ClaimTokenPath || progress.Result.ContextPacketSHA256 != state.OwnerArtifacts.ContextPacketSHA256 {
				t.Fatalf("result=%+v artifacts=%+v", progress.Result, state.OwnerArtifacts)
			}
		}
	}
	if store.cas != 12 {
		t.Fatalf("CAS count=%d want=12", store.cas)
	}
	if _, exists, _ := store.Get(intentBucket, begin.OperationID); exists {
		t.Fatal("terminal intent row was not deleted")
	}
	record, err := leasecontract.Decode(begin.Snapshot.Record.ID, store.mustGet(recordBucket, begin.Snapshot.Record.ID))
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Pending != nil || record.Execution.Failure != nil || record.Execution.Lease.Status != "claimable" || record.Execution.Lease.Holder != nil || record.Execution.Lease.ClaimTokenSHA256 != strings.Repeat("d", 64) {
		t.Fatalf("execution=%+v", record.Execution)
	}
	if record.PlanPath != state.OwnerArtifacts.PlanPath {
		t.Fatalf("durable plan path=%q want %q", record.PlanPath, state.OwnerArtifacts.PlanPath)
	}
	binding := record.Execution.Orca
	if binding == nil || binding.RuntimeID != "runtime" || binding.RunID != "run" || binding.TaskID != "task" || binding.DispatchID != "dispatch" || binding.LeaseGeneration != 1 {
		t.Fatalf("binding=%+v", binding)
	}
	if binding.ArtifactIdentityVersion != leasecontract.OrcaArtifactIdentityVersion {
		t.Fatalf("binding artifact identity version=%d want=%d", binding.ArtifactIdentityVersion, leasecontract.OrcaArtifactIdentityVersion)
	}
	if binding.IssueBodySHA256 != state.Intent.IssueBodySHA256 ||
		binding.ContextPacketSHA256 != state.OwnerArtifacts.ContextPacketSHA256 ||
		binding.OwnerPromptSHA256 != state.OwnerArtifacts.OwnerPromptSHA256 {
		t.Fatalf("binding artifact identity=%+v intent=%+v artifacts=%+v", binding, state.Intent, state.OwnerArtifacts)
	}
	if len(store.rows[holderBucket]) != 0 {
		t.Fatalf("claimable execution wrote holder index: %+v", store.rows[holderBucket])
	}
}

func TestOrcaIntentRepositoryRejectsStaleDualRawCAS(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	store.rows[recordBucket][begin.Snapshot.Record.ID] = append(store.rows[recordBucket][begin.Snapshot.Record.ID], ' ')
	if _, err := repository.MarkInvoking(context.Background(), state); err == nil || !strings.Contains(err.Error(), "stale raw") {
		t.Fatalf("record CAS err=%v", err)
	}

	store, repository, begin = newOrcaRepositoryFixture(t)
	state, err = repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	store.rows[intentBucket][begin.OperationID] = append(store.rows[intentBucket][begin.OperationID], ' ')
	if _, err := repository.MarkInvoking(context.Background(), state); err == nil || !strings.Contains(err.Error(), "stale raw") {
		t.Fatalf("intent CAS err=%v", err)
	}
}

func TestOrcaIntentRepositoryRecordsBoundedFailureByCAS(t *testing.T) {
	store, repository, begin := newOrcaRepositoryFixture(t)
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	state.FailureAt = "2026-08-02T00:00:01Z"
	if err := repository.RecordFailure(context.Background(), state, preparationcontract.InvocationUnknown, &longDiagnosticError{}); err != nil {
		t.Fatal(err)
	}
	record, err := leasecontract.Decode(begin.Snapshot.Record.ID, store.mustGet(recordBucket, begin.Snapshot.Record.ID))
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Failure == nil || record.Execution.Failure.Message != "external operation failed" || record.Execution.Failure.At != state.FailureAt {
		t.Fatalf("failure=%+v", record.Execution.Failure)
	}
	intent, err := (preparationcontract.IntentCodec{}).Decode(begin.OperationID, store.mustGet(intentBucket, begin.OperationID))
	if err != nil || intent.InvocationState != preparationcontract.InvocationUnknown {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
}

func TestOrcaIntentRepositoryUsesInjectedBoundedDiagnosticRedactor(t *testing.T) {
	store, _, begin := newOrcaRepositoryFixture(t)
	repository := NewSQLiteRepositoryWithDiagnosticRedactor(store, func(message string) string {
		return "redacted:" + message
	})
	state, err := repository.BeginIntent(context.Background(), begin)
	if err != nil {
		t.Fatal(err)
	}
	state.FailureAt = "2026-08-02T00:00:01Z"
	if err := repository.RecordFailure(context.Background(), state, preparationcontract.InvocationUnknown, &longDiagnosticError{}); err != nil {
		t.Fatal(err)
	}
	record, err := leasecontract.Decode(begin.Snapshot.Record.ID, store.mustGet(recordBucket, begin.Snapshot.Record.ID))
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution.Failure == nil || !strings.HasPrefix(record.Execution.Failure.Message, "redacted:") || len(record.Execution.Failure.Message) > 4096 {
		t.Fatalf("failure=%+v", record.Execution.Failure)
	}
}

func newOrcaRepositoryFixture(t *testing.T) (*preparationStore, *SQLiteRepository, preparationapp.OrcaBegin) {
	t.Helper()
	store := newPreparationStore()
	record := repositoryRecord("io-orca", "/repo", "199-orca")
	record.IssueURL = "https://github.com/example/repo/issues/199"
	record.BranchPrepare = json.RawMessage(`{"provider":"github","issue_url":"https://github.com/example/repo/issues/199","branch":"199-orca","base_branch":"main","base_sha":"base","link_verified":true}`)
	store.seedRecord(t, record)
	repository := NewSQLiteRepository(store)
	snapshot, err := repository.Load(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	workspace := preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: record.Repo, Root: "/repo.worktrees/199-orca", Branch: record.Branch, BaseBranch: "main", BaseHead: "base", Confirm: true, CWD: "/repo"}
	command := preparationcontract.Command{ID: record.ID, Mode: preparationcontract.ModeOrca, OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh", Confirm: true}
	decision, err := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command, Orca: preparationdomain.OrcaReadiness{Available: true, Ready: true, Provider: "github", Issue: 199}})
	if err != nil {
		t.Fatal(err)
	}
	return store, repository, preparationapp.OrcaBegin{
		Snapshot: snapshot, Command: command,
		Workspace:   workspace,
		Probe:       preparationcontract.ProbeRequest{Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh", Provider: "github", Issue: 199, Marker: "issueops-v1 lifecycle=io-orca provider=github issue=199", Workspace: workspace},
		Owner:       preparationcontract.OwnerEvidence{IssueURL: record.IssueURL, IssueBody: "body", BodySHA256: strings.Repeat("a", 64), Source: "github", Provider: "github", Issue: 199},
		OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
		Selection: leasecontract.Selection{RequestedMode: "orca", ResolvedMode: "orca", ProbeAttempted: true, ProbeAvailable: true, ProbeReady: true, ProbeCode: "ready", ReadinessFingerprint: decision.ReadinessFingerprint, SelectedAt: "2026-08-02T00:00:00Z"},
	}
}

func repositoryOrcaReceipt(stage preparationcontract.IntentStage) preparationcontract.IntentReceipt {
	workspace := &preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{SourceRoot: "/repo", Root: "/repo.worktrees/199-orca", Branch: "199-orca", BaseHead: "base", Driver: "orca", Exists: true},
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
	}
	switch stage {
	case preparationcontract.IntentStageWorktree:
		return preparationcontract.IntentReceipt{Workspace: workspace}
	case preparationcontract.IntentStageTerminal:
		return preparationcontract.IntentReceipt{TerminalPTYID: "terminal"}
	case preparationcontract.IntentStageRun:
		return preparationcontract.IntentReceipt{RunID: "run"}
	case preparationcontract.IntentStageRunBind:
		return preparationcontract.IntentReceipt{RunID: "run", RunBound: true}
	case preparationcontract.IntentStageTask:
		return preparationcontract.IntentReceipt{TaskID: "task"}
	case preparationcontract.IntentStageDispatch:
		return preparationcontract.IntentReceipt{TaskID: "task", DispatchID: "dispatch"}
	default:
		return preparationcontract.IntentReceipt{}
	}
}

type longDiagnosticError struct{}

func (*longDiagnosticError) Error() string { return strings.Repeat("x", 5000) }
