package issueopslease

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

func TestResumeRejectsBeforeArtifactsAndInventory(t *testing.T) {
	artifacts, owners := 0, 0
	record := resumeApplicationTestRecord(3)
	service := NewResumeService(
		resumeFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		resumeRepositoryFake{snapshot: ResumeSnapshot{Record: record}},
		resumeArtifactsFunc(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
			artifacts++
			return leasecontract.ResumeArtifacts{}, nil
		}),
		resumeOwnersFunc(func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
			owners++
			return leasedomain.ResumeInventory{}, true, nil
		}),
		resumeStagesFake{},
		resumeOperationIDsFunc(func() (string, error) { return strings.Repeat("a", 32), nil }),
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", *resumeApplicationActor().Process, nil
		},
		reseedPathMatcher{},
	)
	_, err := service.Resume(context.Background(), ResumeRequest{
		ID: record.ID, ExpectedGeneration: 2, Actor: resumeApplicationActor(),
		Ancestry: []leasedomain.ProcessReceipt{*resumeApplicationActor().Process}, CWD: "/worktree", Confirm: true,
	})
	if err == nil || !strings.Contains(err.Error(), "holderless claimable lease") {
		t.Fatalf("resume error=%v", err)
	}
	if artifacts != 0 || owners != 0 {
		t.Fatalf("precondition called artifacts=%d owners=%d", artifacts, owners)
	}
}

func TestResumeCreatesTerminalTaskDispatchInApplicationOrder(t *testing.T) {
	trace := []string{}
	record := resumeApplicationTestRecord(4)
	repository := &resumeTraceRepository{record: record, trace: &trace}
	service := NewResumeService(
		resumeFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			trace = append(trace, "fence")
			return fn(context.Background())
		}),
		repository,
		resumeArtifactsFunc(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
			trace = append(trace, "artifacts")
			return leasecontract.ResumeArtifacts{ClaimTokenPath: "token", IssueBodySHA256: strings.Repeat("a", 64), ContextPacketPath: "packet", ContextPacketSHA256: strings.Repeat("b", 64), OwnerPromptPath: "prompt", OwnerPromptSHA256: strings.Repeat("c", 64)}, nil
		}),
		resumeOwnersFunc(func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
			trace = append(trace, "owners")
			return leasedomain.ResumeInventory{RuntimeID: "runtime"}, true, nil
		}),
		resumeTraceStages{trace: &trace},
		resumeOperationIDsFunc(func() (string, error) { trace = append(trace, "operation_id"); return strings.Repeat("d", 32), nil }),
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			trace = append(trace, "actor")
			return "live", *resumeApplicationActor().Process, nil
		},
		reseedPathMatcher{},
	)
	result, err := service.Resume(context.Background(), ResumeRequest{ID: record.ID, ExpectedGeneration: 4, Actor: resumeApplicationActor(), Ancestry: []leasedomain.ProcessReceipt{*resumeApplicationActor().Process}, CWD: "/worktree", Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Receipt.Execution.Pending != nil {
		t.Fatalf("result=%+v", result)
	}
	want := []string{"fence", "actor", "snapshot", "artifacts", "owners", "operation_id", "begin", "load_terminal", "inspect_terminal", "mark_terminal", "invoke_terminal", "apply_terminal", "load_task", "inspect_task", "mark_task", "invoke_task", "apply_task", "load_dispatch", "inspect_dispatch", "mark_dispatch", "invoke_dispatch", "apply_dispatch"}
	if !slices.Equal(trace, want) {
		t.Fatalf("trace=%v\nwant=%v", trace, want)
	}
}

func TestResumePreservesLegacyInspectionAndReconcileErrors(t *testing.T) {
	tests := []struct {
		name            string
		invocationState string
		stages          resumeTraceStages
		want            string
	}{
		{
			name:   "inspect_error",
			stages: resumeTraceStages{inspectErr: errors.New("inventory unavailable")},
			want:   "Orca intent inventory is ambiguous; intent retained: inventory unavailable",
		},
		{
			name:            "unknown_invocation",
			invocationState: "unknown",
			want:            "authoritative zero cannot retry an Orca mutation whose absence was not proven; intent retained",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := []string{}
			record := resumeApplicationTestRecord(4)
			repository := &resumeTraceRepository{record: record, trace: &trace, invocationState: tt.invocationState}
			service := resumeApplicationStageService(record, repository, tt.stages)

			_, err := service.Resume(context.Background(), ResumeRequest{
				ID: record.ID, ExpectedGeneration: 4, Actor: resumeApplicationActor(),
				Ancestry: []leasedomain.ProcessReceipt{*resumeApplicationActor().Process}, CWD: "/worktree", Confirm: true,
			})
			if err == nil || err.Error() != tt.want {
				t.Fatalf("resume error=%v want=%q", err, tt.want)
			}
		})
	}
}

func resumeApplicationStageService(record Record, repository ResumeRepository, stages ResumeStageExecutor) *ResumeService {
	return NewResumeService(
		resumeFenceFunc(func(_ context.Context, _ string, fn func(context.Context) error) error {
			return fn(context.Background())
		}),
		repository,
		resumeArtifactsFunc(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
			return leasecontract.ResumeArtifacts{ClaimTokenPath: "token", IssueBodySHA256: strings.Repeat("a", 64), ContextPacketPath: "packet", ContextPacketSHA256: strings.Repeat("b", 64), OwnerPromptPath: "prompt", OwnerPromptSHA256: strings.Repeat("c", 64)}, nil
		}),
		resumeOwnersFunc(func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
			return leasedomain.ResumeInventory{RuntimeID: "runtime"}, true, nil
		}),
		stages,
		resumeOperationIDsFunc(func() (string, error) { return strings.Repeat("d", 32), nil }),
		func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", *resumeApplicationActor().Process, nil
		},
		reseedPathMatcher{},
	)
}

func resumeApplicationTestRecord(generation uint64) Record {
	stable := leasecontract.Record{ID: "io-resume-application", Execution: &leasecontract.Execution{
		Mode:      "orca",
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "193-resume", BaseHead: "base", Driver: "orca", LinkedAt: "2026-07-31T00:00:00Z"},
		Lease:     leasecontract.Lease{Generation: generation, Status: "claimable", ClaimTokenSHA256: strings.Repeat("b", 64)},
		Orca:      &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty", LeaseGeneration: generation},
	}}
	return Record{ID: stable.ID, SourceRoot: "/source", CanonicalRoot: "/worktree", Lease: stable.Execution.Lease, Stable: stable}
}

func resumeApplicationActor() leasedomain.Actor {
	return leasedomain.Actor{Host: "codex", SessionID: "resume-session", Process: &leasedomain.ProcessReceipt{PID: 1, StartedAt: "start", Executable: "codex"}}
}

type resumeFenceFunc func(context.Context, string, func(context.Context) error) error

func (f resumeFenceFunc) Within(ctx context.Context, id string, fn func(context.Context) error) error {
	return f(ctx, id, fn)
}

type resumeRepositoryFake struct{ snapshot ResumeSnapshot }

func (f resumeRepositoryFake) LoadSnapshot(context.Context, string, uint64) (ResumeSnapshot, error) {
	return f.snapshot, nil
}
func (resumeRepositoryFake) BeginIntent(context.Context, ResumeSnapshot, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeProgress, error) {
	return ResumeProgress{}, nil
}
func (resumeRepositoryFake) LoadIntent(context.Context, ResumeProgress) (ResumeIntentState, error) {
	return ResumeIntentState{}, nil
}
func (resumeRepositoryFake) MarkInvoking(context.Context, ResumeIntentState) (ResumeIntentState, error) {
	return ResumeIntentState{}, nil
}
func (resumeRepositoryFake) RecordFailure(context.Context, ResumeIntentState, string, error) error {
	return nil
}
func (resumeRepositoryFake) ApplyReceipt(context.Context, ResumeIntentState, leasecontract.ResumeStageReceipt) (ResumeProgress, error) {
	return ResumeProgress{}, nil
}

type resumeArtifactsFunc func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error)

func (f resumeArtifactsFunc) ReadAndVerify(ctx context.Context, record leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
	return f(ctx, record)
}

type resumeOwnersFunc func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, bool, error)

func (f resumeOwnersFunc) Observe(ctx context.Context, record leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
	return f(ctx, record)
}

type resumeStagesFake struct{}

func (resumeStagesFake) Inspect(context.Context, ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	return leasecontract.ResumeStageInventory{}, nil
}
func (resumeStagesFake) Invoke(context.Context, ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
	return leasecontract.ResumeStageReceipt{}, nil
}

type resumeOperationIDsFunc func() (string, error)

func (f resumeOperationIDsFunc) New() (string, error) { return f() }

type resumeTraceRepository struct {
	record          Record
	trace           *[]string
	index           int
	invocationState string
}

func (r *resumeTraceRepository) LoadSnapshot(context.Context, string, uint64) (ResumeSnapshot, error) {
	*r.trace = append(*r.trace, "snapshot")
	return ResumeSnapshot{Record: r.record}, nil
}
func (r *resumeTraceRepository) BeginIntent(context.Context, ResumeSnapshot, leasecontract.ResumeArtifacts, leasedomain.ResumePlan, string) (ResumeProgress, error) {
	*r.trace = append(*r.trace, "begin")
	return r.progress(true), nil
}
func (r *resumeTraceRepository) LoadIntent(context.Context, ResumeProgress) (ResumeIntentState, error) {
	stage := []string{"terminal", "task", "dispatch"}[r.index]
	*r.trace = append(*r.trace, "load_"+stage)
	invocationState := r.invocationState
	if invocationState == "" {
		invocationState = "not_invoked_proven"
	}
	return ResumeIntentState{Progress: r.progress(true), OperationID: strings.Repeat("d", 32), Stage: stage, InvocationState: invocationState}, nil
}
func (r *resumeTraceRepository) MarkInvoking(_ context.Context, intent ResumeIntentState) (ResumeIntentState, error) {
	*r.trace = append(*r.trace, "mark_"+intent.Stage)
	intent.InvocationState, intent.InvocationAttempts = "unknown", 1
	return intent, nil
}
func (r *resumeTraceRepository) RecordFailure(context.Context, ResumeIntentState, string, error) error {
	return nil
}
func (r *resumeTraceRepository) ApplyReceipt(_ context.Context, intent ResumeIntentState, _ leasecontract.ResumeStageReceipt) (ResumeProgress, error) {
	*r.trace = append(*r.trace, "apply_"+intent.Stage)
	r.index++
	return r.progress(r.index < 3), nil
}
func (r *resumeTraceRepository) progress(pending bool) ResumeProgress {
	execution := *r.record.Stable.Execution
	if pending {
		execution.Pending = &leasecontract.ExternalIntent{OperationID: strings.Repeat("d", 32), Kind: "owner_launch", Marker: "marker", StartedAt: "start"}
	} else {
		execution.Pending = nil
	}
	return ResumeProgress{Record: r.record, Execution: execution, Pending: pending}
}

type resumeTraceStages struct {
	trace      *[]string
	inspectErr error
}

func (s resumeTraceStages) Inspect(_ context.Context, intent ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
	if s.trace != nil {
		*s.trace = append(*s.trace, "inspect_"+intent.Stage)
	}
	if s.inspectErr != nil {
		return leasecontract.ResumeStageInventory{}, s.inspectErr
	}
	return leasecontract.ResumeStageInventory{AuthoritativeZero: true}, nil
}
func (s resumeTraceStages) Invoke(_ context.Context, intent ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
	if s.trace != nil {
		*s.trace = append(*s.trace, "invoke_"+intent.Stage)
	}
	switch intent.Stage {
	case "terminal":
		return leasecontract.ResumeStageReceipt{TerminalPTYID: "pty"}, nil
	case "task":
		return leasecontract.ResumeStageReceipt{TaskID: "task"}, nil
	default:
		return leasecontract.ResumeStageReceipt{TaskID: "task", DispatchID: "dispatch"}, nil
	}
}
