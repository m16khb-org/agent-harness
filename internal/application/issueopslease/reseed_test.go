package issueopslease

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

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
	if committed.LeaseGeneration != 3 {
		t.Fatalf("previous owner binding generation=%d want=3", committed.LeaseGeneration)
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
	return NewReseedService(fence, repository, reseedInventoryFunc(func(context.Context, leasecontract.Record, leasedomain.Actor) (ReseedInventoryReceipt, error) {
		return ReseedInventoryReceipt{Fingerprint: "current"}, nil
	}), artifacts, fixedClock{now: time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)}, func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}, nil
	}, reseedPathMatcher{})
}

func reseedServiceRequest(id string) ReseedRequest {
	return ReseedRequest{ID: id, ExpectedGeneration: 3, Actor: reseedTestActor(), Ancestry: []leasedomain.ProcessReceipt{{PID: 1, StartedAt: "start", Executable: "codex"}}, CWD: "/worktree", InventoryFingerprint: "current", Confirm: true}
}
