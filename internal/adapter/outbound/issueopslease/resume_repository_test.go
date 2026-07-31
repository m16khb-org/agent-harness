package issueopslease

import (
	"context"
	"fmt"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/sqlstore"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func TestResumeRepositoryLoadsExactGenerationSnapshot(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewResumeRepository(store, resumeEffectsFake{})

	snapshot, err := repository.LoadSnapshot(context.Background(), "io-resume-repository", 4)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Record.Lease.Generation != 4 || len(snapshot.Raw) == 0 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestResumeRepositoryPropagatesBridgeBeginFailure(t *testing.T) {
	_, store := newResumeRepositoryStore(t, resumeRepositoryRecord(t, 4))
	repository := NewResumeRepository(store, resumeEffectsFake{beginErr: fmt.Errorf("stale raw record snapshot")})
	snapshot, err := repository.LoadSnapshot(context.Background(), "io-resume-repository", 4)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.BeginIntent(context.Background(), snapshot, leasecontract.ResumeArtifacts{}, resumeRepositoryPlan(), strings.Repeat("a", 32))
	if err == nil || !strings.Contains(err.Error(), "stale raw record snapshot") {
		t.Fatalf("begin error=%v", err)
	}
}

func newResumeRepositoryStore(t *testing.T, record leasecontract.Record) (string, *sqlstore.DB) {
	t.Helper()
	stateRoot := t.TempDir()
	store, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(context.Background(), []port.RecordMutation{resumeRepositoryMutation(t, record)}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, store
}

func resumeRepositoryMutation(t *testing.T, record leasecontract.Record) port.RecordMutation {
	t.Helper()
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	return port.RecordMutation{Bucket: recordBucket, ID: record.ID, Data: data}
}

func resumeRepositoryRecord(t *testing.T, generation uint64) leasecontract.Record {
	t.Helper()
	return leasecontract.Record{OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-resume-repository", Repo: "m16khb/agent-harness", IssueURL: "https://github.com/m16khb/agent-harness/issues/193", Phase: "implement", CreatedAt: "2026-07-31T00:00:00Z", UpdatedAt: "2026-07-31T00:00:00Z", Execution: &leasecontract.Execution{
		Mode: "orca", Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "193-resume", BaseHead: "c30fb6761a24eae102f9e79e043306e60525207d", Driver: "orca", LinkedAt: "2026-07-31T00:00:00Z"},
		Lease: leasecontract.Lease{Generation: generation, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
		Orca:  &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty", LeaseGeneration: generation},
	}}
}

func resumeRepositoryPlan() leasedomain.ResumePlan {
	return leasedomain.ResumePlan{Disposition: leasedomain.ResumeCreateTerminal, RuntimeID: "runtime"}
}

type resumeEffectsFake struct{ beginErr error }

func (f resumeEffectsFake) Begin(context.Context, leasecontract.Record, []byte, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeEffectState, error) {
	return ResumeEffectState{}, f.beginErr
}
func (resumeEffectsFake) Read(context.Context, string, string) (ResumeEffectState, error) {
	return ResumeEffectState{}, nil
}
func (resumeEffectsFake) MarkInvoking(context.Context, ResumeEffectState) (ResumeEffectState, error) {
	return ResumeEffectState{}, nil
}
func (resumeEffectsFake) RecordFailure(context.Context, ResumeEffectState, string, error) error {
	return nil
}
func (resumeEffectsFake) ApplyReceipt(context.Context, ResumeEffectState, leasecontract.ResumeStageReceipt) (ResumeEffectState, error) {
	return ResumeEffectState{}, nil
}
