package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"

	leaseadapter "agent-harness/internal/adapter/outbound/issueopslease"
	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/core/issueops"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

func TestReleaseHandlerReturnsCommittedProjectionWithoutStatusReadback(t *testing.T) {
	actor := leasecontract.Actor{
		Host: "codex", SessionID: "release-session",
		SessionProcess: &leasecontract.ProcessReceipt{PID: 1234, StartedAt: "2026-07-29T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	record := leasecontract.Record{
		OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-inbound-release", Repo: "/source", Branch: "196-release", Phase: "implement",
		Execution: &leasecontract.Execution{
			Mode:           "orca",
			Workspace:      leasecontract.Workspace{SourceRoot: "/source", Root: "/canonical", Branch: "196-release", BaseHead: strings.Repeat("a", 40), Driver: "orca", LinkedAt: "2026-07-29T00:00:00Z"},
			Lease:          leasecontract.Lease{Generation: 1, Status: "active", Holder: &actor, ClaimedAt: "2026-07-29T00:00:01Z"},
			Orca:           &leasecontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", RunID: "run", OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", TaskID: "task", DispatchID: "dispatch"},
			Pending:        &leasecontract.ExternalIntent{OperationID: "pending", Kind: "pr_create", Marker: "marker", StartedAt: "2026-07-29T00:00:02Z"},
			Completion:     &leasecontract.Completion{FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/196.json", Verification: []string{"focused"}, RemoteArtifactURL: "https://example.test/pull/196", CompletedAt: "2026-07-29T00:00:03Z"},
			Failure:        &leasecontract.FailureDetail{OperationID: "failed-operation", Code: "transient", Message: "retry", At: "2026-07-29T00:00:04Z"},
			SyncBaseEvents: []leasecontract.SyncBaseEvent{{Mode: "apply", BaseBranch: "117-parent", BaseOID: strings.Repeat("c", 40), MergeCommit: strings.Repeat("d", 40), Actor: "codex", At: "2026-07-29T00:00:05Z"}},
		},
	}
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := newReleaseTestRepository(record.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	service := leaseapp.NewReleaseService(
		repository,
		fixedClock{at: time.Date(2026, 7, 29, 0, 3, 0, 0, time.UTC)},
		func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", receipt, nil
		},
		leaseadapter.FilesystemPathMatcher{},
	)
	handler := NewReleaseHandler(service)
	result, err := handler(context.Background(), "/state-root-that-must-not-be-read", issueops.ExecutionReleaseRequest{
		ID: record.ID, Generation: 1, CWD: "/canonical",
		Actor: issueopscontract.NativeActor{Host: actor.Host, SessionID: actor.SessionID, SessionProcess: &issueopscontract.NativeProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}, ProcessAncestry: []issueopscontract.NativeProcessReceipt{{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}}},
	})
	if err != nil {
		t.Fatalf("release handler: %v", err)
	}
	if !result.OK || result.Execution.Lease.Status != "released" || result.Execution.Orca == nil || result.Execution.Pending == nil || result.Execution.Completion == nil || result.Execution.Failure == nil || len(result.Execution.SyncBaseEvents) != 1 {
		t.Fatalf("handler did not return committed execution projection: %#v", result)
	}
	if result.Execution.Orca.RunID != "run" {
		t.Fatalf("handler lost Orca Run projection: %#v", result.Execution.Orca)
	}
}

func TestPublicReleaseErrorRetainsCompatibilityText(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "holder", err: leasedomain.Deny(leasedomain.DenyLeaseAuthority, errors.New("internal")), want: "only the current holder may release generation 7"},
		{name: "cwd", err: leasedomain.Deny(leasedomain.DenyCanonicalCWD, errors.New("internal")), want: "release cwd must be the canonical worktree"},
		{name: "invalid state", err: leasecontract.Fail(leasecontract.FailureInvalidState, statecontract.ErrInvalidState), want: "invalid state"},
		{name: "persistence", err: leasecontract.Fail(leasecontract.FailurePersistence, errors.New("holder index is unavailable")), want: "holder index is unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicReleaseError(tc.err, 7).Error(); got != tc.want {
				t.Fatalf("public error=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestReleaseHandlerPreservesNotPreparedCompatibilityText(t *testing.T) {
	actor := leasecontract.Actor{
		Host: "codex", SessionID: "unprepared-session",
		SessionProcess: &leasecontract.ProcessReceipt{PID: 1234, StartedAt: "2026-07-29T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	record := leasecontract.Record{OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-unprepared", Repo: "/source", Branch: "196-release", Phase: "implement"}
	data, err := leasecontract.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	repository, err := newReleaseTestRepository(record.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	service := leaseapp.NewReleaseService(
		repository,
		fixedClock{at: time.Date(2026, 7, 29, 0, 3, 0, 0, time.UTC)},
		func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
			return "live", receipt, nil
		},
		leaseadapter.FilesystemPathMatcher{},
	)
	_, err = NewReleaseHandler(service)(context.Background(), "/unused", issueops.ExecutionReleaseRequest{
		ID: record.ID, Generation: 1, CWD: "/canonical",
		Actor: issueopscontract.NativeActor{Host: actor.Host, SessionID: actor.SessionID, SessionProcess: &issueopscontract.NativeProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}, ProcessAncestry: []issueopscontract.NativeProcessReceipt{{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}}},
	})
	if err == nil || err.Error() != "IssueOps execution v1 is not prepared" {
		t.Fatalf("not prepared error=%v", err)
	}
}

func TestReleaseHandlerPreservesLegacyNativeActorValidationText(t *testing.T) {
	handler := NewReleaseHandler(leaseapp.NewReleaseService(nil, nil, nil, nil))
	for _, tc := range []struct {
		name  string
		actor issueopscontract.NativeActor
		want  string
	}{
		{name: "invalid host", actor: issueopscontract.NativeActor{Host: "other"}, want: "native actor host must be codex or claude"},
		{name: "missing session", actor: issueopscontract.NativeActor{Host: "codex"}, want: "native actor session_id is required"},
		{name: "missing receipt", actor: issueopscontract.NativeActor{Host: "codex", SessionID: "missing-receipt"}, want: "native actor requires a PID reuse-safe session_process receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler(context.Background(), "/unused", issueops.ExecutionReleaseRequest{ID: "io-native-actor", Generation: 1, Actor: tc.actor})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("handler error=%v want=%q", err, tc.want)
			}
		})
	}
}

func TestReleaseHandlerPreservesLegacyContractAndPersistenceText(t *testing.T) {
	actor := issueopscontract.NativeActor{
		Host:      "codex",
		SessionID: "public-error-session",
		SessionProcess: &issueopscontract.NativeProcessReceipt{
			PID: 1234, StartedAt: "2026-07-29T00:00:00Z", Executable: "/usr/bin/codex",
		},
		ProcessAncestry: []issueopscontract.NativeProcessReceipt{{
			PID: 1234, StartedAt: "2026-07-29T00:00:00Z", Executable: "/usr/bin/codex",
		}},
	}
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{
			name: "invalid state",
			err:  leasecontract.Fail(leasecontract.FailureInvalidState, statecontract.ErrInvalidState),
			want: "invalid state",
		},
		{
			name: "conflicting holder index",
			err:  leasecontract.Fail(leasecontract.FailurePersistence, errors.New("refusing to delete another lifecycle's lease-holder index")),
			want: "refusing to delete another lifecycle's lease-holder index",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := leaseapp.NewReleaseService(
				releaseErrorRepository{err: tc.err},
				fixedClock{at: time.Date(2026, 7, 29, 0, 3, 0, 0, time.UTC)},
				func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
					return "live", receipt, nil
				},
				leaseadapter.FilesystemPathMatcher{},
			)
			_, err := NewReleaseHandler(service)(context.Background(), "/unused", issueops.ExecutionReleaseRequest{
				ID: "io-public-error", Generation: 1, CWD: "/canonical", Actor: actor,
			})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("handler error=%v want=%q", err, tc.want)
			}
		})
	}
}

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }

type releaseErrorRepository struct{ err error }

func (r releaseErrorRepository) Update(
	context.Context,
	string,
	leaseapp.RecordValidator,
	leaseapp.RecordTransition,
) (leaseapp.RepositoryResult, error) {
	return leaseapp.RepositoryResult{}, r.err
}

type releaseTestRepository struct {
	id     string
	record leasecontract.Record
}

func newReleaseTestRepository(id string, data []byte) (*releaseTestRepository, error) {
	record, err := leasecontract.Decode(id, data)
	if err != nil {
		return nil, err
	}
	return &releaseTestRepository{id: id, record: record}, nil
}

func (r *releaseTestRepository) Update(
	_ context.Context,
	id string,
	validate leaseapp.RecordValidator,
	transition leaseapp.RecordTransition,
) (leaseapp.RepositoryResult, error) {
	if id != r.id {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, errors.New("issueops record not found"))
	}
	if r.record.Execution == nil {
		return leaseapp.RepositoryResult{}, leasecontract.Fail(leasecontract.FailurePersistence, leasecontract.ErrExecutionNotPrepared)
	}
	before := leaseapp.Record{ID: r.record.ID, CanonicalRoot: r.record.Execution.Workspace.Root, Lease: r.record.Execution.Lease}
	if err := validate(before); err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	after, err := transition(before)
	if err != nil {
		return leaseapp.RepositoryResult{}, err
	}
	r.record.Execution.Lease = after.Lease
	return leaseapp.RepositoryResult{Record: after, Execution: *r.record.Execution}, nil
}
