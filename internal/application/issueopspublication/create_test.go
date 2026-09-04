package issueopspublication

import (
	"context"
	"errors"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopspublication"
)

func TestCreateRejectsMissingDependencies(t *testing.T) {
	repository := newFakeRepository(t)
	provider := newFakeProvider(t)
	verifier := acceptingVerifier(t)
	tests := []struct {
		name    string
		service *CreateService
	}{
		{name: "nil receiver"},
		{name: "repository", service: NewCreateService(nil, provider, verifier)},
		{name: "provider", service: NewCreateService(repository, nil, verifier)},
		{name: "verifier", service: NewCreateService(repository, provider, nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.service.Create(context.Background(), validCreateCommand(true))
			if err == nil || err.Error() != "publication create dependencies are required" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestCreatePreviewCallsProviderWithoutPersistence(t *testing.T) {
	command := validCreateCommand(false)
	intent := validIntent()
	request := intent.Request.Clone()
	request.Confirm = false
	eligibility := intent.Eligibility
	eligibility.Confirm = false
	eligibility.ExecutionActive = false
	eligibility.NoPending = false

	repository := newFakeRepository(t)
	repository.preview = func(_ context.Context, got contract.CreateCommand) (contract.PreparedCreate, error) {
		if got.Confirm || got.ID != "io-1" {
			t.Fatalf("command=%#v", got)
		}
		return contract.PreparedCreate{Request: request, Eligibility: eligibility}, nil
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(_ context.Context, gotProvider string, gotRequest contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		if gotProvider != "github" || gotRequest.Confirm {
			t.Fatalf("provider=%q request=%#v", gotProvider, gotRequest)
		}
		return contract.ProviderCreateResult{OK: true, Preview: "would create pull request"}, contract.InvocationUnknown, nil
	}

	result, err := NewCreateService(repository, provider, acceptingVerifier(t)).Create(context.Background(), command)
	if err != nil || result.Preview != "would create pull request" || providerCalls != 1 {
		t.Fatalf("result=%#v calls=%d err=%v", result, providerCalls, err)
	}
}

func TestCreatePersistsIntentBeforeProviderAndCompletesAfterVerify(t *testing.T) {
	events := []string{}
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) {
		events = append(events, "intent")
		return validIntent(), nil
	}
	repository.complete = func(_ context.Context, _ contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
		events = append(events, "receipt")
		if url != "https://github.com/acme/repo/pull/1" || !enforceOriginalGeneration {
			t.Fatalf("url=%q enforceOriginalGeneration=%v", url, enforceOriginalGeneration)
		}
		return validRecord(), nil
	}
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		events = append(events, "provider")
		return successfulResult(), contract.InvocationUnknown, nil
	}
	verifier := acceptingVerifier(t)
	verifier.live = func(_ context.Context, _ contract.Intent, url string) error {
		events = append(events, "verify")
		if url != "https://github.com/acme/repo/pull/1" {
			t.Fatalf("url=%q", url)
		}
		return nil
	}

	result, err := NewCreateService(repository, provider, verifier).Create(context.Background(), validCreateCommand(true))
	if err != nil || result.URL != "https://github.com/acme/repo/pull/1" || strings.Join(events, ",") != "intent,provider,verify,receipt" {
		t.Fatalf("events=%v result=%#v err=%v", events, result, err)
	}
}

func TestCreateBeginFailurePreventsProviderCall(t *testing.T) {
	cause := errors.New("intent CAS failed")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) {
		return contract.Intent{}, cause
	}

	_, err := NewCreateService(repository, newFakeProvider(t), acceptingVerifier(t)).Create(context.Background(), validCreateCommand(true))
	if err != cause {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateEligibilityFailurePreventsProviderCall(t *testing.T) {
	tests := []struct {
		name    string
		confirm bool
		wantErr string
	}{
		{name: "preview", confirm: false, wantErr: "publication eligibility: phase"},
		{name: "confirm", confirm: true, wantErr: "publication eligibility: pending"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newFakeRepository(t)
			if test.confirm {
				repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) {
					intent := validIntent()
					intent.Eligibility.NoPending = false
					return intent, nil
				}
			} else {
				repository.preview = func(context.Context, contract.CreateCommand) (contract.PreparedCreate, error) {
					intent := validIntent()
					intent.Eligibility.Confirm = false
					intent.Eligibility.PhasePR = false
					return contract.PreparedCreate{Request: intent.Request, Eligibility: intent.Eligibility}, nil
				}
			}
			_, err := NewCreateService(repository, newFakeProvider(t), acceptingVerifier(t)).Create(context.Background(), validCreateCommand(test.confirm))
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("err=%v want=%q", err, test.wantErr)
			}
		})
	}
}

func TestCreateRecordsTypedPreInvocationFailure(t *testing.T) {
	cause := errors.New("provider command was not started")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	recordedInvocation := contract.InvocationUnknown
	var recordedCause error
	repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, gotCause error) error {
		recordedInvocation = invocation
		recordedCause = gotCause
		if knownURL != "" {
			t.Fatalf("knownURL=%q", knownURL)
		}
		return nil
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		return contract.ProviderCreateResult{}, contract.InvocationNotInvokedProven, cause
	}

	_, err := NewCreateService(repository, provider, acceptingVerifier(t)).Create(context.Background(), validCreateCommand(true))
	if err == nil || err.Error() != "remote create outcome requires execution reconcile; creation was not retried: provider command was not started" {
		t.Fatalf("err=%v", err)
	}
	if recordedInvocation != contract.InvocationNotInvokedProven || recordedCause != cause || providerCalls != 1 {
		t.Fatalf("invocation=%q cause=%v calls=%d", recordedInvocation, recordedCause, providerCalls)
	}
}

func TestCreatePreservesAmbiguousProviderFailure(t *testing.T) {
	cause := errors.New("provider timeout")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	recordCalls := 0
	repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, gotCause error) error {
		recordCalls++
		if invocation != contract.InvocationUnknown || knownURL != "" || gotCause != cause {
			t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, gotCause)
		}
		return errors.New("failure receipt also failed")
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		return contract.ProviderCreateResult{}, contract.InvocationUnknown, cause
	}

	_, err := NewCreateService(repository, provider, acceptingVerifier(t)).Create(context.Background(), validCreateCommand(true))
	if err == nil || err.Error() != "remote create outcome requires execution reconcile; creation was not retried: provider timeout" || recordCalls != 1 || providerCalls != 1 {
		t.Fatalf("recordCalls=%d providerCalls=%d err=%v", recordCalls, providerCalls, err)
	}
}

func TestCreatePropagatesKnownURLIntoFailureReceipt(t *testing.T) {
	cause := errors.New("provider returned an ambiguous status")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	recordedURL := ""
	repository.recordFailure = func(_ context.Context, _ contract.Intent, _ contract.InvocationState, knownURL string, _ error) error {
		recordedURL = knownURL
		return nil
	}
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		return contract.ProviderCreateResult{URL: "https://github.com/acme/repo/pull/1"}, contract.InvocationUnknown, cause
	}

	result, err := NewCreateService(repository, provider, acceptingVerifier(t)).Create(context.Background(), validCreateCommand(true))
	if err == nil || result.URL != "https://github.com/acme/repo/pull/1" || recordedURL != "https://github.com/acme/repo/pull/1" {
		t.Fatalf("result=%#v recordedURL=%q err=%v", result, recordedURL, err)
	}
}

func TestCreateRejectsEmptyCanonicalURL(t *testing.T) {
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	recordCalls := 0
	repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
		recordCalls++
		if invocation != contract.InvocationUnknown || knownURL != "" || cause.Error() != "provider create returned no canonical URL" {
			t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, cause)
		}
		return nil
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		return contract.ProviderCreateResult{OK: true, URL: " \t "}, contract.InvocationUnknown, nil
	}

	result, err := NewCreateService(repository, provider, &fakeVerifier{t: t}).Create(context.Background(), validCreateCommand(true))
	if err == nil || err.Error() != "provider create returned no canonical URL" || result.URL != " \t " || recordCalls != 1 || providerCalls != 1 {
		t.Fatalf("result=%#v recordCalls=%d providerCalls=%d err=%v", result, recordCalls, providerCalls, err)
	}
}

func TestCreateVerificationFailureRetainsKnownURL(t *testing.T) {
	cause := errors.New("live artifact mismatch")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, gotCause error) error {
		if invocation != contract.InvocationUnknown || knownURL != "https://github.com/acme/repo/pull/1" || gotCause != cause {
			t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, gotCause)
		}
		return nil
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		return successfulResult(), contract.InvocationUnknown, nil
	}
	verifier := acceptingVerifier(t)
	verifier.live = func(context.Context, contract.Intent, string) error { return cause }

	result, err := NewCreateService(repository, provider, verifier).Create(context.Background(), validCreateCommand(true))
	if err == nil || err.Error() != "provider returned a URL but durable verification requires execution reconcile: live artifact mismatch" || result.URL != "https://github.com/acme/repo/pull/1" || providerCalls != 1 {
		t.Fatalf("result=%#v providerCalls=%d err=%v", result, providerCalls, err)
	}
}

func TestCreateReceiptFailureRetainsKnownURL(t *testing.T) {
	cause := errors.New("receipt CAS failed")
	repository := newFakeRepository(t)
	repository.begin = func(context.Context, contract.CreateCommand) (contract.Intent, error) { return validIntent(), nil }
	repository.complete = func(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error) {
		return contract.RecordSnapshot{}, cause
	}
	repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, gotCause error) error {
		if invocation != contract.InvocationUnknown || knownURL != "https://github.com/acme/repo/pull/1" || gotCause != cause {
			t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, gotCause)
		}
		return nil
	}
	providerCalls := 0
	provider := newFakeProvider(t)
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		providerCalls++
		return successfulResult(), contract.InvocationUnknown, nil
	}

	result, err := NewCreateService(repository, provider, acceptingVerifier(t)).Create(context.Background(), validCreateCommand(true))
	if err == nil || err.Error() != "provider succeeded but durable receipt requires execution reconcile: receipt CAS failed" || result.URL != "https://github.com/acme/repo/pull/1" || providerCalls != 1 {
		t.Fatalf("result=%#v providerCalls=%d err=%v", result, providerCalls, err)
	}
}
