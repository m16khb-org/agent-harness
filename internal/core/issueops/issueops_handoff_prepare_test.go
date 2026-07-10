package issueops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestWorktreePrepareAutoProbeFailurePreservesLegacyInlineResult(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probeErr: errors.New("orca unavailable")}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "auto", Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("PrepareIssueOpsHandoffWorktree: %v", err)
	}
	if got.ResolvedMode != "inline" || got.FallbackCode == "" {
		t.Fatalf("expected inline fallback, got %#v", got)
	}
	if got.ID != record.ID || got.Repo != record.Repo || got.Branch != record.Branch || len(got.Command) == 0 {
		t.Fatalf("legacy inline fields changed: %#v", got)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("expected probe-only trace, got %v", client.trace)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff != nil {
		t.Fatalf("fallback must not persist handoff: %#v", persisted.ExecutionHandoff)
	}
}

func TestWorktreePrepareExplicitOrcaProbeFailureHasProbeOnlyTrace(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probeErr: errors.New("not ready")}

	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "probe") {
		t.Fatalf("expected explicit probe error, got %v", err)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("expected probe-only trace, got %v", client.trace)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff != nil {
		t.Fatal("probe failure mutated the record")
	}
}

func TestWorktreePreparePreviewNeverMutates(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Preview || got.ResolvedMode != "orca" {
		t.Fatalf("unexpected preview: %#v", got)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("preview invoked mutation: %v", client.trace)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff != nil {
		t.Fatal("preview mutated record")
	}
}

func TestWorktreePrepareReadyOrcaCreatesExactlyOnce(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	client := &prepareOrcaFake{
		probe:  port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1"},
		create: port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16},
	}
	req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true}

	first, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	second, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if client.createCalls != 1 {
		t.Fatalf("expected one create call, got %d (%v)", client.createCalls, client.trace)
	}
	if first.State != "coordinator_preparing" || second.State != first.State || first.Orca == nil || first.Orca.WorktreeID != "wt-1" {
		t.Fatalf("unexpected results: first=%#v second=%#v", first, second)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.WorktreePath != worktree || persisted.ExecutionHandoff == nil || persisted.ExecutionHandoff.PendingOperation != nil {
		t.Fatalf("unexpected persisted record: %#v", persisted.ExecutionHandoff)
	}
}

func TestWorktreePrepareCrashAfterInvocationNeverCreatesTwice(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true}

	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("expected ambiguous invocation error")
	}
	second, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("repeat should return recovery status: %v", err)
	}
	if client.createCalls != 1 || second.State != "recovery_required" || second.RecoveryCode == "" {
		t.Fatalf("automatic retry occurred or recovery missing: calls=%d result=%#v", client.createCalls, second)
	}
}

func TestWorktreePrepareRejectsReturnedBranchPathOrInstanceMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*port.OrcaWorktree)
	}{
		{name: "branch", mutate: func(w *port.OrcaWorktree) { w.Branch = "refs/heads/wrong" }},
		{name: "path", mutate: func(w *port.OrcaWorktree) { w.Path += "-wrong" }},
		{name: "instance", mutate: func(w *port.OrcaWorktree) { w.InstanceID = "" }},
		{name: "lineage", mutate: func(w *port.OrcaWorktree) { w.Head = "wrong" }},
		{name: "issue", mutate: func(w *port.OrcaWorktree) { w.Issue = 17 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			worktree := handoffPrepareWorktreePath(record)
			makeGitWorktreeMarker(t, worktree)
			created := port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16}
			tt.mutate(&created)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}, create: created}

			_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true,
			}, client, handoffPrepareTestClock())
			if err == nil {
				t.Fatal("expected validation error")
			}
			persisted, _ := ReadIssueOps(stateRoot, record.ID)
			if persisted.ExecutionHandoff == nil || persisted.ExecutionHandoff.State != "recovery_required" {
				t.Fatalf("expected recovery_required, got %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestWorktreePrepareExactOneMarkerRecovery(t *testing.T) {
	pending := IssueOpsExecutionHandoffPendingOperation{Kind: "worktree_create", BaselineWorktreeIDs: []string{"before"}}
	tests := []struct {
		name string
		rows []port.OrcaWorktree
		ok   bool
	}{
		{name: "zero", rows: []port.OrcaWorktree{{ID: "before"}}},
		{name: "one", rows: []port.OrcaWorktree{{ID: "before"}, {ID: "new", InstanceID: "inst", Comment: "agent-harness ownership=epoch-1 attempt=1"}}, ok: true},
		{name: "multiple", rows: []port.OrcaWorktree{{ID: "new-1", Comment: "ownership=epoch-1 attempt=1"}, {ID: "new-2", Comment: "ownership=epoch-1 attempt=1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReconcileIssueOpsHandoffWorktree(pending, "epoch-1", 1, tt.rows)
			if tt.ok && (err != nil || got.ID != "new") {
				t.Fatalf("expected unique candidate, got %#v err=%v", got, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

type prepareOrcaFake struct {
	probe       port.OrcaProbeResult
	probeErr    error
	worktrees   []port.OrcaWorktree
	create      port.OrcaWorktree
	createErr   error
	createCalls int
	trace       []string
}

func (f *prepareOrcaFake) Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	f.trace = append(f.trace, "probe")
	return f.probe, f.probeErr
}

func (f *prepareOrcaFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-list")
	return append([]port.OrcaWorktree(nil), f.worktrees...), nil
}

func (f *prepareOrcaFake) CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-create")
	f.createCalls++
	return f.create, f.createErr
}

func handoffPrepareRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record := IssueOpsRecord{
		SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID:            NewIssueOpsID(repo, "16-demo"),
		Repo:          repo,
		Branch:        "16-demo",
		Phase:         IssueOpsPhasePlan,
		IssueURL:      "https://github.com/acme/repo/issues/16",
		DesignReview:  &IssueOpsDesignReview{Approved: true, ReviewedAt: "2026-07-11T00:00:00Z"},
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: "16-demo", BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true, CreatedAt: "2026-07-11T00:00:00Z"},
		CreatedAt:     "2026-07-11T00:00:00Z",
		UpdatedAt:     "2026-07-11T00:00:00Z",
	}
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, got
}

func handoffPrepareWorktreePath(record IssueOpsRecord) string {
	return record.Repo + ".worktrees/" + strings.ReplaceAll(record.Branch, "/", "-")
}

func makeGitWorktreeMarker(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func handoffPrepareTestClock() IssueOpsHandoffPrepareClock {
	return IssueOpsHandoffPrepareClock{
		Now:      func() time.Time { return time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC) },
		NewEpoch: func() (string, error) { return "epoch-1", nil },
	}
}
