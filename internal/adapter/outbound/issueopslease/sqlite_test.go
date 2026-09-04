package issueopslease

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
)

func TestSQLiteRepositoryLeavesRecordUnchangedWhenApplyFailsAfterClock(t *testing.T) {
	actor := leasecontract.Actor{
		Host: "codex", SessionID: "apply-failure-session",
		SessionProcess: &leasecontract.ProcessReceipt{PID: 1234, StartedAt: "2026-07-29T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	record := leasecontract.Record{
		OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-apply-failure", Repo: "/source", Branch: "196-release", Phase: "implement",
		Execution: &leasecontract.Execution{
			Mode:      "direct",
			Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/canonical", Branch: "196-release", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-07-29T00:00:00Z"},
			Lease:     leasecontract.Lease{Generation: 1, Status: "active", Holder: &actor, ClaimedAt: "2026-07-29T00:00:01Z"},
		},
	}
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	index, err := json.Marshal(holderIndex{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Generation: 1, Host: actor.Host, SessionID: actor.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	store := &applyFailingStore{records: map[string][]byte{recordBucket + "\x00" + record.ID: data, holderBucket + "\x00" + holderIndexKey(actor): index}}
	clock := &applyFailureClock{at: time.Date(2026, 7, 29, 0, 1, 0, 0, time.UTC)}
	service := leaseapp.NewReleaseService(
		NewSQLiteRepository(store),
		clock,
		func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", receipt, nil
		},
		applyFailurePaths{},
	)
	_, err = service.Release(context.Background(), leaseapp.ReleaseRequest{
		ID: record.ID, Generation: 1,
		Actor:    leasedomain.Actor{Host: actor.Host, SessionID: actor.SessionID, Process: &leasedomain.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}},
		Ancestry: []leasedomain.ProcessReceipt{{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}},
		CWD:      "/canonical",
	})
	if leasecontract.FailureCodeOf(err) != leasecontract.FailurePersistence {
		t.Fatalf("apply failure=%v", err)
	}
	if clock.calls != 1 {
		t.Fatalf("clock calls=%d want=1 after validation before Apply", clock.calls)
	}
	if got := store.records[recordBucket+"\x00"+record.ID]; string(got) != string(data) {
		t.Fatalf("Apply failure changed record\nbefore=%s\nafter=%s", data, got)
	}
	if got := store.records[holderBucket+"\x00"+holderIndexKey(actor)]; string(got) != string(index) {
		t.Fatalf("Apply failure changed holder index\nbefore=%s\nafter=%s", index, got)
	}
}

type applyFailingStore struct{ records map[string][]byte }

func (s *applyFailingStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (s *applyFailingStore) Get(bucket, id string) ([]byte, bool, error) {
	data, ok := s.records[bucket+"\x00"+id]
	return append([]byte(nil), data...), ok, nil
}

func (*applyFailingStore) Apply(context.Context, []port.RecordMutation) error {
	return errors.New("injected Apply failure")
}

type applyFailureClock struct {
	at    time.Time
	calls int
}

func (c *applyFailureClock) Now() time.Time {
	c.calls++
	return c.at
}

type applyFailurePaths struct{}

func (applyFailurePaths) Matches(left, right string) bool { return left == right }
