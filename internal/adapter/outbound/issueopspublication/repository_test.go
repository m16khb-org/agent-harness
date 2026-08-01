package issueopspublication

import (
	"bytes"
	"context"
	"errors"
	"testing"

	contract "agent-harness/internal/contract/issueopspublication"
)

type fakeEffects struct {
	preview            func(context.Context, contract.CreateCommand) (EffectState, error)
	begin              func(context.Context, contract.CreateCommand) (EffectState, error)
	load               func(context.Context, string) (EffectState, error)
	markRetry          func(context.Context, EffectState) (EffectState, error)
	recordFailure      func(context.Context, EffectState, contract.InvocationState, string, error) error
	complete           func(context.Context, EffectState, string, bool) (EffectState, error)
	completeNotInvoked func(context.Context, EffectState, error) (EffectState, error)
	latest             func(context.Context, string) (EffectState, error)
}

func (f *fakeEffects) PreviewCreate(ctx context.Context, command contract.CreateCommand) (EffectState, error) {
	return f.preview(ctx, command)
}

func (f *fakeEffects) BeginCreate(ctx context.Context, command contract.CreateCommand) (EffectState, error) {
	return f.begin(ctx, command)
}

func (f *fakeEffects) LoadIntent(ctx context.Context, id string) (EffectState, error) {
	return f.load(ctx, id)
}

func (f *fakeEffects) MarkRetry(ctx context.Context, state EffectState) (EffectState, error) {
	return f.markRetry(ctx, state)
}

func (f *fakeEffects) RecordFailure(ctx context.Context, state EffectState, invocation contract.InvocationState, knownURL string, cause error) error {
	return f.recordFailure(ctx, state, invocation, knownURL, cause)
}

func (f *fakeEffects) Complete(ctx context.Context, state EffectState, url string, enforceOriginalGeneration bool) (EffectState, error) {
	return f.complete(ctx, state, url, enforceOriginalGeneration)
}

func (f *fakeEffects) CompleteNotInvoked(ctx context.Context, state EffectState, cause error) (EffectState, error) {
	return f.completeNotInvoked(ctx, state, cause)
}

func (f *fakeEffects) Latest(ctx context.Context, id string) (EffectState, error) {
	return f.latest(ctx, id)
}

func TestRepositoryRejectsMissingPersistenceBridge(t *testing.T) {
	repository := NewRepository(nil)
	if _, err := repository.LoadIntent(context.Background(), "io-1"); err == nil || err.Error() != "publication persistence bridge is required" {
		t.Fatalf("err=%v", err)
	}
}

func TestRepositoryPassesRawSnapshotsWithoutRemarshal(t *testing.T) {
	recordRaw := []byte("{\n  \"schema_version\": 1\n}")
	intentRaw := []byte("{\"operation_id\":\"op-1\"}")
	requestLabels := []string{"enhancement"}
	effects := &fakeEffects{}
	effects.load = func(_ context.Context, id string) (EffectState, error) {
		if id != "io-1" {
			t.Fatalf("id=%q", id)
		}
		return EffectState{
			RecordID: "io-1", RecordRaw: recordRaw, IntentRaw: intentRaw, OperationID: "op-1", Generation: 7,
			Provider: "github", Kind: "pr", InvocationState: contract.InvocationUnknown, RetryCount: 1,
			KnownURL:    "https://github.com/acme/repo/pull/1",
			Request:     contract.ProviderCreateRequest{Title: "title", Labels: requestLabels},
			Eligibility: contract.CreateEligibility{Provider: "github", Kind: "pr", PhasePR: true},
		}, nil
	}

	intent, err := NewRepository(effects).LoadIntent(context.Background(), "io-1")
	if err != nil || intent.Record.ID != "io-1" || !bytes.Equal(intent.Record.Raw, recordRaw) || !bytes.Equal(intent.Raw, intentRaw) || intent.Request.Title != "title" || intent.RetryCount != 1 {
		t.Fatalf("intent=%#v err=%v", intent, err)
	}
	recordRaw[0], intentRaw[0], requestLabels[0] = 'x', 'y', "changed"
	if intent.Record.Raw[0] != '{' || intent.Raw[0] != '{' || intent.Request.Labels[0] != "enhancement" {
		t.Fatalf("effect mutation leaked into intent: %#v", intent)
	}
}

func TestRepositoryMapsEveryPersistenceOperation(t *testing.T) {
	cause := errors.New("provider failed")
	events := []string{}
	effects := &fakeEffects{}
	effects.preview = func(_ context.Context, command contract.CreateCommand) (EffectState, error) {
		events = append(events, "preview")
		return EffectState{Request: contract.ProviderCreateRequest{Title: command.Title}, Eligibility: contract.CreateEligibility{Provider: "github", Kind: "pr"}}, nil
	}
	effects.begin = func(context.Context, contract.CreateCommand) (EffectState, error) {
		events = append(events, "begin")
		return repositoryEffectState(), nil
	}
	effects.markRetry = func(_ context.Context, state EffectState) (EffectState, error) {
		events = append(events, "mark-retry")
		if !bytes.Equal(state.IntentRaw, []byte("intent")) {
			t.Fatalf("state=%#v", state)
		}
		state.RetryCount = 1
		return state, nil
	}
	effects.recordFailure = func(_ context.Context, state EffectState, invocation contract.InvocationState, knownURL string, gotCause error) error {
		events = append(events, "failure")
		if state.OperationID != "op-1" || invocation != contract.InvocationUnknown || knownURL != "known" || gotCause != cause {
			t.Fatalf("state=%#v invocation=%q knownURL=%q cause=%v", state, invocation, knownURL, gotCause)
		}
		return nil
	}
	effects.complete = func(_ context.Context, state EffectState, url string, enforce bool) (EffectState, error) {
		events = append(events, "complete")
		if state.OperationID != "op-1" || url != "url" || !enforce {
			t.Fatalf("state=%#v url=%q enforce=%v", state, url, enforce)
		}
		return EffectState{RecordID: "io-1", RecordRaw: []byte("complete")}, nil
	}
	effects.completeNotInvoked = func(_ context.Context, state EffectState, gotCause error) (EffectState, error) {
		events = append(events, "terminal")
		if state.OperationID != "op-1" || gotCause != cause {
			t.Fatalf("state=%#v cause=%v", state, gotCause)
		}
		return EffectState{RecordID: "io-1", RecordRaw: []byte("terminal")}, nil
	}
	effects.latest = func(_ context.Context, id string) (EffectState, error) {
		events = append(events, "latest")
		return EffectState{RecordID: id, RecordRaw: []byte("latest")}, nil
	}

	repository := NewRepository(effects)
	prepared, err := repository.PreviewCreate(context.Background(), contract.CreateCommand{Title: "title"})
	if err != nil || prepared.Request.Title != "title" || prepared.Eligibility.Provider != "github" {
		t.Fatalf("prepared=%#v err=%v", prepared, err)
	}
	intent, err := repository.BeginCreate(context.Background(), contract.CreateCommand{})
	if err != nil {
		t.Fatal(err)
	}
	invoking, err := repository.MarkRetry(context.Background(), intent)
	if err != nil || invoking.RetryCount != 1 {
		t.Fatalf("invoking=%#v err=%v", invoking, err)
	}
	if err := repository.RecordFailure(context.Background(), intent, contract.InvocationUnknown, "known", cause); err != nil {
		t.Fatal(err)
	}
	completed, err := repository.Complete(context.Background(), intent, "url", true)
	if err != nil || string(completed.Raw) != "complete" {
		t.Fatalf("completed=%#v err=%v", completed, err)
	}
	terminal, err := repository.CompleteNotInvoked(context.Background(), intent, cause)
	if err != nil || string(terminal.Raw) != "terminal" {
		t.Fatalf("terminal=%#v err=%v", terminal, err)
	}
	latest, err := repository.Latest(context.Background(), "io-1")
	if err != nil || string(latest.Raw) != "latest" {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	if got := stringsJoin(events); got != "preview,begin,mark-retry,failure,complete,terminal,latest" {
		t.Fatalf("events=%s", got)
	}
}

func repositoryEffectState() EffectState {
	return EffectState{
		RecordID: "io-1", RecordRaw: []byte("record"), IntentRaw: []byte("intent"), OperationID: "op-1",
		Generation: 7, Provider: "github", Kind: "pr", Request: contract.ProviderCreateRequest{Title: "title"},
		InvocationState: contract.InvocationUnknown,
	}
}

func stringsJoin(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ","
		}
		result += value
	}
	return result
}
