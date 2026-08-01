package issueopspublication

import (
	"context"
	"errors"
	"strings"
	"testing"

	contract "agent-harness/internal/contract/issueopspublication"
)

func TestReconcileRejectsMissingDependencies(t *testing.T) {
	repository := newFakeRepository(t)
	provider := newFakeProvider(t)
	verifier := acceptingVerifier(t)
	tests := []struct {
		name    string
		service *ReconcileService
	}{
		{name: "nil receiver"},
		{name: "repository", service: NewReconcileService(nil, provider, verifier)},
		{name: "provider", service: NewReconcileService(repository, nil, verifier)},
		{name: "verifier", service: NewReconcileService(repository, provider, nil)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.service.Reconcile(context.Background(), "io-1")
			if err == nil || err.Error() != "publication reconcile dependencies are required" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReconcileLoadFailurePreventsInventory(t *testing.T) {
	cause := errors.New("intent load failed")
	repository := newFakeRepository(t)
	repository.load = func(context.Context, string) (contract.Intent, error) {
		return contract.Intent{}, cause
	}

	provider := newFakeProvider(t)
	result, err := NewReconcileService(repository, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
	if err != cause || result.ExternalStateInspected || result.Code != "" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if provider.inspectCalls != 0 || provider.createCalls != 0 {
		t.Fatalf("inspectCalls=%d createCalls=%d", provider.inspectCalls, provider.createCalls)
	}
}

func TestReconcileTransportFailurePreservesLatestAndAttemptedTruth(t *testing.T) {
	for _, attempted := range []bool{false, true} {
		t.Run(map[bool]string{false: "not-attempted", true: "attempted"}[attempted], func(t *testing.T) {
			cause := errors.New("provider inventory timeout")
			intent := validIntent()
			intent.KnownURL = "https://github.com/acme/repo/pull/known"
			latest := reconcileLatestRecord()
			recordCalls := 0
			repository := newFakeRepository(t)
			repository.load = func(context.Context, string) (contract.Intent, error) { return intent, nil }
			repository.recordFailure = func(_ context.Context, got contract.Intent, invocation contract.InvocationState, knownURL string, gotCause error) error {
				recordCalls++
				if got.OperationID != intent.OperationID || invocation != contract.InvocationUnknown || knownURL != intent.KnownURL || gotCause != cause {
					t.Fatalf("intent=%#v invocation=%q knownURL=%q cause=%v", got, invocation, knownURL, gotCause)
				}
				return errors.New("best-effort receipt failed")
			}
			repository.latest = latestRecordFunc(t, latest)
			provider := newFakeProvider(t)
			provider.inspect = func(context.Context, contract.Intent) (contract.Inventory, bool, error) {
				return contract.Inventory{}, attempted, cause
			}

			result, err := NewReconcileService(repository, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
			assertReconcileFailure(t, result, err, latest, "remote_reconcile_ambiguous", attempted, "remote reconcile transport is ambiguous; intent retained: provider inventory timeout")
			if provider.inspectCalls != 1 || provider.createCalls != 0 || recordCalls != 1 {
				t.Fatalf("inspectCalls=%d createCalls=%d recordCalls=%d", provider.inspectCalls, provider.createCalls, recordCalls)
			}
		})
	}
}

func TestReconcilePreserveMatrix(t *testing.T) {
	retryExhausted := provenZeroIntent()
	retryExhausted.RetryCount = 1
	tests := []struct {
		name         string
		intent       contract.Intent
		inventory    contract.Inventory
		wantCode     string
		wantErr      string
		failureCalls int
	}{
		{
			name: "multiple candidates", intent: validIntent(),
			inventory: contract.Inventory{Candidates: []contract.Candidate{{URL: "https://github.com/acme/repo/pull/1"}, {URL: "https://github.com/acme/repo/pull/2"}}},
			wantCode:  "remote_reconcile_multiple", wantErr: "remote reconcile found multiple candidates; intent retained", failureCalls: 1,
		},
		{
			name: "non-authoritative zero", intent: validIntent(), inventory: contract.Inventory{},
			wantCode: "remote_reconcile_zero_ambiguous", wantErr: "remote reconcile returned a non-authoritative zero candidate result; intent retained", failureCalls: 1,
		},
		{
			name: "zero without invocation proof", intent: validIntent(), inventory: contract.Inventory{AuthoritativeZero: true},
			wantCode: "remote_reconcile_zero_unproven", wantErr: "authoritative zero cannot clear an invocation whose absence was not proven; intent retained",
		},
		{
			name: "retry exhausted", intent: retryExhausted, inventory: contract.Inventory{AuthoritativeZero: true},
			wantCode: "remote_reconcile_retry_exhausted", wantErr: "remote create pre-invocation retry is unavailable or already consumed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := test.intent.Clone()
			intent.KnownURL = "https://github.com/acme/repo/pull/known"
			latest := reconcileLatestRecord()
			failureCalls := 0
			repository := newFakeRepository(t)
			repository.load = func(context.Context, string) (contract.Intent, error) { return intent, nil }
			if test.failureCalls > 0 {
				repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
					failureCalls++
					if invocation != contract.InvocationUnknown || knownURL != intent.KnownURL || cause.Error() != test.wantErr {
						t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, cause)
					}
					return nil
				}
			}
			repository.latest = latestRecordFunc(t, latest)
			provider := newFakeProvider(t)
			provider.inspect = func(context.Context, contract.Intent) (contract.Inventory, bool, error) {
				return test.inventory.Clone(), true, nil
			}

			result, err := NewReconcileService(repository, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
			assertReconcileFailure(t, result, err, latest, test.wantCode, true, test.wantErr)
			if provider.inspectCalls != 1 || provider.createCalls != 0 || failureCalls != test.failureCalls {
				t.Fatalf("inspectCalls=%d createCalls=%d failureCalls=%d wantFailureCalls=%d", provider.inspectCalls, provider.createCalls, failureCalls, test.failureCalls)
			}
		})
	}
}

func TestReconcileAdoptsExactCandidate(t *testing.T) {
	intent := validIntent()
	candidate := contract.Candidate{URL: "https://github.com/acme/repo/pull/1"}
	events := []string{}
	repository := newFakeRepository(t)
	repository.load = func(context.Context, string) (contract.Intent, error) { return intent, nil }
	repository.complete = func(_ context.Context, got contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
		events = append(events, "receipt")
		if got.OperationID != intent.OperationID || url != candidate.URL || enforceOriginalGeneration {
			t.Fatalf("intent=%#v url=%q enforceOriginalGeneration=%v", got, url, enforceOriginalGeneration)
		}
		return validRecord(), nil
	}
	provider := newFakeProvider(t)
	provider.inspect = func(context.Context, contract.Intent) (contract.Inventory, bool, error) {
		events = append(events, "inventory")
		return contract.Inventory{Candidates: []contract.Candidate{candidate}}, true, nil
	}
	verifier := acceptingVerifier(t)
	verifier.candidate = func(_ context.Context, got contract.Intent, gotCandidate contract.Candidate) error {
		events = append(events, "candidate")
		if got.OperationID != intent.OperationID || gotCandidate.URL != candidate.URL {
			t.Fatalf("intent=%#v candidate=%#v", got, gotCandidate)
		}
		return nil
	}
	verifier.live = func(_ context.Context, _ contract.Intent, url string) error {
		events = append(events, "live")
		if url != candidate.URL {
			t.Fatalf("url=%q", url)
		}
		return nil
	}

	result, err := NewReconcileService(repository, provider, verifier).Reconcile(context.Background(), "io-1")
	if err != nil || result.Code != "remote_reconcile_adopted" || !result.Reconciled || !result.ExternalStateInspected || result.Record.ID != "io-1" || strings.Join(events, ",") != "inventory,candidate,live,receipt" || provider.inspectCalls != 1 || provider.createCalls != 0 {
		t.Fatalf("events=%v inspectCalls=%d createCalls=%d result=%#v err=%v", events, provider.inspectCalls, provider.createCalls, result, err)
	}
}

func TestReconcileAdoptFailuresPreserveLatest(t *testing.T) {
	tests := []struct {
		name         string
		stage        string
		cause        error
		wantCode     string
		failureCalls int
		failureURL   string
	}{
		{name: "candidate mismatch", stage: "candidate", cause: errors.New("remote reconcile candidate does not match the exact durable intent"), wantCode: "remote_reconcile_candidate_mismatch", failureCalls: 1, failureURL: "known"},
		{name: "live verification", stage: "live", cause: errors.New("remote artifact verification failed"), wantCode: "remote_reconcile_verification_failed", failureCalls: 1, failureURL: "candidate"},
		{name: "receipt", stage: "receipt", cause: errors.New("receipt CAS failed"), wantCode: "remote_reconcile_receipt_failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			intent := validIntent()
			intent.KnownURL = "https://github.com/acme/repo/pull/known"
			candidate := contract.Candidate{URL: "https://github.com/acme/repo/pull/1"}
			latest := reconcileLatestRecord()
			failureCalls := 0
			repository := newFakeRepository(t)
			repository.load = func(context.Context, string) (contract.Intent, error) { return intent, nil }
			if test.failureCalls > 0 {
				repository.recordFailure = func(_ context.Context, _ contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
					failureCalls++
					wantURL := candidate.URL
					if test.failureURL == "known" {
						wantURL = intent.KnownURL
					}
					if invocation != contract.InvocationUnknown || knownURL != wantURL || cause != test.cause {
						t.Fatalf("invocation=%q knownURL=%q cause=%v", invocation, knownURL, cause)
					}
					return errors.New("best-effort receipt failed")
				}
			}
			repository.complete = func(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error) {
				if test.stage != "receipt" {
					t.Fatalf("unexpected Complete call at stage %q", test.stage)
				}
				return contract.RecordSnapshot{}, test.cause
			}
			repository.latest = latestRecordFunc(t, latest)
			provider := newFakeProvider(t)
			provider.inspect = func(context.Context, contract.Intent) (contract.Inventory, bool, error) {
				return contract.Inventory{Candidates: []contract.Candidate{candidate}}, true, nil
			}
			verifier := acceptingVerifier(t)
			verifier.candidate = func(context.Context, contract.Intent, contract.Candidate) error {
				if test.stage == "candidate" {
					return test.cause
				}
				return nil
			}
			verifier.live = func(context.Context, contract.Intent, string) error {
				if test.stage == "live" {
					return test.cause
				}
				return nil
			}

			result, err := NewReconcileService(repository, provider, verifier).Reconcile(context.Background(), "io-1")
			assertReconcileFailure(t, result, err, latest, test.wantCode, true, test.cause.Error())
			if provider.inspectCalls != 1 || provider.createCalls != 0 || failureCalls != test.failureCalls {
				t.Fatalf("inspectCalls=%d createCalls=%d failureCalls=%d want=%d", provider.inspectCalls, provider.createCalls, failureCalls, test.failureCalls)
			}
		})
	}
}

func TestReconcileRetryRequiresMarkerCASBeforeProvider(t *testing.T) {
	events := []string{}
	repository := newFakeRepository(t)
	repository.load = func(context.Context, string) (contract.Intent, error) { return provenZeroIntent(), nil }
	repository.markRetry = func(context.Context, contract.Intent) (contract.Intent, error) {
		events = append(events, "retry-cas")
		return retryIntent(), nil
	}
	repository.complete = func(_ context.Context, got contract.Intent, url string, enforceOriginalGeneration bool) (contract.RecordSnapshot, error) {
		if got.RetryCount != 1 || url != successfulResult().URL || enforceOriginalGeneration {
			t.Fatalf("intent=%#v url=%q enforceOriginalGeneration=%v", got, url, enforceOriginalGeneration)
		}
		return validRecord(), nil
	}
	provider := newFakeProvider(t)
	provider.inspect = func(ctx context.Context, intent contract.Intent) (contract.Inventory, bool, error) {
		return authoritativeZero(ctx, intent)
	}
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		events = append(events, "provider")
		return successfulResult(), contract.InvocationUnknown, nil
	}
	verifier := acceptingVerifier(t)
	verifier.live = func(context.Context, contract.Intent, string) error {
		return nil
	}

	result, err := NewReconcileService(repository, provider, verifier).Reconcile(context.Background(), "io-1")
	if err != nil || result.Code != "remote_reconcile_retry_succeeded" || !result.Reconciled || !result.ExternalStateInspected || provider.inspectCalls != 1 || provider.createCalls != 1 || strings.Join(events, ",") != "retry-cas,provider" {
		t.Fatalf("events=%v inspectCalls=%d createCalls=%d result=%#v err=%v", events, provider.inspectCalls, provider.createCalls, result, err)
	}
}

func TestReconcileRetryMarkerFailurePreventsProvider(t *testing.T) {
	cause := errors.New("external intent changed before retry CAS")
	latest := reconcileLatestRecord()
	repository := newFakeRepository(t)
	repository.load = func(context.Context, string) (contract.Intent, error) { return provenZeroIntent(), nil }
	repository.markRetry = func(context.Context, contract.Intent) (contract.Intent, error) { return contract.Intent{}, cause }
	repository.latest = latestRecordFunc(t, latest)
	provider := newFakeProvider(t)
	provider.inspect = authoritativeZero

	result, err := NewReconcileService(repository, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
	assertReconcileFailure(t, result, err, latest, "remote_reconcile_retry_cas_failed", true, cause.Error())
	if provider.inspectCalls != 1 || provider.createCalls != 0 {
		t.Fatalf("inspectCalls=%d createCalls=%d", provider.inspectCalls, provider.createCalls)
	}
}

func TestReconcileRetryTerminalNotInvoked(t *testing.T) {
	cause := errors.New("provider command was not started")
	invoking := retryIntent()
	repository := newFakeRepository(t)
	repository.load = func(context.Context, string) (contract.Intent, error) { return provenZeroIntent(), nil }
	repository.markRetry = func(context.Context, contract.Intent) (contract.Intent, error) { return invoking, nil }
	repository.completeNotInvoked = func(_ context.Context, got contract.Intent, gotCause error) (contract.RecordSnapshot, error) {
		if got.RetryCount != invoking.RetryCount || gotCause != cause {
			t.Fatalf("intent=%#v cause=%v", got, gotCause)
		}
		return validRecord(), nil
	}
	provider := newFakeProvider(t)
	provider.inspect = authoritativeZero
	provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
		return contract.ProviderCreateResult{}, contract.InvocationNotInvokedProven, cause
	}

	result, err := NewReconcileService(repository, provider, acceptingVerifier(t)).Reconcile(context.Background(), "io-1")
	if err != cause || result.Code != "remote_reconcile_retry_not_invoked" || !result.Reconciled || !result.ExternalStateInspected || result.Record.ID != "io-1" || provider.inspectCalls != 1 || provider.createCalls != 1 {
		t.Fatalf("inspectCalls=%d createCalls=%d result=%#v err=%v", provider.inspectCalls, provider.createCalls, result, err)
	}
}

func TestReconcileRetryFailuresPreserveLatest(t *testing.T) {
	tests := []struct {
		name         string
		stage        string
		invocation   contract.InvocationState
		cause        error
		wantCode     string
		wantErr      string
		failureCalls int
	}{
		{name: "ambiguous provider", stage: "provider", invocation: contract.InvocationUnknown, cause: errors.New("provider timeout"), wantCode: "remote_reconcile_retry_ambiguous", wantErr: "remote retry outcome is ambiguous; creation was not retried again: provider timeout", failureCalls: 1},
		{name: "live verification", stage: "live", cause: errors.New("remote artifact verification failed"), wantCode: "remote_reconcile_retry_verification_failed", wantErr: "remote artifact verification failed", failureCalls: 1},
		{name: "successful create receipt", stage: "receipt", cause: errors.New("receipt CAS failed"), wantCode: "remote_reconcile_retry_receipt_failed", wantErr: "receipt CAS failed"},
		{name: "terminal failure receipt", stage: "terminal-receipt", invocation: contract.InvocationNotInvokedProven, cause: errors.New("terminal receipt CAS failed"), wantCode: "remote_reconcile_retry_receipt_failed", wantErr: "terminal receipt CAS failed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providerCause := test.cause
			if test.stage == "terminal-receipt" {
				providerCause = errors.New("provider command was not started")
			}
			latest := reconcileLatestRecord()
			invoking := retryIntent()
			failureCalls := 0
			repository := newFakeRepository(t)
			repository.load = func(context.Context, string) (contract.Intent, error) { return provenZeroIntent(), nil }
			repository.markRetry = func(context.Context, contract.Intent) (contract.Intent, error) { return invoking, nil }
			if test.failureCalls > 0 {
				repository.recordFailure = func(_ context.Context, got contract.Intent, invocation contract.InvocationState, knownURL string, cause error) error {
					failureCalls++
					if got.RetryCount != 1 || invocation != contract.InvocationUnknown || knownURL != successfulResult().URL || cause != test.cause {
						t.Fatalf("intent=%#v invocation=%q knownURL=%q cause=%v", got, invocation, knownURL, cause)
					}
					return errors.New("best-effort receipt failed")
				}
			}
			repository.complete = func(context.Context, contract.Intent, string, bool) (contract.RecordSnapshot, error) {
				if test.stage != "receipt" {
					t.Fatalf("unexpected Complete call at stage %q", test.stage)
				}
				return contract.RecordSnapshot{}, test.cause
			}
			repository.completeNotInvoked = func(context.Context, contract.Intent, error) (contract.RecordSnapshot, error) {
				if test.stage != "terminal-receipt" {
					t.Fatalf("unexpected CompleteNotInvoked call at stage %q", test.stage)
				}
				return contract.RecordSnapshot{}, test.cause
			}
			repository.latest = latestRecordFunc(t, latest)
			provider := newFakeProvider(t)
			provider.inspect = authoritativeZero
			provider.create = func(context.Context, string, contract.ProviderCreateRequest) (contract.ProviderCreateResult, contract.InvocationState, error) {
				if test.stage == "provider" || test.stage == "terminal-receipt" {
					return successfulResult(), test.invocation, providerCause
				}
				return successfulResult(), contract.InvocationUnknown, nil
			}
			verifier := acceptingVerifier(t)
			verifier.live = func(context.Context, contract.Intent, string) error {
				if test.stage == "live" {
					return test.cause
				}
				return nil
			}

			result, err := NewReconcileService(repository, provider, verifier).Reconcile(context.Background(), "io-1")
			assertReconcileFailure(t, result, err, latest, test.wantCode, true, test.wantErr)
			if provider.inspectCalls != 1 || provider.createCalls != 1 || failureCalls != test.failureCalls {
				t.Fatalf("inspectCalls=%d createCalls=%d failureCalls=%d wantFailureCalls=%d", provider.inspectCalls, provider.createCalls, failureCalls, test.failureCalls)
			}
		})
	}
}

func reconcileLatestRecord() contract.RecordSnapshot {
	return contract.RecordSnapshot{ID: "io-1", Raw: []byte("{\"latest\":true}")}
}

func latestRecordFunc(t *testing.T, latest contract.RecordSnapshot) func(context.Context, string) (contract.RecordSnapshot, error) {
	t.Helper()
	return func(_ context.Context, id string) (contract.RecordSnapshot, error) {
		if id != "io-1" {
			t.Fatalf("id=%q", id)
		}
		return latest.Clone(), nil
	}
}

func assertReconcileFailure(t *testing.T, result contract.ReconcileResult, err error, latest contract.RecordSnapshot, code string, inspected bool, wantErr string) {
	t.Helper()
	if err == nil || err.Error() != wantErr || result.Code != code || result.Reconciled || result.ExternalStateInspected != inspected || result.Record.ID != latest.ID || string(result.Record.Raw) != string(latest.Raw) {
		t.Fatalf("result=%#v err=%v wantCode=%q wantInspected=%v wantErr=%q", result, err, code, inspected, wantErr)
	}
}
