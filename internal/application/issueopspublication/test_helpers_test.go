package issueopspublication

import (
	"context"
	"testing"

	contract "agent-harness/internal/contract/issueopspublication"
)

type fakeRepository struct {
	t                  *testing.T
	preview            func(context.Context, contract.CreateCommand) (contract.PreparedCreate, error)
	begin              func(context.Context, contract.CreateCommand) (contract.Intent, error)
	load               func(context.Context, string) (contract.Intent, error)
	markRetry          func(context.Context, contract.Intent) (contract.Intent, error)
	recordFailure      func(context.Context, contract.Intent, contract.InvocationState, string, error) error
	complete           func(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error)
	completeNotInvoked func(context.Context, contract.Intent, error) (contract.RecordSnapshot, error)
	latest             func(context.Context, string) (contract.RecordSnapshot, error)
}

func (f *fakeRepository) PreviewCreate(ctx context.Context, command contract.CreateCommand) (contract.PreparedCreate, error) {
	f.t.Helper()
	if f.preview == nil {
		f.t.Fatalf("unexpected Repository.PreviewCreate call")
	}
	return f.preview(ctx, command)
}

func (f *fakeRepository) BeginCreate(ctx context.Context, command contract.CreateCommand) (contract.Intent, error) {
	f.t.Helper()
	if f.begin == nil {
		f.t.Fatalf("unexpected Repository.BeginCreate call")
	}
	return f.begin(ctx, command)
}

func (f *fakeRepository) LoadIntent(ctx context.Context, id string) (contract.Intent, error) {
	f.t.Helper()
	if f.load == nil {
		f.t.Fatalf("unexpected Repository.LoadIntent call")
	}
	return f.load(ctx, id)
}

func (f *fakeRepository) MarkRetry(ctx context.Context, intent contract.Intent) (contract.Intent, error) {
	f.t.Helper()
	if f.markRetry == nil {
		f.t.Fatalf("unexpected Repository.MarkRetry call")
	}
	return f.markRetry(ctx, intent)
}

func (f *fakeRepository) RecordFailure(ctx context.Context, intent contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
	f.t.Helper()
	if f.recordFailure == nil {
		f.t.Fatalf("unexpected Repository.RecordFailure call")
	}
	return f.recordFailure(ctx, intent, invocation, knownURL, cause)
}

func (f *fakeRepository) Complete(ctx context.Context, intent contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
	f.t.Helper()
	if f.complete == nil {
		f.t.Fatalf("unexpected Repository.Complete call")
	}
	return f.complete(ctx, intent, url, enforceOriginalGeneration)
}

func (f *fakeRepository) CompleteNotInvoked(ctx context.Context, intent contract.Intent, cause error) (contract.RecordSnapshot, error) {
	f.t.Helper()
	if f.completeNotInvoked == nil {
		f.t.Fatalf("unexpected Repository.CompleteNotInvoked call")
	}
	return f.completeNotInvoked(ctx, intent, cause)
}

func (f *fakeRepository) Latest(ctx context.Context, id string) (contract.RecordSnapshot, error) {
	f.t.Helper()
	if f.latest == nil {
		f.t.Fatalf("unexpected Repository.Latest call")
	}
	return f.latest(ctx, id)
}

type fakeProvider struct {
	t       *testing.T
	create  func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error)
	inspect func(context.Context, contract.Intent) (contract.Inventory, bool, error)
}

func (f *fakeProvider) Create(ctx context.Context, provider string, request contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
	f.t.Helper()
	if f.create == nil {
		f.t.Fatalf("unexpected Provider.Create call")
	}
	return f.create(ctx, provider, request)
}

func (f *fakeProvider) Inspect(ctx context.Context, intent contract.Intent) (contract.Inventory, bool, error) {
	f.t.Helper()
	if f.inspect == nil {
		f.t.Fatalf("unexpected Provider.Inspect call")
	}
	return f.inspect(ctx, intent)
}

type fakeVerifier struct {
	t         *testing.T
	candidate func(context.Context, contract.Intent, contract.Candidate) error
	live      func(context.Context, contract.Intent, string) error
}

func (f *fakeVerifier) VerifyCandidate(ctx context.Context, intent contract.Intent, candidate contract.Candidate) error {
	f.t.Helper()
	if f.candidate == nil {
		f.t.Fatalf("unexpected Verifier.VerifyCandidate call")
	}
	return f.candidate(ctx, intent, candidate)
}

func (f *fakeVerifier) VerifyLive(ctx context.Context, intent contract.Intent, url string) error {
	f.t.Helper()
	if f.live == nil {
		f.t.Fatalf("unexpected Verifier.VerifyLive call")
	}
	return f.live(ctx, intent, url)
}

var _ Repository = (*fakeRepository)(nil)
var _ Provider = (*fakeProvider)(nil)
var _ Verifier = (*fakeVerifier)(nil)

func newFakeRepository(t *testing.T) *fakeRepository {
	t.Helper()
	return &fakeRepository{t: t}
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	return &fakeProvider{t: t}
}

func acceptingVerifier(t *testing.T) *fakeVerifier {
	t.Helper()
	return &fakeVerifier{
		t:         t,
		candidate: func(context.Context, contract.Intent, contract.Candidate) error { return nil },
		live:      func(context.Context, contract.Intent, string) error { return nil },
	}
}

func validCreateCommand(confirm bool) contract.CreateCommand {
	command := contract.CreateCommand{
		ID: "io-1", Provider: "github", Title: "title", Body: "body", Head: "195-branch", Base: "117-parent",
		Labels: []string{"enhancement"}, Assignees: []string{"maintainer"}, ExpectedGeneration: 7,
		Actor: contract.Actor{
			Host: "codex", SessionID: "session", AgentID: "agent",
			SessionProcess:  &contract.ProcessReceipt{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/usr/bin/codex"},
			ProcessAncestry: []contract.ProcessReceipt{{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/usr/bin/codex"}},
		},
		CWD: "/repo.worktrees/195-branch", Confirm: confirm,
	}
	return command.Clone()
}

func validIntent() contract.Intent {
	intent := contract.Intent{
		Record:      validRecord(),
		OperationID: "op-create-1",
		Generation:  8,
		Provider:    "github",
		Kind:        "pr",
		Request: contract.ProviderCreateRequest{
			Repo: "/repo", ProjectKey: "github.com/acme/repo", Title: "title", Body: "body",
			HeadBranch: "195-branch", BaseBranch: "117-parent", Labels: []string{"enhancement"},
			Assignees: []string{"maintainer"}, ExpectedHeadSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Confirm: true, Host: "codex", SessionID: "session", AgentID: "agent", CWD: "/repo.worktrees/195-branch",
		},
		InvocationState: contract.InvocationUnknown,
		Eligibility: contract.CreateEligibility{
			Provider: "github", Kind: "pr", Confirm: true, PhasePR: true,
			ExecutionActive: true, NoPending: true, NoArtifact: true,
			BranchAuthority: true, CanonicalLabelsAssignees: true,
		},
		Raw: []byte("{\"schema_version\":1,\"operation_id\":\"op-create-1\"}"),
	}
	return intent.Clone()
}

func provenZeroIntent() contract.Intent {
	intent := validIntent()
	intent.InvocationState = contract.InvocationNotInvokedProven
	intent.RetryCount = 0
	return intent.Clone()
}

func retryIntent() contract.Intent {
	intent := provenZeroIntent()
	intent.InvocationState = contract.InvocationUnknown
	intent.RetryCount = 1
	return intent.Clone()
}

func validRecord() contract.RecordSnapshot {
	record := contract.RecordSnapshot{ID: "io-1", Raw: []byte("{\"schema_version\":1,\"id\":\"io-1\"}")}
	return record.Clone()
}

func successfulResult() contract.ProviderCreateResult {
	return contract.ProviderCreateResult{OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1"}
}

func authoritativeZero(context.Context, contract.Intent) (contract.Inventory, bool, error) {
	return contract.Inventory{AuthoritativeZero: true}, true, nil
}
