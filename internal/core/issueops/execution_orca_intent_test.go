package issueops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestExecutionOrcaLegacyMarkerMigrationSupportsPrepareAndResumeProviders(t *testing.T) {
	tests := []struct {
		name    string
		fixture func(*testing.T) (string, IssueOpsRecord, externalOrcaIntentPayload)
	}{
		{name: "github prepare", fixture: legacyPrepareIntentFixture},
		{name: "github resume", fixture: func(t *testing.T) (string, IssueOpsRecord, externalOrcaIntentPayload) {
			return legacyResumeIntentFixture(t, "github", 16)
		}},
		{name: "gitlab resume", fixture: func(t *testing.T) (string, IssueOpsRecord, externalOrcaIntentPayload) {
			return legacyResumeIntentFixture(t, "gitlab", 2646)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record, payload := test.fixture(t)
			record, payload = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, nil)

			updated, sealed, migrated, err := reconcileCanonicalOrcaIntent(stateRoot, record)
			if err != nil {
				t.Fatal(err)
			}
			if !migrated {
				t.Fatal("exact proven-not-invoked legacy marker was not migrated")
			}
			identity, err := parseOrcaIntentMarker(sealed.Marker)
			if err != nil {
				t.Fatal(err)
			}
			want, err := authoritativeOrcaIssueIdentity(updated)
			if err != nil {
				t.Fatal(err)
			}
			if identity.Provider != want.Provider || identity.Issue != want.Issue ||
				sealed.Probe.Marker != sealed.Marker ||
				updated.Execution.Pending.Marker != sealed.Marker {
				t.Fatalf("migration was not atomic: marker=%q probe=%#v pending=%#v",
					sealed.Marker, sealed.Probe, updated.Execution.Pending)
			}
			if updated.Execution.Failure == nil ||
				updated.Execution.Failure.Message != "fixture text is not migration authority" {
				t.Fatalf("migration used or discarded failure text: %#v", updated.Execution.Failure)
			}
			persisted, err := readExternalOrcaIntentPayload(stateRoot, sealed.OperationID)
			if err != nil || persisted.Marker != sealed.Marker {
				t.Fatalf("canonical readback = %#v err=%v", persisted, err)
			}
		})
	}
}

func TestReconcileLegacyOrcaMarkerRejectsUnsafeEvidenceBeforeInspection(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IssueOpsRecord, *externalOrcaIntentPayload)
	}{
		{name: "unknown invocation", mutate: func(_ *IssueOpsRecord, payload *externalOrcaIntentPayload) {
			payload.InvocationState = orcaIntentUnknown
		}},
		{name: "changed operation marker", mutate: func(record *IssueOpsRecord, payload *externalOrcaIntentPayload) {
			changed := strings.Replace(payload.Marker, "operation=", "operation=x", 1)
			payload.Marker = changed
			payload.Probe.Marker = changed
			record.Execution.Pending.Marker = changed
		}},
		{name: "probe identity mismatch", mutate: func(_ *IssueOpsRecord, payload *externalOrcaIntentPayload) {
			payload.Probe.Provider = "gitlab"
			payload.Probe.Issue = 2646
		}},
		{name: "pending marker mismatch", mutate: func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
			record.Execution.Pending.Marker += "x"
		}},
		{name: "pending kind mismatch", mutate: func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
			record.Execution.Pending.Kind = "dispatch"
		}},
		{name: "pending operation mismatch", mutate: func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
			record.Execution.Pending.OperationID = strings.Repeat("e", 32)
		}},
		{name: "lease generation mismatch", mutate: func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
			record.Execution.Lease.Generation++
		}},
		{name: "failure operation mismatch", mutate: func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
			record.Execution.Failure.OperationID = strings.Repeat("f", 32)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record, payload := legacyPrepareIntentFixture(t)
			record, payload = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, test.mutate)
			beforeRecord := rawIssueOpsRow(t, stateRoot, record.ID)
			beforePayload := rawExternalIntentRow(t, stateRoot, payload.OperationID)
			inspectCalls, invokeCalls := 0, 0
			fake := &executionOrcaFake{
				inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
					inspectCalls++
					return port.ExecutionOrcaIntentInventory{}, nil
				},
				invoke: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
					invokeCalls++
					return port.ExecutionOrcaIntentReceipt{}, nil
				},
			}

			result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: record.ID, Confirm: true, Actor: executionActor("codex", "legacy-reconciler"), CWD: record.Repo,
			}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader})
			if err == nil || result.Code != "legacy_intent_upgrade_unsafe" {
				t.Fatalf("unsafe migration result = %#v err=%v", result, err)
			}
			if result.ExternalStateInspected || inspectCalls != 0 || invokeCalls != 0 {
				t.Fatalf("unsafe migration reached Orca: result=%#v inspect=%d invoke=%d",
					result, inspectCalls, invokeCalls)
			}
			if got := rawIssueOpsRow(t, stateRoot, record.ID); !bytes.Equal(got, beforeRecord) {
				t.Fatalf("unsafe migration changed IssueOps row\nbefore=%s\nafter=%s", beforeRecord, got)
			}
			if got := rawExternalIntentRow(t, stateRoot, payload.OperationID); !bytes.Equal(got, beforePayload) {
				t.Fatalf("unsafe migration changed intent row\nbefore=%s\nafter=%s", beforePayload, got)
			}
		})
	}
}

func TestReconcileLegacyOrcaMarkerRejectsUnverifiedBranchLink(t *testing.T) {
	stateRoot, record, payload := legacyPrepareIntentFixture(t)
	record, payload = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, func(record *IssueOpsRecord, _ *externalOrcaIntentPayload) {
		record.BranchPrepare.LinkVerified = false
	})

	_, _, migrated, err := reconcileCanonicalOrcaIntent(stateRoot, record)
	if err == nil || migrated || !strings.Contains(err.Error(), "legacy_intent_upgrade_unsafe") {
		t.Fatalf("legacy unverified reconcile = migrated:%t err:%v", migrated, err)
	}
}

func TestReconcileLegacyOrcaMarkerReportsMigrationBeforeInventory(t *testing.T) {
	stateRoot, record, payload := legacyResumeIntentFixture(t, "github", 16)
	record, _ = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, nil)
	inspectCalls := 0
	fake := &executionOrcaFake{inspect: func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		inspectCalls++
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{
			TerminalPTYID: "pty-migrated",
		}}}, nil
	}}

	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("codex", "legacy-reconciler"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reconciled || result.IntentMigrationCode != "legacy_intent_upgraded" ||
		!result.ExternalStateInspected || inspectCalls != 1 {
		t.Fatalf("migration disclosure = %#v inspect=%d", result, inspectCalls)
	}
}

func TestReconcileLegacyOrcaMarkerLeavesCanonicalPayloadByteExact(t *testing.T) {
	stateRoot, record, payload := legacyPrepareIntentFixture(t)
	beforeRecord := rawIssueOpsRow(t, stateRoot, record.ID)
	beforePayload := rawExternalIntentRow(t, stateRoot, payload.OperationID)

	updated, current, migrated, err := reconcileCanonicalOrcaIntent(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	if migrated || updated.Execution.Pending.Marker != payload.Marker || current.Marker != payload.Marker {
		t.Fatalf("canonical intent was unexpectedly migrated: migrated=%t current=%q", migrated, current.Marker)
	}
	if got := rawIssueOpsRow(t, stateRoot, record.ID); !bytes.Equal(got, beforeRecord) {
		t.Fatalf("canonical record was rewritten\nbefore=%s\nafter=%s", beforeRecord, got)
	}
	if got := rawExternalIntentRow(t, stateRoot, payload.OperationID); !bytes.Equal(got, beforePayload) {
		t.Fatalf("canonical payload was rewritten\nbefore=%s\nafter=%s", beforePayload, got)
	}
}

func legacyPrepareIntentFixture(t *testing.T) (string, IssueOpsRecord, externalOrcaIntentPayload) {
	t.Helper()
	stateRoot, record := orcaPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	persisted, payload, err := beginOrcaExecutionIntent(
		stateRoot, record, workspace,
		port.ExecutionOrcaProbeRequest{
			Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		},
		ExecutionPrepareRequest{OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh"},
		executionOwnerSnapshot{issue: executionOwnerIssue{BodySHA256: strings.Repeat("a", 64)}}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, persisted, payload
}

func legacyResumeIntentFixture(t *testing.T, provider string, issue int) (string, IssueOpsRecord, externalOrcaIntentPayload) {
	t.Helper()
	stateRoot, record, _ := reseededOrcaCycle(t)
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatal(err)
	}
	if provider == "gitlab" {
		record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/" + strconv.Itoa(issue)
		record.BranchPrepare.Provider = provider
		record.BranchPrepare.IssueURL = record.IssueURL
		record, err = writeIssueOps(stateRoot, record)
		if err != nil {
			t.Fatal(err)
		}
	}
	persisted, payload, err := beginOrcaExecutionResumeIntent(
		stateRoot, record, artifacts, record.Execution.Orca.RuntimeID, "", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, persisted, payload
}

func writeLegacyNotInvokedIntent(
	t *testing.T,
	stateRoot string,
	record IssueOpsRecord,
	payload externalOrcaIntentPayload,
	mutate func(*IssueOpsRecord, *externalOrcaIntentPayload),
) (IssueOpsRecord, externalOrcaIntentPayload) {
	t.Helper()
	legacy, err := renderLegacyOrcaIntentMarker(orcaIntentMarkerIdentity{
		Purpose: normalizedOrcaIntentPurpose(payload), LifecycleID: payload.LifecycleID,
		Generation: payload.Generation, OperationID: payload.OperationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload.Marker = legacy
	payload.Probe.Marker = legacy
	payload.InvocationState = orcaIntentNotInvoked
	payload.InvocationAttempts = 0
	record.Execution.Pending.Marker = legacy
	record.Execution.Failure = &model.ExecutionFailure{
		OperationID: payload.OperationID, Code: "external_operation_ambiguous",
		Message: "fixture text is not migration authority", At: "2026-07-30T00:00:00Z",
	}
	if mutate != nil {
		mutate(&record, &payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{
		Bucket: externalIntentBucket, ID: payload.OperationID, Data: raw,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return persisted, payload
}

func rawIssueOpsRow(t *testing.T, stateRoot, id string) []byte {
	t.Helper()
	return rawSQLStoreRow(t, stateRoot, issueOpsBucket, id)
}

func rawExternalIntentRow(t *testing.T, stateRoot, id string) []byte {
	t.Helper()
	return rawSQLStoreRow(t, stateRoot, externalIntentBucket, id)
}

func rawSQLStoreRow(t *testing.T, stateRoot, bucket, id string) []byte {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get(bucket, id)
	if err != nil || !ok {
		t.Fatalf("read raw %s/%s: ok=%t err=%v", bucket, id, ok, err)
	}
	return append([]byte(nil), raw...)
}

func TestExecutionOrcaReconcileAcceptsLegacyPrepareIntentWithoutPurpose(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		if request.Stage == port.ExecutionOrcaIntentTerminal {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true}
		}
		return successfulExecutionOrcaIntentReceipt(t, request), nil
	}
	if _, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("fixture must retain the terminal intent")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := readExternalOrcaIntentPayload(stateRoot, pending.Execution.Pending.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	payload.Purpose = ""
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(externalIntentBucket, payload.OperationID, raw); err != nil {
		t.Fatal(err)
	}
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{
			TerminalPTYID: "pty-legacy",
		}}}, nil
	}
	reconciled, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("codex", "reconciler"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Pending == nil || reconciled.Pending.Kind != "owner_launch" {
		t.Fatalf("legacy prepare intent did not advance: %#v", reconciled)
	}
}

func TestExecutionOrcaCrashAfterMutationReconcilesExactlyOneCandidateWithoutDuplicate(t *testing.T) {
	tests := []struct {
		name      string
		stage     port.ExecutionOrcaIntentStage
		nextKind  string
		completed bool
	}{
		{name: "worktree", stage: port.ExecutionOrcaIntentWorktree, nextKind: "owner_launch"},
		{name: "terminal", stage: port.ExecutionOrcaIntentTerminal, nextKind: "owner_launch"},
		{name: "Run", stage: port.ExecutionOrcaIntentRun, nextKind: "owner_launch"},
		{name: "Run bind", stage: port.ExecutionOrcaIntentRunBind, nextKind: "owner_launch"},
		{name: "task", stage: port.ExecutionOrcaIntentTask, nextKind: "dispatch"},
		{name: "dispatch", stage: port.ExecutionOrcaIntentDispatch, completed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := orcaPrepareRecord(t)
			calls := map[port.ExecutionOrcaIntentStage]int{}
			var crashedRequest port.ExecutionOrcaIntentRequest
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
				calls[request.Stage]++
				if request.Stage == test.stage {
					crashedRequest = request
					return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "injected_timeout", Invoked: true, Timeout: true}
				}
				return successfulExecutionOrcaIntentReceipt(t, request), nil
			}
			req := ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", OwnerEffort: "high",
			}
			if _, err := PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
				t.Fatal("injected post-mutation crash must leave a pending intent")
			}
			pending, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := readExternalOrcaIntentPayload(stateRoot, pending.Execution.Pending.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if intentPortStage(payload.Stage) != test.stage || payload.InvocationState != orcaIntentUnknown || calls[test.stage] != 1 {
				t.Fatalf("crash receipt was not fenced at %s: payload=%#v calls=%v", test.stage, payload, calls)
			}
			db, err := sqlstore.Open(stateRoot)
			if err != nil {
				t.Fatal(err)
			}
			raw, ok, err := db.Get(externalIntentBucket, payload.OperationID)
			if err != nil || !ok {
				t.Fatalf("read raw Orca intent: ok=%t err=%v", ok, err)
			}
			if bytes.Contains(raw, []byte(`"terminal_handle"`)) || bytes.Contains(raw, []byte("terminal-1")) {
				t.Fatalf("runtime-scoped terminal handle leaked into durable intent: %s", raw)
			}

			candidate := successfulExecutionOrcaIntentReceipt(t, crashedRequest)
			fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
				if request.Stage != test.stage {
					t.Fatalf("reconcile inspected %s, want %s", request.Stage, test.stage)
				}
				return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{candidate}}, nil
			}
			result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: record.ID, Confirm: true, Actor: executionActor("codex", "fresh-reconciler"), CWD: record.Repo,
			}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Reconciled || calls[test.stage] != 1 {
				t.Fatalf("reconcile repeated the external mutation: result=%#v calls=%v", result, calls)
			}
			if test.completed {
				if result.Pending != nil || result.Execution.Lease.Status != model.LeaseStatusClaimable || result.Execution.Orca == nil {
					t.Fatalf("dispatch adoption did not finalize claimable authority: %#v", result)
				}
				if _, err := readExternalOrcaIntentPayload(stateRoot, payload.OperationID); err == nil {
					t.Fatal("completed dispatch left an external intent payload")
				}
			} else if result.Pending == nil || result.Pending.Kind != test.nextKind {
				t.Fatalf("reconcile must advance exactly one stage: %#v", result)
			}
		})
	}
}

func TestExecutionOrcaReconcileZeroMultipleAndTransportAmbiguityNeverMutate(t *testing.T) {
	for _, test := range []struct {
		name    string
		inspect func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error)
	}{
		{name: "authoritative zero after unknown invocation", inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
		}},
		{name: "multiple", inspect: func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			candidate := successfulExecutionOrcaIntentReceipt(t, request)
			return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{candidate, candidate}}, nil
		}},
		{name: "transport", inspect: func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			return port.ExecutionOrcaIntentInventory{}, errors.New("inventory transport unavailable")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := orcaPrepareRecord(t)
			invokeCalls := 0
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
				invokeCalls++
				return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true}
			}
			req := ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
			}
			_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
			fake.inspect = test.inspect
			if _, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
				ID: record.ID, Confirm: true, Actor: executionActor("claude", "fresh"), CWD: record.Repo,
			}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
				t.Fatal("ambiguous inventory must retain the intent")
			}
			if invokeCalls != 1 {
				t.Fatalf("ambiguous reconcile repeated external mutation: calls=%d", invokeCalls)
			}
			current, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil || current.Execution.Pending == nil || current.Execution.Lease.Status != model.LeaseStatusReleased {
				t.Fatalf("ambiguous reconcile changed authority: record=%#v err=%v", current.Execution, err)
			}
		})
	}
}

func TestExecutionOrcaReconcileRetriesOnlyProvenNotInvokedAndOnlyOnce(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	invokeCalls := 0
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		invokeCalls++
		if invokeCalls <= 2 {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "pre_invocation_rejected", Invoked: false}
		}
		return successfulExecutionOrcaIntentReceipt(t, request), nil
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	fake.inspect = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
	}
	reconcile := func() error {
		_, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
			ID: record.ID, Confirm: true, Actor: executionActor("codex", "fresh"), CWD: record.Repo,
		}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader})
		return err
	}
	if err := reconcile(); err == nil {
		t.Fatal("the one permitted retry was also proven not invoked and must remain pending")
	}
	if invokeCalls != 2 {
		t.Fatalf("expected initial attempt plus one proven-not-invoked retry, got %d", invokeCalls)
	}
	if err := reconcile(); err == nil {
		t.Fatal("exhausted retry must fail closed")
	}
	if invokeCalls != 2 {
		t.Fatalf("exhausted retry invoked Orca again: %d", invokeCalls)
	}
}

func TestExecutionOrcaRunBindCanConvergeAfterUnknownOutcome(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	bindCalls := 0
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		if request.Stage != port.ExecutionOrcaIntentRunBind {
			return successfulExecutionOrcaIntentReceipt(t, request), nil
		}
		bindCalls++
		if bindCalls == 1 {
			return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "transport", Invoked: true}
		}
		return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}, nil
	}
	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err == nil {
		t.Fatal("첫 Run bind의 불명확한 결과는 reconcile을 요구해야 한다")
	}
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		if request.Stage != port.ExecutionOrcaIntentRunBind {
			t.Fatalf("reconcile stage = %s", request.Stage)
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
	}
	result, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("codex", "reconciler"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if bindCalls != 2 || result.Pending == nil || result.Pending.Kind != "owner_launch" {
		t.Fatalf("Run bind did not converge within the bounded retry: calls=%d result=%#v", bindCalls, result)
	}
}

func TestExecutionOrcaReceiptCASRejectsConcurrentIntentChange(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.invoke = func(port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
		return port.ExecutionOrcaIntentReceipt{}, &port.OrcaError{Code: "timeout", Invoked: true}
	}
	req := ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}
	_, _ = PrepareExecution(context.Background(), stateRoot, req, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	fake.inspect = func(request port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
		err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
			current, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				return err
			}
			current.Execution.Pending.Marker += "-changed"
			_, err = persistExecutionTransition(stateRoot, current, nil)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{successfulExecutionOrcaIntentReceipt(t, request)}}, nil
	}
	if _, err := ReconcileExecutionWithDependencies(context.Background(), stateRoot, ExecutionReconcileRequest{
		ID: record.ID, Confirm: true, Actor: executionActor("claude", "fresh"), CWD: record.Repo,
	}, ExecutionReconcileDependencies{Handler: legacyReconcileTestHandler, Orca: fake, ReadIssue: executionIssueSnapshotReader}); err == nil {
		t.Fatal("receipt CAS must reject a concurrent intent identity change")
	}
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || current.Execution.Pending == nil || current.Execution.Lease.Status != model.LeaseStatusReleased {
		t.Fatalf("failed CAS changed writer authority: record=%#v err=%v", current.Execution, err)
	}
}

func successfulExecutionOrcaIntentReceipt(t *testing.T, request port.ExecutionOrcaIntentRequest) port.ExecutionOrcaIntentReceipt {
	t.Helper()
	switch request.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if err := os.MkdirAll(request.Workspace.Root, 0o700); err != nil {
			t.Fatal(err)
		}
		prepared := executionOrcaWorkspaceReceipt(request.Workspace)
		return port.ExecutionOrcaIntentReceipt{Workspace: &prepared}
	case port.ExecutionOrcaIntentTerminal:
		return port.ExecutionOrcaIntentReceipt{TerminalPTYID: "pty-1", TerminalHandle: "terminal-1"}
	case port.ExecutionOrcaIntentRun:
		return port.ExecutionOrcaIntentReceipt{RunID: "run-1"}
	case port.ExecutionOrcaIntentRunBind:
		return port.ExecutionOrcaIntentReceipt{RunID: request.RunID, RunBound: true}
	case port.ExecutionOrcaIntentTask:
		return port.ExecutionOrcaIntentReceipt{TaskID: "task-1"}
	case port.ExecutionOrcaIntentDispatch:
		return port.ExecutionOrcaIntentReceipt{TaskID: request.TaskID, DispatchID: "dispatch-1"}
	default:
		t.Fatalf("unsupported stage %q", request.Stage)
		return port.ExecutionOrcaIntentReceipt{}
	}
}
