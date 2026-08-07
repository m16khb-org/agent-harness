package issueopslease

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

func TestReseedServiceRejectsParentDriftBeforeArtifactsOrCommit(t *testing.T) {
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	record.Stable.BranchPrepare = json.RawMessage(`{"base_branch":"main"}`)
	before, err := cloneReseedRecord(record.Stable)
	if err != nil {
		t.Fatal(err)
	}
	commitCalls := 0
	prepareCalls := 0
	inspectorCalls := 0
	service := newReseedServiceForTestWithBaseSync(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
			commitCalls++
			return RepositoryResult{}, nil
		}},
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			prepareCalls++
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64)}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
		baseSyncInspectorFunc(func(_ context.Context, request basesyncport.Request) (basesyncport.Receipt, error) {
			inspectorCalls++
			if request.Worktree != "/worktree" || request.BaseBranch != "main" {
				t.Fatalf("base sync request=%+v", request)
			}
			return basesyncport.Receipt{BaseOID: strings.Repeat("a", 40), WorkOID: strings.Repeat("b", 40), SyncRequired: true}, nil
		}),
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	request.CompletionGeneration = 4
	_, err = service.Reseed(context.Background(), request)
	if err == nil {
		t.Fatal("drifted completed reseed was allowed")
	}
	structured, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok {
		t.Fatalf("drift rejection is not structured: %T %v", err, err)
	}
	fields := structured.IssueOpsErrorFields()
	if fields["code"] != "post_completion_sync_base_required" || fields["completion_generation"] != uint64(4) {
		t.Fatalf("drift rejection fields=%v", fields)
	}
	if inspectorCalls != 1 || commitCalls != 0 || prepareCalls != 0 {
		t.Fatalf("calls inspector=%d commit=%d prepare=%d", inspectorCalls, commitCalls, prepareCalls)
	}
	if !reflect.DeepEqual(record.Stable, before) {
		t.Fatalf("drift rejection mutated loaded state: before=%+v after=%+v", before, record.Stable)
	}
}

func TestReseedServiceReopensCompletedExecutionAtomically(t *testing.T) {
	oldHead := "d6d8c6a5a98fcca6bca33edf9e7965636429ce28"
	newHead := "ff27b34520e4e253d8ebfd523e4e4352bf93e8d8"
	record := completedReseedTestRecord(oldHead)
	var prepared leasecontract.Record
	var committed leasecontract.Record
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
			committed = next.Stable
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedArtifactsFake{prepare: func(_ context.Context, next leasecontract.Record) (ReseedArtifactReceipt, error) {
			prepared = next
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64), Receipt: leasecontract.ReseedReceipt{ClaimTokenPath: "/worktree/token"}}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	request.Reason = "functional HEAD moved to " + newHead
	result, err := service.Reseed(context.Background(), request)
	if err != nil {
		t.Fatalf("reseed completed execution: %v", err)
	}
	for name, candidate := range map[string]leasecontract.Record{"prepared": prepared, "committed": committed} {
		if candidate.Execution.Completion != nil || candidate.Phase != "implement" {
			t.Fatalf("%s current completion=%+v phase=%q", name, candidate.Execution.Completion, candidate.Phase)
		}
		if len(candidate.Execution.CompletionHistory) != 1 {
			t.Fatalf("%s history=%+v", name, candidate.Execution.CompletionHistory)
		}
		entry := candidate.Execution.CompletionHistory[0]
		if entry.Generation != 4 || entry.Completion.FinalHead != oldHead || entry.Reason != request.Reason || entry.ReopenedAt != "2026-07-30T09:00:00Z" {
			t.Fatalf("%s history entry=%+v", name, entry)
		}
		assertCompletedReseedProofCleared(t, candidate)
		assertCompletedReseedHistoryPreserved(t, candidate)
		assertCompletedReseedLedgerStale(t, candidate)
	}
	if result.Execution.Lease.Generation != 5 || result.Execution.Completion != nil || result.Execution.CompletionHistory[0].Completion.FinalHead != oldHead {
		t.Fatalf("result execution=%+v", result.Execution)
	}
}

func TestReseedServiceArchivesCompletionAtItsOriginGeneration(t *testing.T) {
	record := completedReseedTestRecord(strings.Repeat("e", 40))
	record.Stable.Execution.Lease.Generation = 5
	record.Stable.Execution.Completion.Generation = 5
	record.Lease = record.Stable.Execution.Lease
	record.Stable.Execution.CompletionHistory = []leasecontract.CompletionHistoryEntry{{
		Generation: 1,
		Completion: leasecontract.Completion{FinalHead: strings.Repeat("a", 40), TuringReportPath: ".agent-harness/turing/prior.json", Verification: []string{"prior verification"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-07-01T00:00:00Z"},
		Reason:     "prior reopen", ReopenedAt: "2026-07-02T00:00:00Z",
	}}
	var committed leasecontract.Record
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
			committed = next.Stable
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedArtifactsFake{prepare: func(_ context.Context, _ leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("c", 64)}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 5
	request.Reason = "functional HEAD moved"
	if _, err := service.Reseed(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(committed.Execution.CompletionHistory) != 2 || committed.Execution.CompletionHistory[1].Generation != 5 {
		t.Fatalf("history=%+v", committed.Execution.CompletionHistory)
	}
	if got := committed.Execution.CompletionHistory[0].Completion.FinalHead; got != strings.Repeat("a", 40) {
		t.Fatalf("prior history changed: %+v", committed.Execution.CompletionHistory[0])
	}
}

func TestReseedServiceRejectsMissingStampedCompletionGeneration(t *testing.T) {
	record := completedReseedTestRecord("d6d8c6a5a98fcca6bca33edf9e7965636429ce28")
	record.Stable.Execution.Lease.Generation = 5
	record.Stable.Execution.Lease.ReplacedAt = "2026-08-03T17:58:40.077656Z"
	record.Stable.Execution.Completion.Generation = 0
	record.Stable.Execution.Completion.CompletedAt = "2026-08-03T17:41:13.488177Z"
	record.Lease = record.Stable.Execution.Lease
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
			t.Fatal("commit must not run")
			return RepositoryResult{}, nil
		}},
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			t.Fatal("prepare must not run")
			return ReseedArtifactReceipt{}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	for _, selected := range []uint64{4, 0} {
		request := reseedServiceRequest(record.ID)
		request.ExpectedGeneration = 5
		request.CompletionGeneration = selected
		if _, err := service.Reseed(context.Background(), request); err == nil || err.Error() != "invalid or missing stamped completion generation" {
			t.Fatalf("selected=%d invalid completion error=%v", selected, err)
		}
	}
}

func TestReseedServiceRejectsConflictingCompletionProvenance(t *testing.T) {
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
			t.Fatal("commit must not run")
			return RepositoryResult{}, nil
		}},
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			t.Fatal("prepare must not run")
			return ReseedArtifactReceipt{}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	request.CompletionGeneration = 3
	if _, err := service.Reseed(context.Background(), request); err == nil || err.Error() != "completion_generation conflicts with stamped completion generation 4" {
		t.Fatalf("conflicting provenance error=%v", err)
	}
}

func TestReseedServiceCommitFailureDoesNotPersistCompletedReopen(t *testing.T) {
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	before, err := cloneReseedRecord(record.Stable)
	if err != nil {
		t.Fatal(err)
	}
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
			return RepositoryResult{}, errReseedCommit
		}},
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64)}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	if _, err := service.Reseed(context.Background(), request); !errors.Is(err, errReseedCommit) {
		t.Fatalf("commit error=%v", err)
	}
	if !reflect.DeepEqual(record.Stable, before) {
		t.Fatal("commit failure mutated the loaded completed snapshot")
	}
}

func TestReseedServicePreservesCompletionFreeSemantics(t *testing.T) {
	record := completedReseedTestRecord("9c8db06313cfce39d17a53123f84da1fc4bc7b34")
	record.Stable.Execution.Completion = nil
	before := record.Stable
	var committed leasecontract.Record
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
			committed = next.Stable
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedArtifactsFake{prepare: func(_ context.Context, _ leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64)}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	if _, err := service.Reseed(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if committed.Phase != before.Phase || !reflect.DeepEqual(committed.PhaseLedger, before.PhaseLedger) || committed.AISlopCleanHead != before.AISlopCleanHead || !reflect.DeepEqual(committed.ImplementationReview, before.ImplementationReview) || !reflect.DeepEqual(committed.RemoteCompletion, before.RemoteCompletion) {
		t.Fatalf("completion-free reseed changed existing semantics: before=%+v after=%+v", before, committed)
	}
}

func TestReseedServicePrepareFailureDoesNotMutateCompletedSnapshot(t *testing.T) {
	record := completedReseedTestRecord(strings.Repeat("d", 40))
	before, err := cloneReseedRecord(record.Stable)
	if err != nil {
		t.Fatal(err)
	}
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error) {
			t.Fatal("commit must not run")
			return RepositoryResult{}, nil
		}},
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{}, errors.New("prepare failed")
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	request.ExpectedGeneration = 4
	if _, err := service.Reseed(context.Background(), request); err == nil || err.Error() != "prepare failed" {
		t.Fatalf("prepare error=%v", err)
	}
	if !reflect.DeepEqual(record.Stable, before) {
		t.Fatal("prepare failure mutated the loaded completed snapshot")
	}
}

func completedReseedTestRecord(oldHead string) Record {
	record := reseedTestRecord("released", 4)
	record.Stable.Phase = "done"
	record.Stable.BranchPrepare = json.RawMessage(`{"base_branch":"main"}`)
	record.Stable.Execution.Completion = &leasecontract.Completion{
		Generation: 4, FinalHead: oldHead, TuringReportPath: ".agent-harness/turing/old.json", Verification: []string{"old verification"},
		RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-07-29T09:00:00Z",
	}
	record.Stable.Execution.SyncBaseEvents = []leasecontract.SyncBaseEvent{{Mode: "apply", BaseBranch: "main", BaseOID: strings.Repeat("a", 40), MergeCommit: strings.Repeat("b", 40), Actor: "codex", At: "2026-07-29T10:00:00Z"}}
	record.Stable.RemoteArtifact = json.RawMessage(`{"provider":"github","kind":"draft_pr","url":"https://github.com/acme/repo/pull/1"}`)
	record.Stable.Feedback = json.RawMessage(`[{"source":"review","body":"keep"}]`)
	record.Stable.Decisions = json.RawMessage(`[{"title":"keep"}]`)
	record.Stable.RemoteCompletion = json.RawMessage(`{"reflected_at":"old"}`)
	record.Stable.ImplementationReview = json.RawMessage(`{"verdict":"pass"}`)
	record.Stable.AISlopCleanAt = "old-at"
	record.Stable.AISlopCleanHead = oldHead
	record.Stable.AISlopCleanFingerprint = "old-fingerprint"
	record.Stable.AISlopCleanCategories = json.RawMessage(`["duplication"]`)
	record.Stable.AISlopCleanVerification = json.RawMessage(`["old verification"]`)
	record.Stable.PhaseLedger = json.RawMessage(`{"problem":{"phase":"problem","entered_at":"p0","completed_at":"p1","notes":["keep upstream"]},"implement":{"phase":"implement","entered_at":"i0","completed_at":"i1","artifacts":["old implementation"]},"ai-slop-clean":{"phase":"ai-slop-clean","entered_at":"a0","completed_at":"a1"},"feedback":{"phase":"feedback","entered_at":"f0","completed_at":"f1"},"pr":{"phase":"pr","entered_at":"r0","completed_at":"r1","notes":["keep pr"]},"done":{"phase":"done","entered_at":"d0","completed_at":"d1"}}`)
	return record
}

func assertCompletedReseedProofCleared(t *testing.T, record leasecontract.Record) {
	t.Helper()
	if record.AISlopCleanAt != "" || record.AISlopCleanHead != "" || record.AISlopCleanFingerprint != "" || record.AISlopCleanCategories != nil || record.AISlopCleanVerification != nil || record.ImplementationReview != nil || record.RemoteCompletion != nil {
		t.Fatalf("current proof not cleared: %+v", record)
	}
}

func assertCompletedReseedHistoryPreserved(t *testing.T, record leasecontract.Record) {
	t.Helper()
	if record.RemoteArtifact == nil || record.Feedback == nil || record.Decisions == nil || len(record.Execution.SyncBaseEvents) != 1 {
		t.Fatalf("historical sidecars not preserved: %+v", record)
	}
}

func assertCompletedReseedLedgerStale(t *testing.T, record leasecontract.Record) {
	t.Helper()
	var ledger map[string]struct {
		CompletedAt string   `json:"completed_at"`
		Notes       []string `json:"notes"`
	}
	if err := json.Unmarshal(record.PhaseLedger, &ledger); err != nil {
		t.Fatal(err)
	}
	if ledger["problem"].CompletedAt != "p1" || !reflect.DeepEqual(ledger["problem"].Notes, []string{"keep upstream"}) {
		t.Fatalf("upstream ledger changed: %+v", ledger["problem"])
	}
	wantNote := "stale: completed execution reseed (4 -> 5)"
	for _, phase := range []string{"implement", "ai-slop-clean", "feedback", "pr", "done"} {
		if ledger[phase].CompletedAt != "" || !containsString(ledger[phase].Notes, wantNote) {
			t.Fatalf("phase %s not stale: %+v", phase, ledger[phase])
		}
	}
	if !containsString(ledger["pr"].Notes, "keep pr") {
		t.Fatalf("prior pr note lost: %+v", ledger["pr"])
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestReseedServiceOrdersPrepareCommitAndBestEffortCleanup(t *testing.T) {
	trace := []string{}
	record := reseedTestRecord("released", 3)
	service := NewReseedService(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			trace = append(trace, "fence")
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, snapshot ReseedSnapshot, next Record) (RepositoryResult, error) {
			trace = append(trace, "commit")
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedInventoryFunc(func(_ context.Context, _ leasecontract.Record, _ leasedomain.Actor) (ReseedInventoryReceipt, error) {
			trace = append(trace, "inventory")
			return ReseedInventoryReceipt{Fingerprint: "current"}, nil
		}),
		baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
			return basesyncport.Receipt{}, nil
		}),
		reseedArtifactsFake{prepare: func(_ context.Context, next leasecontract.Record) (ReseedArtifactReceipt, error) {
			trace = append(trace, "prepare")
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64), Receipt: leasecontract.ReseedReceipt{ClaimTokenPath: "/worktree/token"}}, nil
		}, cleanup: func(_ context.Context, previous leasecontract.Record) error {
			trace = append(trace, "cleanup")
			if previous.Execution.Lease.Generation != 3 {
				t.Fatalf("superseded cleanup generation=%d want=3", previous.Execution.Lease.Generation)
			}
			return context.DeadlineExceeded
		}},
		fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)},
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
		},
		reseedPathMatcher{},
	)
	result, err := service.Reseed(context.Background(), ReseedRequest{ID: record.ID, ExpectedGeneration: 3, Actor: reseedTestActor(), Ancestry: []leasedomain.ProcessReceipt{{PID: 1, StartedAt: "start", Executable: "codex"}}, CWD: "/worktree", InventoryFingerprint: "current", Confirm: true})
	if err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if !result.OK || result.Execution.Lease.Generation != 4 || result.Execution.Lease.Status != "claimable" {
		t.Fatalf("result=%+v", result)
	}
	if got := strings.Join(trace, ","); got != "fence,inventory,prepare,commit,cleanup" {
		t.Fatalf("trace=%s", got)
	}
}

func TestReseedServicePersistsOrcaArtifactIdentityBeforeCommit(t *testing.T) {
	record := reseedTestRecord("claimable", 3)
	record.Stable.Execution.Mode = "orca"
	record.Stable.Execution.Workspace.Driver = "orca"
	record.Stable.Execution.Orca = &leasecontract.OrcaBinding{
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 3,
		OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch",
	}
	var committed leasecontract.OrcaBinding
	service := newReseedServiceForTest(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
			committed = *next.Stable.Execution.Orca
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedArtifactsFake{prepare: func(_ context.Context, _ leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{
				TokenSHA256: strings.Repeat("d", 64),
				Receipt: leasecontract.ReseedReceipt{
					IssueBodySHA256: strings.Repeat("a", 64), ContextPacketSHA256: strings.Repeat("b", 64), OwnerPromptSHA256: strings.Repeat("c", 64),
				},
			}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
	)
	request := reseedServiceRequest(record.ID)
	if _, err := service.Reseed(context.Background(), request); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if committed.IssueBodySHA256 != strings.Repeat("a", 64) || committed.ContextPacketSHA256 != strings.Repeat("b", 64) || committed.OwnerPromptSHA256 != strings.Repeat("c", 64) {
		t.Fatalf("committed Orca artifact identity=%+v", committed)
	}
	if committed.ArtifactIdentityVersion != leasecontract.OrcaArtifactIdentityVersion {
		t.Fatalf("committed artifact identity version=%d want=%d", committed.ArtifactIdentityVersion, leasecontract.OrcaArtifactIdentityVersion)
	}
	if committed.LeaseGeneration != 4 {
		t.Fatalf("reseeded binding generation=%d want=4", committed.LeaseGeneration)
	}
}

func TestReseedServicePersistsSettledHolderlessRuntimeRollover(t *testing.T) {
	record := reseedTestRecord("claimable", 3)
	record.Stable.Execution.Mode = "orca"
	record.Stable.Execution.Workspace.Driver = "orca"
	record.Stable.Execution.Orca = &leasecontract.OrcaBinding{
		RuntimeID: "runtime-old", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 3,
		OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch",
	}
	var committed leasecontract.OrcaBinding
	service := NewReseedService(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}, commit: func(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
			committed = *next.Stable.Execution.Orca
			return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
		}},
		reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
			return ReseedInventoryReceipt{
				Fingerprint: "current", RuntimeID: "runtime-current",
				Inventory: leasedomain.ResumeInventory{RuntimeID: "runtime-current", TerminalInventoryComplete: true, TaskStatus: "failed", DispatchStatus: "dispatched", DispatchAssigneeHandle: "term-old"},
			}, nil
		}),
		baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
			return basesyncport.Receipt{}, nil
		}),
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("d", 64)}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
		fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)},
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
		},
		reseedPathMatcher{},
	)
	if _, err := service.Reseed(context.Background(), reseedServiceRequest(record.ID)); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	if committed.RuntimeID != "runtime-current" || committed.LeaseGeneration != 4 {
		t.Fatalf("reseeded Orca binding=%+v", committed)
	}
}

func TestReseedServiceRejectsChangedOwnerEvidenceFingerprintBeforePrepare(t *testing.T) {
	record := reseedTestRecord("claimable", 3)
	record.Stable.Execution.Mode = "orca"
	record.Stable.Execution.Workspace.Driver = "orca"
	record.Stable.Execution.Orca = &leasecontract.OrcaBinding{RuntimeID: "runtime-old", WorktreeID: "worktree", TaskID: "task", DispatchID: "dispatch"}
	prepares := 0
	service := NewReseedService(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}},
		reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
			return ReseedInventoryReceipt{
				Fingerprint: "changed-owner-evidence",
				Inventory: leasedomain.ResumeInventory{
					RuntimeID: "runtime-current", TerminalInventoryComplete: true, TaskStatus: "failed", DispatchStatus: "pending", DispatchAssigneeHandle: "term-old",
				},
			}, nil
		}),
		baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
			return basesyncport.Receipt{}, nil
		}),
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			prepares++
			return ReseedArtifactReceipt{}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
		fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)},
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
		},
		reseedPathMatcher{},
	)
	request := reseedServiceRequest(record.ID)
	request.InventoryFingerprint = "preview-owner-evidence"
	if _, err := service.Reseed(context.Background(), request); err == nil || !strings.Contains(err.Error(), "stale replacement inventory fingerprint") {
		t.Fatalf("reseed error=%v", err)
	}
	if prepares != 0 {
		t.Fatalf("changed owner evidence reached artifact preparation %d times", prepares)
	}
}

func TestReseedServiceRejectsUnsettledRolloverAfterFingerprintMatch(t *testing.T) {
	record := reseedTestRecord("claimable", 3)
	record.Stable.Execution.Mode = "orca"
	record.Stable.Execution.Workspace.Driver = "orca"
	record.Stable.Execution.Orca = &leasecontract.OrcaBinding{RuntimeID: "runtime-old", WorktreeID: "worktree", TaskID: "task", DispatchID: "dispatch"}
	prepares := 0
	service := NewReseedService(
		reseedFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		reseedRepositoryFake{snapshot: ReseedSnapshot{Record: record}},
		reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
			return ReseedInventoryReceipt{
				Fingerprint: "current",
				Inventory: leasedomain.ResumeInventory{
					RuntimeID: "runtime-current", TerminalInventoryComplete: true, TaskStatus: "failed", DispatchStatus: "pending", DispatchAssigneeHandle: "term-old",
				},
			}, nil
		}),
		baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
			return basesyncport.Receipt{}, nil
		}),
		reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
			prepares++
			return ReseedArtifactReceipt{}, nil
		}, cleanup: func(context.Context, leasecontract.Record) error { return nil }},
		fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)},
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
		},
		reseedPathMatcher{},
	)
	if _, err := service.Reseed(context.Background(), reseedServiceRequest(record.ID)); err == nil || !strings.Contains(err.Error(), "resume_runtime_identity") {
		t.Fatalf("reseed error=%v", err)
	}
	if prepares != 0 {
		t.Fatalf("unsettled rollover reached artifact preparation %d times", prepares)
	}
}

func TestReseedServiceSameLifecycleSuccessMakesSecondAttemptStaleBeforePrepare(t *testing.T) {
	repository := &serializedReseedRepository{record: reseedTestRecord("released", 3)}
	fence := &serializedReseedFence{locks: map[string]*sync.Mutex{}}
	prepareEntered := make(chan struct{})
	allowFirstPrepare := make(chan struct{})
	var prepares int
	artifacts := reseedArtifactsFake{prepare: func(_ context.Context, record leasecontract.Record) (ReseedArtifactReceipt, error) {
		prepares++
		if prepares == 1 {
			close(prepareEntered)
			<-allowFirstPrepare
		}
		return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64), Receipt: leasecontract.ReseedReceipt{ClaimTokenPath: "/worktree/token"}}, nil
	}, cleanup: func(context.Context, leasecontract.Record) error { return nil }}
	service := newReseedServiceForTest(fence, repository, artifacts)
	request := reseedServiceRequest("io-reseed-test")
	results := make(chan error, 2)
	go func() { _, err := service.Reseed(context.Background(), request); results <- err }()
	<-prepareEntered
	go func() { _, err := service.Reseed(context.Background(), request); results <- err }()
	select {
	case <-time.After(50 * time.Millisecond):
	case err := <-results:
		t.Fatalf("second reseed escaped the lifecycle fence early: %v", err)
	}
	close(allowFirstPrepare)
	first := <-results
	second := <-results
	if first != nil && second != nil {
		t.Fatalf("both attempts failed: first=%v second=%v", first, second)
	}
	stale := second
	if stale == nil {
		stale = first
	}
	if stale == nil || !strings.Contains(stale.Error(), "stale lease generation: current=4 expected=3") || prepares != 1 {
		t.Fatalf("stale=%v prepares=%d", stale, prepares)
	}
}

func TestReseedServiceCompensatedCommitFailureAllowsRetry(t *testing.T) {
	repository := &serializedReseedRepository{record: reseedTestRecord("released", 3), commitFailures: 1}
	rollback := 0
	artifacts := reseedArtifactsFake{prepare: func(_ context.Context, record leasecontract.Record) (ReseedArtifactReceipt, error) {
		return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64), Receipt: leasecontract.ReseedReceipt{ClaimTokenPath: "/worktree/token"}}, nil
	}, rollback: func(context.Context, ReseedArtifactReceipt) error { rollback++; return nil }, cleanup: func(context.Context, leasecontract.Record) error { return nil }}
	service := newReseedServiceForTest(&serializedReseedFence{locks: map[string]*sync.Mutex{}}, repository, artifacts)
	request := reseedServiceRequest("io-reseed-test")
	if _, err := service.Reseed(context.Background(), request); !errors.Is(err, errReseedCommit) {
		t.Fatalf("first reseed error=%v", err)
	}
	result, err := service.Reseed(context.Background(), request)
	if err != nil || !result.OK || result.Execution.Lease.Generation != 4 || rollback != 1 {
		t.Fatalf("retry result=%+v err=%v rollback=%d", result, err, rollback)
	}
}

func TestReseedServiceValidatesActorBeforeConfirmForLegacyErrorPriority(t *testing.T) {
	service := newReseedServiceForTest(&serializedReseedFence{locks: map[string]*sync.Mutex{}}, &serializedReseedRepository{record: reseedTestRecord("released", 3)}, reseedArtifactsFake{prepare: func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error) {
		return ReseedArtifactReceipt{}, nil
	}, cleanup: func(context.Context, leasecontract.Record) error { return nil }})
	request := reseedServiceRequest("io-reseed-test")
	request.Confirm = false
	request.Actor.Host = "unknown"
	if _, err := service.Reseed(context.Background(), request); err == nil || err.Error() != "native actor host must be codex or claude" {
		t.Fatalf("confirm/actor error priority=%v", err)
	}
}

func TestReseedServiceDifferentLifecyclesPrepareInParallel(t *testing.T) {
	fence := &serializedReseedFence{locks: map[string]*sync.Mutex{}}
	entered := make(chan string, 2)
	release := make(chan struct{})
	artifacts := reseedArtifactsFake{prepare: func(_ context.Context, record leasecontract.Record) (ReseedArtifactReceipt, error) {
		entered <- record.ID
		<-release
		return ReseedArtifactReceipt{TokenSHA256: strings.Repeat("a", 64), Receipt: leasecontract.ReseedReceipt{ClaimTokenPath: "/worktree/token"}}, nil
	}, cleanup: func(context.Context, leasecontract.Record) error { return nil }}
	first := newReseedServiceForTest(fence, &serializedReseedRepository{record: reseedTestRecordWithID("io-reseed-first", "released", 3)}, artifacts)
	second := newReseedServiceForTest(fence, &serializedReseedRepository{record: reseedTestRecordWithID("io-reseed-second", "released", 3)}, artifacts)
	errs := make(chan error, 2)
	go func() {
		_, err := first.Reseed(context.Background(), reseedServiceRequest("io-reseed-first"))
		errs <- err
	}()
	go func() {
		_, err := second.Reseed(context.Background(), reseedServiceRequest("io-reseed-second"))
		errs <- err
	}()
	seen := map[string]bool{}
	for range 2 {
		select {
		case id := <-entered:
			seen[id] = true
		case <-time.After(time.Second):
			t.Fatalf("different lifecycle reseed did not enter prepare in parallel: %v", seen)
		}
	}
	if !seen["io-reseed-first"] || !seen["io-reseed-second"] {
		t.Fatalf("prepare lifecycles=%v", seen)
	}
	close(release)
	if err := <-errs; err != nil {
		t.Fatalf("first reseed: %v", err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("second reseed: %v", err)
	}
}

func reseedTestRecord(status string, generation uint64) Record {
	return reseedTestRecordWithID("io-reseed-test", status, generation)
}

func reseedTestRecordWithID(id, status string, generation uint64) Record {
	stable := leasecontract.Record{ID: id, Execution: &leasecontract.Execution{Mode: "direct", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base", Driver: "git", LinkedAt: "2026-07-30T09:00:00Z"}, Lease: leasecontract.Lease{Generation: generation, Status: status}}}
	if status == "claimable" {
		stable.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("b", 64)
	}
	return Record{ID: stable.ID, SourceRoot: "/source", CanonicalRoot: "/worktree", Lease: stable.Execution.Lease, Stable: stable}
}

func reseedTestActor() leasedomain.Actor {
	return leasedomain.Actor{Host: "codex", SessionID: "session", Process: &leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}}
}

type reseedFenceFunc func(context.Context, string, func(context.Context) error) error

func (f reseedFenceFunc) Within(ctx context.Context, id string, fn func(context.Context) error) error {
	return f(ctx, id, fn)
}

type reseedInventoryFunc func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error)

func (f reseedInventoryFunc) Observe(ctx context.Context, record leasecontract.Record, actor leasedomain.Actor) (ReseedInventoryReceipt, error) {
	return f(ctx, record, actor)
}

type baseSyncInspectorFunc func(context.Context, basesyncport.Request) (basesyncport.Receipt, error)

func (f baseSyncInspectorFunc) Observe(ctx context.Context, request basesyncport.Request) (basesyncport.Receipt, error) {
	return f(ctx, request)
}

type reseedRepositoryFake struct {
	snapshot ReseedSnapshot
	commit   func(context.Context, ReseedSnapshot, Record) (RepositoryResult, error)
}

func (f reseedRepositoryFake) LoadSnapshot(context.Context, string) (ReseedSnapshot, error) {
	return f.snapshot, nil
}
func (f reseedRepositoryFake) CommitReseed(ctx context.Context, snapshot ReseedSnapshot, record Record) (RepositoryResult, error) {
	return f.commit(ctx, snapshot, record)
}

type reseedArtifactsFake struct {
	prepare  func(context.Context, leasecontract.Record) (ReseedArtifactReceipt, error)
	rollback func(context.Context, ReseedArtifactReceipt) error
	cleanup  func(context.Context, leasecontract.Record) error
}

func (f reseedArtifactsFake) Prepare(ctx context.Context, record leasecontract.Record) (ReseedArtifactReceipt, error) {
	return f.prepare(ctx, record)
}
func (f reseedArtifactsFake) Rollback(ctx context.Context, receipt ReseedArtifactReceipt) error {
	if f.rollback == nil {
		return nil
	}
	return f.rollback(ctx, receipt)
}
func (f reseedArtifactsFake) CleanupSuperseded(ctx context.Context, record leasecontract.Record) error {
	return f.cleanup(ctx, record)
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type reseedPathMatcher struct{}

func (reseedPathMatcher) Matches(left, right string) bool { return left == right }

type serializedReseedFence struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (f *serializedReseedFence) Within(ctx context.Context, id string, fn func(context.Context) error) error {
	f.mu.Lock()
	lock := f.locks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		f.locks[id] = lock
	}
	f.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return fn(ctx)
}

var errReseedCommit = errors.New("commit failed")

type serializedReseedRepository struct {
	mu             sync.Mutex
	record         Record
	commitFailures int
}

func (r *serializedReseedRepository) LoadSnapshot(context.Context, string) (ReseedSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ReseedSnapshot{Record: r.record}, nil
}

func (r *serializedReseedRepository) CommitReseed(_ context.Context, _ ReseedSnapshot, next Record) (RepositoryResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.commitFailures > 0 {
		r.commitFailures--
		return RepositoryResult{}, errReseedCommit
	}
	r.record = next
	return RepositoryResult{Record: next, Execution: *next.Stable.Execution}, nil
}

func newReseedServiceForTest(fence ReseedFence, repository ReseedRepository, artifacts ReseedArtifacts) *ReseedService {
	return NewReseedService(fence, repository, reseedInventoryFunc(func(_ context.Context, record leasecontract.Record, _ leasedomain.Actor) (ReseedInventoryReceipt, error) {
		inventory := leasedomain.ResumeInventory{}
		if record.Execution != nil && record.Execution.Orca != nil {
			inventory.RuntimeID = record.Execution.Orca.RuntimeID
		}
		return ReseedInventoryReceipt{Fingerprint: "current", Inventory: inventory}, nil
	}), baseSyncInspectorFunc(func(context.Context, basesyncport.Request) (basesyncport.Receipt, error) {
		return basesyncport.Receipt{}, nil
	}), artifacts, fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)}, func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
	}, reseedPathMatcher{})
}

func newReseedServiceForTestWithBaseSync(fence ReseedFence, repository ReseedRepository, artifacts ReseedArtifacts, baseSync basesyncport.Inspector) *ReseedService {
	return NewReseedService(fence, repository, reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
		return ReseedInventoryReceipt{Fingerprint: "current"}, nil
	}), baseSync, artifacts, fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)}, func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
	}, reseedPathMatcher{})
}

func reseedServiceRequest(id string) ReseedRequest {
	return ReseedRequest{ID: id, ExpectedGeneration: 3, Actor: reseedTestActor(), Ancestry: []leasedomain.ProcessReceipt{{PID: 1, StartedAt: "start", Executable: "codex"}}, CWD: "/worktree", InventoryFingerprint: "current", Confirm: true}
}
