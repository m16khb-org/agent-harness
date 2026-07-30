package issueopslease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func TestSQLiteClaimTransaction(t *testing.T) {
	token := "claim-token"
	actor := leasecontract.Actor{Host: "codex", SessionID: "claim-session", SessionProcess: &leasecontract.ProcessReceipt{PID: 42, StartedAt: "2026-07-30T00:00:00Z", Executable: "/usr/bin/codex"}}
	record := claimableRecord(t, actor, token)
	store := newClaimStore(t, record)
	path := writeClaimToken(t, record, token)
	repository := NewSQLiteRepository(store)
	result, err := repository.Claim(context.Background(), leaseapp.ClaimRepositoryRequest{
		ID: record.ID, Generation: record.Execution.Lease.Generation,
		Actor: leasedomain.Actor{Host: actor.Host, SessionID: actor.SessionID, Process: &leasedomain.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}},
		CWD:   record.Execution.Workspace.Root, TokenFile: path, Clock: fixedClaimClock{at: time.Date(2026, 7, 30, 0, 0, 1, 0, time.UTC)},
		ValidateRecord: func(leaseapp.Record) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Execution.Lease.Status != "active" || result.Execution.Lease.Holder == nil {
		t.Fatalf("claim result=%+v", result)
	}
	if result.Execution.Lease.ClaimTokenSHA256 != "" {
		t.Fatalf("token hash retained after claim: %q", result.Execution.Lease.ClaimTokenSHA256)
	}
	if _, exists := store.records[holderBucket+"\x00"+holderIndexKey(actor)]; !exists {
		t.Fatal("holder index was not applied with the record")
	}
}

func TestSQLiteClaimRejectsUnpreparedExecution(t *testing.T) {
	record := claimableRecord(t, leasecontract.Actor{}, "claim-token")
	record.Execution = nil
	_, err := NewSQLiteRepository(newClaimStore(t, record)).Claim(context.Background(), leaseapp.ClaimRepositoryRequest{ID: record.ID})
	if !errors.Is(err, leasecontract.ErrExecutionNotPrepared) {
		t.Fatalf("unprepared execution error=%v", err)
	}
}

func TestClaimTokenContract(t *testing.T) {
	record := claimableRecord(t, leasecontract.Actor{}, "claim-token")
	path := writeClaimToken(t, record, "claim-token")
	if token, err := readCurrentClaimToken(record, path); err != nil || token != "claim-token" {
		t.Fatalf("read token=%q err=%v", token, err)
	}
	if _, err := readCurrentClaimToken(record, filepath.Join(record.Execution.Workspace.Root, "wrong.token")); err == nil || err.Error() != "claim_token_file must be the deterministic current-generation path" {
		t.Fatalf("wrong token path error=%v", err)
	}
}

func claimableRecord(t *testing.T, actor leasecontract.Actor, token string) leasecontract.Record {
	t.Helper()
	root := t.TempDir()
	return leasecontract.Record{
		OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-claim-transaction", Repo: "/source", Branch: "claim", Phase: "implement",
		Execution: &leasecontract.Execution{
			Mode:      "direct",
			Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: root, Branch: "claim", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-07-30T00:00:00Z"},
			Lease:     leasecontract.Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: claimTokenSHA256(token)},
		},
	}
}

func newClaimStore(t *testing.T, record leasecontract.Record) *claimStore {
	t.Helper()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	return &claimStore{records: map[string][]byte{recordBucket + "\x00" + record.ID: data}}
}

func writeClaimToken(t *testing.T, record leasecontract.Record, token string) string {
	t.Helper()
	path := claimTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type claimStore struct{ records map[string][]byte }

type fixedClaimClock struct{ at time.Time }

func (c fixedClaimClock) Now() time.Time { return c.at }

func (s *claimStore) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (s *claimStore) Get(bucket, id string) ([]byte, bool, error) {
	data, ok := s.records[bucket+"\x00"+id]
	return append([]byte(nil), data...), ok, nil
}

func (s *claimStore) Apply(_ context.Context, mutations []port.RecordMutation) error {
	for _, mutation := range mutations {
		key := mutation.Bucket + "\x00" + mutation.ID
		if mutation.RequireAbsent {
			if _, exists := s.records[key]; exists {
				return fmt.Errorf("record %s already exists", mutation.ID)
			}
		}
	}
	for _, mutation := range mutations {
		key := mutation.Bucket + "\x00" + mutation.ID
		if mutation.Delete {
			delete(s.records, key)
			continue
		}
		s.records[key] = append([]byte(nil), mutation.Data...)
	}
	return nil
}
