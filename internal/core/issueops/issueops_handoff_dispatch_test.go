package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

const testCoordinatorRecipient = "term_coordinator"

func TestHandoffStartRequiresPreDispatchReadiness(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.WorktreeTools = nil
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true}, client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "worktree_tools_prepared") {
		t.Fatalf("expected readiness error, got %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("readiness failure called Orca: %v", client.trace)
	}
}

func TestHandoffStartRequiresSealedCoordinatorRecipientBeforeAnyOrcaCall(t *testing.T) {
	for _, raw := range []string{
		`{"id":"%s"}`,
		`{"id":"%s","coordinator_recipient":"@all"}`,
		`{"id":"%s","coordinator_recipient":"term_1;rm"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			var req IssueOpsHandoffStartRequest
			if err := json.Unmarshal([]byte(fmt.Sprintf(raw, record.ID)), &req); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			_, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
			if err == nil || !strings.Contains(err.Error(), "coordinator recipient") {
				t.Fatalf("expected coordinator-recipient rejection, got %v", err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("invalid coordinator recipient called Orca: %v", client.trace)
			}
			if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !slices.Equal(before, after) {
				t.Fatal("invalid coordinator recipient mutated the durable record")
			}
		})
	}
}

func TestHandoffStartPreviewAutoSealsUniqueSourceRecipient(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.worktrees = []port.OrcaWorktree{{ID: "source-wt", Path: record.Repo}}
	client.terminals = []port.OrcaTerminal{{
		Handle: "term_source", PTYID: "pty-source", WorktreeID: "source-wt", WorktreePath: record.Repo, Connected: true, Writable: true,
	}}

	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || preview.CoordinatorRecipient != "term_source" || len(client.trace) != 2 {
		t.Fatalf("unique source recipient was not resolved in preview: result=%#v trace=%v", preview, client.trace)
	}
}

func TestHandoffStartPreviewRejectsAmbiguousSourceRecipients(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.worktrees = []port.OrcaWorktree{{ID: "source-wt", Path: record.Repo}}
	client.terminals = []port.OrcaTerminal{
		{Handle: "term_source_a", PTYID: "pty-source-a", WorktreeID: "source-wt", WorktreePath: record.Repo, Connected: true, Writable: true},
		{Handle: "term_source_b", PTYID: "pty-source-b", WorktreeID: "source-wt", WorktreePath: record.Repo, Connected: true, Writable: true},
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID}), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "exactly one connected writable source terminal") {
		t.Fatalf("ambiguous source recipient error = %v", err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !slices.Equal(before, after) || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("ambiguous source recipient mutated or dispatched: tasks=%d dispatch=%d trace=%v", client.taskCreates, client.dispatchCalls, client.trace)
	}
}

func TestHandoffStartRejectsRecipientSealedByAnotherActiveRecord(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	other := record
	other.ID = "io-abcdef123456"
	otherHandoff := *record.ExecutionHandoff
	otherHandoff.CoordinatorMailboxHandle = testCoordinatorRecipient
	other.ExecutionHandoff = &otherHandoff
	if _, err := WriteIssueOps(stateRoot, other); err != nil {
		t.Fatal(err)
	}

	client := handoffDispatchFake(record)
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient}), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "another active handoff") {
		t.Fatalf("recipient collision error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("recipient collision invoked Orca: %v", client.trace)
	}
}

func TestHandoffStartIgnoresClosedLegacyRecordDuringCoordinatorClaimScan(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	legacy := record
	legacy.ID = "io-abcdef123456"
	legacy.SchemaVersion = 1
	legacyHandoff := *record.ExecutionHandoff
	legacyHandoff.State = handoff.StateClosed
	legacyHandoff.ClosedDisposition = handoff.DispositionAccepted
	legacyHandoff.AttemptBaseHead = ""
	legacy.ExecutionHandoff = &legacyHandoff
	putRawIssueOpsRecordForTest(t, stateRoot, legacy)

	client := handoffDispatchFake(record)
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err != nil {
		t.Fatalf("closed legacy record blocked coordinator claim scan: %v", err)
	}
	if client.dispatchCalls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", client.dispatchCalls)
	}
}

func TestHandoffStartFailsClosedForInvalidActiveRecordDuringCoordinatorClaimScan(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	invalid := record
	invalid.ID = "io-fedcba654321"
	invalidHandoff := *record.ExecutionHandoff
	invalidHandoff.AttemptBaseHead = ""
	invalid.ExecutionHandoff = &invalidHandoff
	putRawIssueOpsRecordForTest(t, stateRoot, invalid)

	client := handoffDispatchFake(record)
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "attempt base head") {
		t.Fatalf("invalid active record error = %v", err)
	}
	if client.dispatchCalls != 0 {
		t.Fatalf("invalid active record reached dispatch: %d", client.dispatchCalls)
	}
}

func TestHandoffStartSealsCoordinatorAndDispatchesFromExactRecipient(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if started.CoordinatorRecipient != testCoordinatorRecipient || len(client.dispatchRequests) != 1 || client.dispatchRequests[0].FromHandle != testCoordinatorRecipient {
		t.Fatalf("coordinator recipient was not sealed into dispatch authority: result=%#v requests=%#v", started, client.dispatchRequests)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := handoff.BuildContext(persisted, handoff.ContextOptionsFromModel(*persisted.ExecutionHandoff.ContextOptions))
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.CoordinatorMailboxHandle != testCoordinatorRecipient || packet.Projection.CoordinatorRecipient != testCoordinatorRecipient || persisted.ExecutionHandoff.Orca.WorkerMailboxHandle != "term-1" || persisted.ExecutionHandoff.Orca.WorkerTerminalHandle != "term-1" {
		t.Fatalf("persisted dispatch authorities are incomplete: handoff=%#v projection=%#v", persisted.ExecutionHandoff, packet.Projection)
	}
	withoutRecipient := persisted
	withoutRecipientHandoff := *persisted.ExecutionHandoff
	withoutRecipientHandoff.CoordinatorMailboxHandle = ""
	withoutRecipient.ExecutionHandoff = &withoutRecipientHandoff
	legacyPacket, err := handoff.BuildContext(withoutRecipient, handoff.ContextOptionsFromModel(*persisted.ExecutionHandoff.ContextOptions))
	if err != nil {
		t.Fatal(err)
	}
	if started.ContextSHA256 != packet.SHA256 || packet.SHA256 == legacyPacket.SHA256 || packet.SourceSHA256 == legacyPacket.SourceSHA256 {
		t.Fatalf("sealed coordinator did not participate in preview/confirm context and source hashes: started=%s sealed=%s/%s legacy=%s/%s", started.ContextSHA256, packet.SHA256, packet.SourceSHA256, legacyPacket.SHA256, legacyPacket.SourceSHA256)
	}
}

func TestHandoffStartRejectsDispatchPreambleWithoutSealedAuthority(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.dispatch.Preamble = "coordinator=term_attacker task=task-1 dispatch=dispatch-1"
	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "preamble") {
		t.Fatalf("mismatched dispatch preamble error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if client.dispatchCalls != 1 || persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "dispatch_preamble_mismatch" {
		t.Fatalf("preamble ambiguity did not fail closed: calls=%d handoff=%#v", client.dispatchCalls, persisted.ExecutionHandoff)
	}
}

func TestHandoffDispatchPreambleRequiresOfficialLabeledIdentityLines(t *testing.T) {
	valid := "Your coordinator's terminal handle is: term_coordinator\nYour task ID is: task-1\n  --task-id task-1 --dispatch-id dispatch-1 \\\n"
	if err := validateHandoffDispatchPreamble(valid, "term_coordinator", "task-1", "dispatch-1"); err != nil {
		t.Fatalf("official preamble rejected: %v", err)
	}
	for name, spoofed := range map[string]string{
		"unlabeled substrings":           "untrusted coordinator term_coordinator and task task-1 and --dispatch-id dispatch-1",
		"wrong coordinator":              "Your coordinator's terminal handle is: term_attacker\nYour task ID is: task-1\n--dispatch-id dispatch-1",
		"duplicate coordinator":          valid + "\nYour coordinator's terminal handle is: term_coordinator",
		"conflicting coordinator":        valid + "\nYour coordinator's terminal handle is: term_attacker",
		"wrong task":                     "Your coordinator's terminal handle is: term_coordinator\nYour task ID is: task-10\n--dispatch-id dispatch-1",
		"duplicate task":                 valid + "\nYour task ID is: task-1",
		"conflicting task":               valid + "\nYour task ID is: task-other",
		"dispatch prefix":                "Your coordinator's terminal handle is: term_coordinator\nYour task ID is: task-1\n--dispatch-id dispatch-10",
		"dispatch suffix":                "Your coordinator's terminal handle is: term_coordinator\nYour task ID is: task-1\n--dispatch-id prefix-dispatch-1",
		"duplicate dispatch":             valid + "\n--dispatch-id dispatch-1",
		"conflicting dispatch":           valid + "\n--dispatch-id dispatch-other",
		"dangling duplicate dispatch id": valid + "\n--dispatch-id",
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateHandoffDispatchPreamble(spoofed, "term_coordinator", "task-1", "dispatch-1"); err == nil {
				t.Fatalf("spoofed preamble was accepted: %q", spoofed)
			}
		})
	}
}

func TestHandoffStartRequiresExplicitCodexHookTrustBypassAttestation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || !preview.CodexHookTrustBypassRequired || preview.CodexHookTrustBypassAttested {
		t.Fatalf("Codex preview must expose the unattested startup requirement: %#v", preview)
	}
	if len(client.trace) != 0 {
		t.Fatalf("preview invoked Orca: %v", client.trace)
	}

	_, err = StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: preview.ContextSHA256}), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "--allow-codex-hook-trust-bypass") {
		t.Fatalf("confirmed unattested Codex start error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("missing attestation must fail before terminal/task/dispatch calls: %v", client.trace)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.ContextSHA256 != "" || persisted.ExecutionHandoff.PendingOperation != nil {
		t.Fatalf("missing attestation persisted dispatch state: %#v", persisted.ExecutionHandoff)
	}

	attestedRequest := IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
	attestedRequest = coordinatorStartIdentity(record, attestedRequest)
	reviewed, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedRequest, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !reviewed.Preview || !reviewed.CodexHookTrustBypassRequired || !reviewed.CodexHookTrustBypassAttested || len(reviewed.ContextSHA256) != 64 {
		t.Fatalf("attested no-confirm preview must expose the reviewed context hash: %#v", reviewed)
	}
	if len(client.trace) != 0 {
		t.Fatalf("attested preview invoked Orca: %v", client.trace)
	}
	attestedRequest.Confirm = true
	attestedRequest.ExpectedContextSHA256 = reviewed.ContextSHA256
	started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedRequest, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if started.ContextSHA256 != reviewed.ContextSHA256 || !started.CodexHookTrustBypassRequired || !started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || !client.terminalRequests[0].AllowCodexHookTrustBypass {
		t.Fatalf("attested Codex start did not preserve launch authority: result=%#v requests=%#v", started, client.terminalRequests)
	}
}

func TestHandoffStartLeavesClaudeAndGJCStartupUnchanged(t *testing.T) {
	for _, agent := range []string{"claude", "gjc"} {
		t.Run(agent, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Agent = agent
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if started.CodexHookTrustBypassRequired || started.CodexHookTrustBypassAttested || len(client.terminalRequests) != 1 || client.terminalRequests[0].AllowCodexHookTrustBypass {
				t.Fatalf("%s startup changed under the Codex-only attestation: result=%#v terminal=%#v", agent, started, client.terminalRequests)
			}
		})
	}
}

func TestHandoffStartPreviewReturnsReviewedContextAndDoesNotMutate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || len(preview.ContextSHA256) != 64 {
		t.Fatalf("preview must return reviewed context hash: %#v", preview)
	}
	if len(client.trace) != 0 {
		t.Fatalf("preview invoked Orca: %v", client.trace)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(before) != string(after) {
		t.Fatal("preview mutated the durable lease")
	}
}

func TestHandoffStartConfirmRejectsCASDriftBeforeMutation(t *testing.T) {
	tests := []struct {
		name     string
		request  func(record IssueOpsRecord, preview IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest
		mutate   func(*testing.T, string, *IssueOpsRecord, IssueOpsHandoffStartRequest)
		expected string
	}{
		{
			name: "missing expected hash",
			request: func(record IssueOpsRecord, _ IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
			},
			expected: "expected_context_sha256",
		},
		{
			name: "malformed expected hash",
			request: func(record IssueOpsRecord, _ IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: "not-a-sha256", Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
			},
			expected: "expected_context_sha256",
		},
		{
			name: "mismatched expected hash",
			request: func(record IssueOpsRecord, _ IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: strings.Repeat("0", 64), Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
			},
			expected: "expected_context_sha256",
		},
		{
			name: "source drift",
			request: func(record IssueOpsRecord, preview IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: preview.ContextSHA256, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
			},
			mutate: func(t *testing.T, stateRoot string, record *IssueOpsRecord, _ IssueOpsHandoffStartRequest) {
				record.Intent.RawRequest = "Write the supervised handoff start contract after durable source drift"
				if _, err := WriteIssueOps(stateRoot, *record); err != nil {
					t.Fatal(err)
				}
			},
			expected: "expected_context_sha256 does not match freshly recomputed sealed context",
		},
		{
			name: "option drift",
			request: func(record IssueOpsRecord, preview IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: preview.ContextSHA256, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true, WorkerScope: "drifted-scope"}}
			},
			expected: "expected_context_sha256 does not match freshly recomputed sealed context",
		},
		{
			name: "GitHub identity drift",
			request: func(record IssueOpsRecord, preview IssueOpsHandoffStartResult) IssueOpsHandoffStartRequest {
				return IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: preview.ContextSHA256, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}
			},
			mutate: func(t *testing.T, stateRoot string, record *IssueOpsRecord, _ IssueOpsHandoffStartRequest) {
				issueURL := "https://gitlab.example/acme/repo/-/issues/16"
				record.IssueURL = issueURL
				record.BranchPrepare.Provider = "gitlab"
				record.BranchPrepare.IssueURL = issueURL
				record.ExecutionHandoff.Orca.ProviderIssueLinkStatus = handoff.ProviderIssueLinkGitLabExact
				if _, err := WriteIssueOps(stateRoot, *record); err != nil {
					t.Fatal(err)
				}
			},
			expected: "context",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}), client, handoffStartTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if !preview.Preview || len(preview.ContextSHA256) != 64 {
				t.Fatalf("preview must return reviewed context hash: %#v", preview)
			}
			if len(client.trace) != 0 {
				t.Fatalf("preview invoked Orca: %v", client.trace)
			}
			req := tt.request(record, preview)
			req = coordinatorStartIdentity(record, req)
			if tt.mutate != nil {
				tt.mutate(t, stateRoot, &record, req)
			}
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			_, err = StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expected)) {
				t.Fatalf("expected %q rejection, got %v", tt.expected, err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("%s drift must fail before Orca calls: %v", tt.name, client.trace)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if string(before) != string(after) {
				t.Fatalf("%s drift mutated the durable lease", tt.name)
			}
		})
	}
}

func TestHandoffStartRejectsWrongDigestOnAlreadySealedRetry(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	preview, err := StartIssueOpsHandoff(context.Background(), stateRoot, coordinatorStartIdentity(record, IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Context: handoff.ContextOptions{AllowCodexHookTrustBypass: true}}), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || len(preview.ContextSHA256) != 64 {
		t.Fatalf("preview must return reviewed context hash: %#v", preview)
	}
	sealed := record
	sealedOptions := handoff.ContextOptions{AllowCodexHookTrustBypass: true}
	packet, err := handoff.BuildContext(sealed, sealedOptions)
	if err != nil {
		t.Fatal(err)
	}
	sealed.ExecutionHandoff.ContextVersion = packet.Version
	sealed.ExecutionHandoff.ContextSHA256 = packet.SHA256
	sealed.ExecutionHandoff.ContextSourceSHA256 = packet.SourceSHA256
	sealed.ExecutionHandoff.ContextOptions = &issueopsmodel.IssueOpsExecutionHandoffContextOptions{AllowCodexHookTrustBypass: true}
	if _, err := WriteIssueOps(stateRoot, sealed); err != nil {
		t.Fatal(err)
	}
	retryClient := handoffDispatchFake(record)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	req := IssueOpsHandoffStartRequest{ID: record.ID, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: strings.Repeat("f", 64), Context: sealedOptions}
	req = coordinatorStartIdentity(record, req)
	_, err = StartIssueOpsHandoff(context.Background(), stateRoot, req, retryClient, handoffStartTestClock())
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "expected_context_sha256") {
		t.Fatalf("expected sealed retry digest rejection, got %v", err)
	}
	if len(retryClient.trace) != 0 {
		t.Fatalf("sealed retry must fail before Orca calls: %v", retryClient.trace)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(before) != string(after) {
		t.Fatal("sealed retry mutated the durable lease")
	}
}

func TestHandoffStartConfirmsReviewedContextForGitHubAndGitLab(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string, *IssueOpsRecord)
	}{
		{
			name:   "github",
			mutate: func(*testing.T, string, *IssueOpsRecord) {},
		},
		{
			name: "gitlab",
			mutate: func(t *testing.T, stateRoot string, record *IssueOpsRecord) {
				issueURL := "https://gitlab.example/acme/repo/-/issues/16"
				record.IssueURL = issueURL
				record.BranchPrepare.Provider = "gitlab"
				record.BranchPrepare.IssueURL = issueURL
				record.ExecutionHandoff.Orca.ProviderIssueLinkStatus = handoff.ProviderIssueLinkGitLabExact
				if _, err := WriteIssueOps(stateRoot, *record); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			tt.mutate(t, stateRoot, &record)
			client := handoffDispatchFake(record)
			started, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if started.State != handoff.StateDispatched || started.ContextSHA256 == "" {
				t.Fatalf("%s confirmed start failed: %#v", tt.name, started)
			}
			if len(client.terminalRequests) != 1 {
				t.Fatalf("%s confirmed start trace=%v result=%#v", tt.name, client.trace, started)
			}
		})
	}
}

func TestHandoffStartAdvancedStateRepeatsNeverMutateExternalSystems(t *testing.T) {
	for _, state := range []string{handoff.StateClaimed, handoff.StateSubmitted, handoff.StateClosed} {
		t.Run(state, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			record.ExecutionHandoff.State = state
			switch state {
			case handoff.StateClaimed:
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
			case handoff.StateSubmitted:
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
				record.ExecutionHandoff.Result = validCompletedHandoffResultForTest(record)
			case handoff.StateClosed:
				record.ExecutionHandoff.ClosedDisposition = handoff.DispositionAccepted
				record.ExecutionHandoff.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1"}
				record.ExecutionHandoff.Result = validCompletedHandoffResultForTest(record)
			}
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			got, err := StartIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffStartRequest{ID: record.ID, Confirm: true}, client, handoffStartTestClock())
			if err != nil || got.State != state {
				t.Fatalf("advanced-state start repeat = %#v err=%v", got, err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("advanced-state start repeat invoked Orca: %v", client.trace)
			}
		})
	}
}

func TestHandoffStartPersistsStableContextBeforeMutation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.beforeTerminalCreate = func() {
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.ExecutionHandoff.ContextVersion != handoff.ContextVersion || len(persisted.ExecutionHandoff.ContextSHA256) != 64 || persisted.ExecutionHandoff.PendingOperation == nil {
			t.Fatalf("context and pending operation must precede mutation: %#v", persisted.ExecutionHandoff)
		}
	}
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ContextSHA256) != 64 {
		t.Fatalf("missing stable context hash: %#v", got)
	}
}

func TestHandoffStartRejectsDirtyOrMovedHeadBeforeAnyOrcaCall(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string)
		want   string
	}{
		{
			name: "dirty worktree",
			mutate: func(t *testing.T, worktree string) {
				writeIssueOpsFile(t, worktree, "dirty.txt", "uncommitted\n")
			},
			want: "clean",
		},
		{
			name: "moved head",
			mutate: func(t *testing.T, worktree string) {
				writeIssueOpsFile(t, worktree, "moved.txt", "committed drift\n")
				for _, args := range [][]string{{"add", "moved.txt"}, {"commit", "-q", "-m", "test: move handoff head"}} {
					if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
						t.Fatalf("git %v failed: %s", args, stderr)
					}
				}
			},
			want: "head",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			tt.mutate(t, record.WorktreePath)
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			client := handoffDispatchFake(record)
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("unsafe initial checkpoint error = %v, want %q", err, tt.want)
			}
			if len(client.trace) != 0 {
				t.Fatalf("unsafe initial checkpoint invoked Orca: %v", client.trace)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if string(after) != string(before) {
				t.Fatal("unsafe initial checkpoint mutated the durable lease")
			}
		})
	}
}

func TestHandoffStartUsesFreshTimestampForEachOperationJournal(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	startedAt := map[string]string{}
	completedAt := map[string]string{}
	recordPending := func(kind string) {
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != kind {
			t.Fatalf("%s journal missing before mutation: %#v", kind, persisted.ExecutionHandoff.PendingOperation)
		}
		startedAt[kind] = persisted.ExecutionHandoff.PendingOperation.StartedAt
	}
	client.beforeTerminalCreate = func() { recordPending(handoff.OperationTerminalCreate) }
	client.beforeTaskCreate = func() { recordPending(handoff.OperationTaskCreate) }
	client.beforeDispatch = func() { recordPending(handoff.OperationDispatch) }
	step := 0
	clock := IssueOpsHandoffStartClock{Now: func() time.Time {
		at := time.Date(2026, 7, 11, 2, 0, step, 0, time.UTC)
		step++
		return at
	}}
	hooks := issueOpsHandoffStartHooks{BeforeStage: func(stage string) {
		previous := ""
		switch stage {
		case handoff.OperationTaskCreate:
			previous = handoff.OperationTerminalCreate
		case handoff.OperationDispatch:
			previous = handoff.OperationTaskCreate
		}
		if previous == "" {
			return
		}
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		completedAt[previous] = persisted.ExecutionHandoff.UpdatedAt
	}}
	if _, err := startIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, clock, hooks); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{handoff.OperationTerminalCreate, handoff.OperationTaskCreate, handoff.OperationDispatch} {
		want := time.Date(2026, 7, 11, 2, 0, i*2+1, 0, time.UTC).Format(time.RFC3339Nano)
		if got := startedAt[kind]; got != want {
			t.Fatalf("%s started_at = %q, want fresh stage timestamp %q (all=%#v)", kind, got, want, startedAt)
		}
	}
	for i, kind := range []string{handoff.OperationTerminalCreate, handoff.OperationTaskCreate} {
		want := time.Date(2026, 7, 11, 2, 0, i*2+2, 0, time.UTC).Format(time.RFC3339Nano)
		if got := completedAt[kind]; got != want {
			t.Fatalf("%s completion UpdatedAt = %q, want fresh post-call timestamp %q (all=%#v)", kind, got, want, completedAt)
		}
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantDispatchedAt := time.Date(2026, 7, 11, 2, 0, 6, 0, time.UTC).Format(time.RFC3339Nano)
	if persisted.ExecutionHandoff.DispatchedAt != wantDispatchedAt || persisted.ExecutionHandoff.UpdatedAt != wantDispatchedAt || persisted.UpdatedAt != wantDispatchedAt {
		t.Fatalf("dispatch completion timestamps reused journal time: handoff=%#v record.updated_at=%q want=%q", persisted.ExecutionHandoff, persisted.UpdatedAt, wantDispatchedAt)
	}
}

func TestHandoffStartStopsBeforeLaterOrcaStageAfterCheckpointDrift(t *testing.T) {
	tests := []struct {
		stage      string
		forbidden  []string
		wantPTY    string
		wantHandle string
		wantTask   string
	}{
		{stage: handoff.OperationTerminalCreate, forbidden: []string{"terminal-list", "terminal-create", "task-list", "task-create", "dispatch"}},
		{stage: handoff.OperationTaskCreate, forbidden: []string{"task-list", "task-create", "dispatch"}, wantPTY: "pty-1", wantHandle: "term-1"},
		{stage: handoff.OperationDispatch, forbidden: []string{"dispatch"}, wantPTY: "pty-1", wantHandle: "term-1", wantTask: "task-1"},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			hooks := issueOpsHandoffStartHooks{BeforeStage: func(stage string) {
				if stage == tt.stage {
					writeIssueOpsFile(t, record.WorktreePath, "stage-drift.txt", stage+"\n")
				}
			}}
			if _, err := startIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock(), hooks); err == nil || !strings.Contains(err.Error(), "clean worker worktree") {
				t.Fatalf("stage drift error = %v", err)
			}
			for _, forbidden := range tt.forbidden {
				if slices.Contains(client.trace, forbidden) {
					t.Fatalf("%s drift allowed later Orca call %q: %v", tt.stage, forbidden, client.trace)
				}
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || persisted.ExecutionHandoff.PendingOperation != nil || len(persisted.ExecutionHandoff.ContextSHA256) != 64 {
				t.Fatalf("stage drift wrote a later journal or released the lease: %#v", persisted.ExecutionHandoff)
			}
			if got := persisted.ExecutionHandoff.Orca.WorkerPTYID; got != tt.wantPTY {
				t.Fatalf("%s drift lost completed worker PTY: got %q want %q", tt.stage, got, tt.wantPTY)
			}
			if got := persisted.ExecutionHandoff.Orca.WorkerTerminalHandle; got != tt.wantHandle {
				t.Fatalf("%s drift lost completed worker handle: got %q want %q", tt.stage, got, tt.wantHandle)
			}
			if got := persisted.ExecutionHandoff.Orca.TaskID; got != tt.wantTask {
				t.Fatalf("%s drift lost completed task identity: got %q want %q", tt.stage, got, tt.wantTask)
			}
		})
	}
}

func TestHandoffOperationJournalRevalidatesCheckpointAfterInventory(t *testing.T) {
	tests := []struct {
		stage      string
		forbidden  []string
		wantPTY    string
		wantHandle string
		wantTask   string
	}{
		{stage: handoff.OperationTerminalCreate, forbidden: []string{"terminal-create", "task-list", "task-create", "dispatch"}},
		{stage: handoff.OperationTaskCreate, forbidden: []string{"task-create", "dispatch"}, wantPTY: "pty-1", wantHandle: "term-1"},
		{stage: handoff.OperationDispatch, forbidden: []string{"dispatch"}, wantPTY: "pty-1", wantHandle: "term-1", wantTask: "task-1"},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			hooks := issueOpsHandoffStartHooks{BeforeJournal: func(stage string) {
				if stage == tt.stage {
					writeIssueOpsFile(t, record.WorktreePath, "journal-drift.txt", stage+"\n")
				}
			}}
			if _, err := startIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock(), hooks); err == nil || !strings.Contains(err.Error(), "clean worker worktree") {
				t.Fatalf("journal drift error = %v", err)
			}
			for _, forbidden := range tt.forbidden {
				if slices.Contains(client.trace, forbidden) {
					t.Fatalf("%s journal drift allowed mutation %q: %v", tt.stage, forbidden, client.trace)
				}
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || persisted.ExecutionHandoff.PendingOperation != nil {
				t.Fatalf("rejected journal drift mutated the operation fence: %#v", persisted.ExecutionHandoff)
			}
			if got := persisted.ExecutionHandoff.Orca.WorkerPTYID; got != tt.wantPTY {
				t.Fatalf("%s journal drift lost completed worker PTY: got %q want %q", tt.stage, got, tt.wantPTY)
			}
			if got := persisted.ExecutionHandoff.Orca.WorkerTerminalHandle; got != tt.wantHandle {
				t.Fatalf("%s journal drift lost completed worker handle: got %q want %q", tt.stage, got, tt.wantHandle)
			}
			if got := persisted.ExecutionHandoff.Orca.TaskID; got != tt.wantTask {
				t.Fatalf("%s journal drift lost completed task identity: got %q want %q", tt.stage, got, tt.wantTask)
			}
		})
	}
}

func TestHandoffDispatchReattestsAfterBeforeJournalCompetitorInjection(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
	record.ExecutionHandoff.Orca.TaskID = "task-1"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	hooks := issueOpsHandoffStartHooks{BeforeJournal: func(stage string) {
		if stage == handoff.OperationDispatch {
			client.terminals = append(client.terminals, port.OrcaTerminal{
				Handle: "term-racing", PTYID: "pty-racing", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
				WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true,
			})
		}
	}}
	_, err := startIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock(), hooks)
	if err == nil || !strings.Contains(err.Error(), "competing connected or writable terminal") {
		t.Fatalf("post-hook sole-writer error = %v", err)
	}
	if client.dispatchCalls != 0 || client.terminalCreates != 0 || client.taskCreates != 0 {
		t.Fatalf("post-hook conflict reached external mutation: terminal=%d task=%d dispatch=%d trace=%v", client.terminalCreates, client.taskCreates, client.dispatchCalls, client.trace)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationLeaseAttestation || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "sole_writer_conflict" {
		t.Fatalf("post-hook conflict did not persist one lease recovery transition: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartLateCreateErrorCannotReopenCancelledAttempt(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	var cancelErr error
	client.beforeTerminalCreate = func() {
		_, cancelErr = RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{
			ID: record.ID, Action: "cancel", Confirm: true,
		}, nil, handoffPrepareTestClock())
	}
	client.terminalErr = errors.New("terminal create returned after coordinator cancel")
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("late terminal create error must still be reported")
	}
	if cancelErr != nil {
		t.Fatalf("coordinator cancel must durably tombstone an unresolved terminal-create journal: %v", cancelErr)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.ClosedDisposition != "" || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.Cancellation == nil || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "cancellation_requested" {
		t.Fatalf("late error must preserve the cancellation tombstone and unresolved journal: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartCreatesTerminalTaskDispatchExactlyOnce(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	req := attestedCodexStart(t, stateRoot, record.ID)
	first, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	second, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if client.terminalCreates != 1 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("expected one of each mutation, got terminal=%d task=%d dispatch=%d trace=%v", client.terminalCreates, client.taskCreates, client.dispatchCalls, client.trace)
	}
	if first.State != handoff.StateDispatched || second.State != handoff.StateDispatched || first.Orca == nil || first.Orca.DispatchID != "dispatch-1" {
		t.Fatalf("unexpected results: first=%#v second=%#v", first, second)
	}
}

func TestHandoffStartAdoptsExactlyOneCleanWorkerBaseline(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{{
		Handle: "term-baseline", PTYID: "pty-baseline", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
		WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true, Writable: true,
	}}
	client.dispatch.AssigneeHandle = "term-baseline"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || len(client.dispatchRequests) != 1 || client.dispatchRequests[0].ToHandle != "term-baseline" {
		t.Fatalf("worker baseline was not adopted: result=%#v creates=%d dispatch=%#v", got, client.terminalCreates, client.dispatchRequests)
	}
}

func TestHandoffStartFailsClosedForMultipleWorkerBaselines(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{
		{Handle: "term_worker_a", PTYID: "pty-worker-a", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true, Writable: true},
		{Handle: "term_worker_b", PTYID: "pty-worker-b", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true, Writable: true},
	}

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "sole writer") {
		t.Fatalf("multiple baseline error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 || persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired {
		t.Fatalf("multiple baseline was not fail-closed: creates=%d tasks=%d dispatch=%d handoff=%#v", client.terminalCreates, client.taskCreates, client.dispatchCalls, persisted.ExecutionHandoff)
	}
}

func TestHandoffStartConcurrentSameRecordDispatchesOnce(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := &lockedDispatchOrcaFake{fake: handoffDispatchFake(record)}
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	hooks := issueOpsHandoffStartHooks{BeforeJournal: func(stage string) {
		if stage == handoff.OperationTerminalCreate {
			arrived <- struct{}{}
			<-release
		}
	}}
	req := attestedCodexStart(t, stateRoot, record.ID)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := startIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock(), hooks)
			errs <- err
		}()
	}
	for range 2 {
		select {
		case <-arrived:
		case <-time.After(2 * time.Second):
			t.Fatal("same-record starts did not race before the terminal journal")
		}
	}
	close(release)

	var succeeded int
	for range 2 {
		if err := <-errs; err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("same-record starts succeeded %d times", succeeded)
	}
	client.mu.Lock()
	terminalCreates, taskCreates, dispatchCalls := client.fake.terminalCreates, client.fake.taskCreates, client.fake.dispatchCalls
	client.mu.Unlock()
	if terminalCreates != 1 || taskCreates != 1 || dispatchCalls != 1 {
		t.Fatalf("same-record start duplicated Orca work: terminal=%d task=%d dispatch=%d", terminalCreates, taskCreates, dispatchCalls)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateDispatched {
		t.Fatalf("same-record winner did not reach dispatched: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffStartRejectsNonDispatchedInitialStatus(t *testing.T) {
	for _, tt := range []struct{ name, status string }{
		{name: "missing"}, {name: "failed", status: "failed"}, {name: "cancelled", status: "cancelled"},
		{name: "completed", status: "completed"}, {name: "unknown", status: "unknown"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			client.dispatch.Status = tt.status
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "status") {
				t.Fatalf("initial dispatch status %q was accepted: %v", tt.status, err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil {
				t.Fatalf("invalid dispatch status did not retain recovery journal: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffStartResolvesOptionalPTYFromExactTerminalDelta(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{
		{Handle: "term-old", PTYID: "pty-old", WorktreeID: "wt-1", WorktreePath: record.WorktreePath},
	}
	client.terminal = port.OrcaTerminal{Handle: "term-create", WorktreeID: "wt-1"}
	client.terminalsAfterCreate = []port.OrcaTerminal{
		{Handle: "term-old", PTYID: "pty-old", WorktreeID: "wt-1", WorktreePath: record.WorktreePath},
		{Handle: "term-live", PTYID: "pty-new", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}
	client.dispatch.AssigneeHandle = "term-live"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.WorkerPTYID != "pty-new" || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("partial create identity did not resolve from PTY delta: %#v", got)
	}
	if client.terminalCreates != 1 || client.terminalListCalls != 9 {
		t.Fatalf("terminal create/list calls = %d/%d, want 1/9; trace=%v", client.terminalCreates, client.terminalListCalls, client.trace)
	}
}

func TestHandoffStartRejectsCreatePTYThatDiffersFromDelta(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminal = port.OrcaTerminal{Handle: "term-create", PTYID: "pty-returned", WorktreeID: "wt-1"}
	client.terminalsAfterCreate = []port.OrcaTerminal{
		{Handle: "term-live", PTYID: "pty-listed", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}

	_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err == nil || !strings.Contains(err.Error(), "created terminal PTY") {
		t.Fatalf("StartIssueOpsHandoff() error = %v, want create/list PTY mismatch", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || client.terminalCreates != 1 || client.taskCreates != 0 {
		t.Fatalf("PTY mismatch did not preserve recovery: handoff=%#v mutations=%d/%d", persisted.ExecutionHandoff, client.terminalCreates, client.taskCreates)
	}
}

func TestHandoffStartCrashMatrixNeverRepeatsCreate(t *testing.T) {
	tests := []struct {
		name string
		fail func(*dispatchOrcaFake)
	}{
		{name: "terminal-after-invoke", fail: func(f *dispatchOrcaFake) { f.terminalErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
		{name: "task-after-invoke", fail: func(f *dispatchOrcaFake) { f.taskErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
		{name: "dispatch-after-invoke", fail: func(f *dispatchOrcaFake) { f.dispatchErr = &port.OrcaError{Code: "timeout", Invoked: true} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			tt.fail(client)
			req := attestedCodexStart(t, stateRoot, record.ID)
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err == nil {
				t.Fatal("expected ambiguous mutation error")
			}
			counts := []int{client.terminalCreates, client.taskCreates, client.dispatchCalls}
			got, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock())
			if err != nil {
				t.Fatalf("repeat should return recovery status: %v", err)
			}
			if got.State != handoff.StateRecoveryRequired || client.terminalCreates != counts[0] || client.taskCreates != counts[1] || client.dispatchCalls != counts[2] {
				t.Fatalf("repeat performed mutation: result=%#v counts=%v now=%d/%d/%d", got, counts, client.terminalCreates, client.taskCreates, client.dispatchCalls)
			}
		})
	}
}

func TestHandoffStartDefinitivePreInvocationFailuresClearOnlyTheirJournal(t *testing.T) {
	tests := []struct {
		name  string
		fail  func(*dispatchOrcaFake)
		clear func(*dispatchOrcaFake)
	}{
		{name: "terminal command_start_failed", fail: func(f *dispatchOrcaFake) {
			f.terminalErr = &port.OrcaError{Code: "command_start_failed", Invoked: false}
		}, clear: func(f *dispatchOrcaFake) { f.terminalErr = nil }},
		{name: "task command_start_failed", fail: func(f *dispatchOrcaFake) { f.taskErr = &port.OrcaError{Code: "command_start_failed", Invoked: false} }, clear: func(f *dispatchOrcaFake) { f.taskErr = nil }},
		{name: "dispatch command_start_failed", fail: func(f *dispatchOrcaFake) {
			f.dispatchErr = &port.OrcaError{Code: "command_start_failed", Invoked: false}
		}, clear: func(f *dispatchOrcaFake) { f.dispatchErr = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			client := handoffDispatchFake(record)
			tt.fail(client)
			req := attestedCodexStart(t, stateRoot, record.ID)
			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "safe to retry") {
				t.Fatalf("definitive failure must return retry guidance: %v", err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateCoordinatorPreparing || persisted.ExecutionHandoff.PendingOperation != nil {
				t.Fatalf("definitive non-invocation left recovery journal: %#v", persisted.ExecutionHandoff)
			}
			tt.clear(client)
			if got, err := StartIssueOpsHandoff(context.Background(), stateRoot, req, client, handoffStartTestClock()); err != nil || got.State != handoff.StateDispatched {
				t.Fatalf("safe explicit retry did not complete: %#v err=%v", got, err)
			}
		})
	}
}

func TestHandoffStartTerminalCapabilityLossPreservesProvisionedLease(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	client := handoffDispatchFake(record)
	client.terminalErr = &port.OrcaError{Code: "terminal_create_capability_missing", Detail: "installed help changed", Invoked: false}
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("terminal capability loss must require recovery")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.PendingOperation == nil || persisted.ExecutionHandoff.PendingOperation.Kind != handoff.OperationTerminalCreate || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "terminal_create_capability_lost" {
		t.Fatalf("terminal capability loss cleared the provisioned lease: %#v", persisted.ExecutionHandoff)
	}
	if client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("terminal capability loss continued to later mutations: trace=%v", client.trace)
	}
}

func TestHandoffStartRejectsFullTerminalAndTaskBaselinesBeforeCreate(t *testing.T) {
	t.Run("terminal", func(t *testing.T) {
		stateRoot, record := handoffDispatchRecord(t)
		client := handoffDispatchFake(record)
		client.terminals = make([]port.OrcaTerminal, 0, handoff.MaxBaselineIDs)
		for i := 0; i < handoff.MaxBaselineIDs; i++ {
			client.terminals = append(client.terminals, port.OrcaTerminal{PTYID: fmt.Sprintf("pty-%03d", i)})
		}
		if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "headroom") {
			t.Fatalf("terminal headroom failure = %v", err)
		}
		if client.terminalCreates != 0 {
			t.Fatalf("terminal create invoked without delta headroom: %d", client.terminalCreates)
		}
	})
	t.Run("task", func(t *testing.T) {
		stateRoot, record := handoffDispatchRecord(t)
		record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
		record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
		if _, err := WriteIssueOps(stateRoot, record); err != nil {
			t.Fatal(err)
		}
		client := handoffDispatchFake(record)
		client.tasks = make([]port.OrcaTask, 0, handoff.MaxBaselineIDs)
		for i := 0; i < handoff.MaxBaselineIDs; i++ {
			client.tasks = append(client.tasks, port.OrcaTask{ID: fmt.Sprintf("task-%03d", i), Status: "ready"})
		}
		if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "headroom") {
			t.Fatalf("task headroom failure = %v", err)
		}
		if client.taskCreates != 0 {
			t.Fatalf("task create invoked without delta headroom: %d", client.taskCreates)
		}
	})
}

func TestHandoffStartBoundsTaskBaselineToReadyInventory(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	for i := 0; i < handoff.MaxBaselineIDs+100; i++ {
		client.tasks = append(client.tasks, port.OrcaTask{ID: fmt.Sprintf("completed-%03d", i), Status: "completed"})
	}
	client.tasks = append(client.tasks,
		port.OrcaTask{ID: "ready-old-1", Status: "ready"},
		port.OrcaTask{ID: "ready-old-2", Status: "ready"},
	)
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("completed task history blocked fresh handoff: result=%#v task/dispatch=%d/%d", got, client.taskCreates, client.dispatchCalls)
	}
}

func TestHandoffStartContinuesAfterTerminalCreateReconcileWithoutDuplicate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTerminalCreate, BaselinePTYIDs: []string{"pty-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminals = []port.OrcaTerminal{
		{Handle: "term-stale", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true},
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	client.refreshedTerminal = port.OrcaTerminal{Handle: "term-live", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: record.WorktreePath, Connected: true, Writable: true}
	client.dispatch.AssigneeHandle = "term-live"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || client.terminalRefreshes != 1 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("reconciled terminal was replayed: result=%#v create/refresh/task/dispatch=%d/%d/%d/%d trace=%v", got, client.terminalCreates, client.terminalRefreshes, client.taskCreates, client.dispatchCalls, client.trace)
	}
	if len(client.dispatchRequests) != 1 || client.dispatchRequests[0].ToHandle != "term-live" || got.Orca == nil || got.Orca.WorkerMailboxHandle != "term-live" {
		t.Fatalf("dispatch did not use the refreshed mailbox: result=%#v requests=%#v", got, client.dispatchRequests)
	}
}

func TestHandoffStartRecoversRuntimeReissuedTerminalWithoutDuplicateCreate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs = []string{"pty-baseline"}
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Title:     "⣴ 16-orca-supervised-ha...",
		Connected: true, Writable: true,
	}}
	client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")}
	client.dispatch.AssigneeHandle = "term-recovered"

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.RuntimeID != "runtime-2" || got.Orca.WorktreeInstanceID != "inst-2" || got.Orca.WorkerPTYID != "pty-recovered" || got.Orca.WorkerMailboxHandle != "term-recovered" || got.Orca.WorkerTabID != "tab-stable" || got.Orca.WorkerLeafID != "leaf-stable" {
		t.Fatalf("runtime-reissued terminal was not sealed before dispatch: %#v", got)
	}
	if client.terminalCreates != 0 || client.terminalRefreshes != 1 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("runtime recovery duplicated terminal or skipped dispatch: create/refresh/task/dispatch=%d/%d/%d/%d trace=%v", client.terminalCreates, client.terminalRefreshes, client.taskCreates, client.dispatchCalls, client.trace)
	}
	if len(client.dispatchRequests) != 1 || client.dispatchRequests[0].ToHandle != "term-recovered" {
		t.Fatalf("dispatch did not use recovered exact handle: %#v", client.dispatchRequests)
	}
}

func TestHandoffRuntimeRestartPreservesGitLabNativeMetadataObservation(t *testing.T) {
	zero, exact, mismatch := 0, 16, 17
	for _, tt := range []struct {
		name            string
		linkedIssue     int
		linkedGitLab    *int
		wantStatus      string
		wantUnavailable bool
		wantErr         bool
	}{
		{name: "null unavailable", wantStatus: handoff.ProviderIssueLinkGitLabUnavailable, wantUnavailable: true},
		{name: "zero unavailable", linkedGitLab: &zero, wantStatus: handoff.ProviderIssueLinkGitLabUnavailable, wantUnavailable: true},
		{name: "exact", linkedGitLab: &exact, wantStatus: handoff.ProviderIssueLinkGitLabExact},
		{name: "conflicting GitHub metadata", linkedIssue: 16, wantErr: true},
		{name: "mismatched GitLab metadata", linkedGitLab: &mismatch, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			issueURL := "https://gitlab.example/acme/repo/-/issues/16"
			record.IssueURL = issueURL
			record.BranchPrepare.Provider = "gitlab"
			record.BranchPrepare.IssueURL = issueURL
			record.ExecutionHandoff.Orca.ProviderIssueLinkStatus = handoff.ProviderIssueLinkGitLabUnavailable
			record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
			record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
			record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
			record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
			client.terminals = []port.OrcaTerminal{{
				RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
				WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
				Connected: true, Writable: true,
			}}
			row := runtimeRestartWorktree(record, "runtime-2", "inst-2")
			row.Issue = tt.linkedIssue
			row.GitLabIssue = tt.linkedGitLab
			client.worktrees = []port.OrcaWorktree{row}
			client.dispatch.AssigneeHandle = "term-recovered"

			got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
			if tt.wantErr {
				if err == nil || client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
					t.Fatalf("mismatched GitLab runtime metadata: result=%#v err=%v trace=%v", got, err, client.trace)
				}
				persisted, readErr := ReadIssueOps(stateRoot, record.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				h := persisted.ExecutionHandoff
				if h == nil || h.State != handoff.StateRecoveryRequired || h.PendingOperation == nil || h.PendingOperation.Kind != handoff.OperationRuntimeRefresh || h.Orca == nil || h.Orca.RuntimeID != "runtime-1" || h.Orca.WorktreeInstanceID != "inst-1" || h.Orca.WorkerTerminalHandle != "term-stale" || h.Orca.WorkerPTYID != "pty-stale" || h.Orca.WorkerTabID != "tab-stable" || h.Orca.WorkerLeafID != "leaf-stable" || h.Orca.ProviderIssueLinkStatus != handoff.ProviderIssueLinkGitLabUnavailable {
					t.Fatalf("conflicting GitLab runtime metadata changed the old identity/status: %#v", h)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Orca == nil || got.Orca.ProviderIssueLinkStatus != tt.wantStatus {
				t.Fatalf("runtime GitLab observation = %#v, want %q", got.Orca, tt.wantStatus)
			}
			projected, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID}, nil, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantUnavailable != containsString(projected.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning) {
				t.Fatalf("runtime-reprojected GitLab warnings = %#v", projected.Warnings)
			}
		})
	}
}

func TestHandoffStartRuntimeRestartWithDirtyWorkerNeverLaunchesReplacement(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "recovered-wip.txt", "uncommitted worker progress\n")
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Title:     "dynamic Codex title",
		Connected: true, Writable: true,
	}}
	client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")}

	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil || !strings.Contains(err.Error(), "clean worker worktree") {
		t.Fatalf("dirty runtime-recovered worker error = %v", err)
	}
	if len(client.trace) != 0 || client.terminalCreates != 0 {
		t.Fatalf("dirty recovered worker launched or inspected a replacement terminal: trace=%v creates=%d", client.trace, client.terminalCreates)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("dirty runtime-recovered worker mutated the durable lease")
	}
}

func TestHandoffStartRuntimeRestartRequiresUniqueExactWorktree(t *testing.T) {
	tests := []struct {
		name               string
		terminalWorktreeID string
		mutate             func(IssueOpsRecord, []port.OrcaWorktree) []port.OrcaWorktree
	}{
		{name: "missing instance", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree {
			rows[0].InstanceID = ""
			return rows
		}},
		{name: "wrong marker", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree {
			rows[0].Comment = "unrelated"
			return rows
		}},
		{name: "wrong head", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree {
			rows[0].Head = strings.Repeat("f", 40)
			return rows
		}},
		{name: "wrong branch", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree {
			rows[0].Branch = "refs/heads/unrelated"
			return rows
		}},
		{name: "missing", mutate: func(_ IssueOpsRecord, _ []port.OrcaWorktree) []port.OrcaWorktree { return nil }},
		{name: "terminal worktree mismatch", terminalWorktreeID: "wt-other", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree { return rows }},
		{name: "conflicting duplicate instance", mutate: func(_ IssueOpsRecord, rows []port.OrcaWorktree) []port.OrcaWorktree {
			conflict := rows[0]
			conflict.InstanceID = "inst-conflict"
			return append(rows, conflict)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
			record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
			record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
			record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
			terminalWorktreeID := tt.terminalWorktreeID
			if terminalWorktreeID == "" {
				terminalWorktreeID = "wt-1"
			}
			client.terminals = []port.OrcaTerminal{{
				RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
				WorktreeID: terminalWorktreeID, WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
				Title: "dynamic Codex title", Connected: true, Writable: true,
			}}
			client.worktrees = tt.mutate(record, []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")})

			if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
				t.Fatal("ambiguous runtime restart must fail closed")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Failure == nil || persisted.ExecutionHandoff.Failure.Code != "runtime_restart_ambiguous" || persisted.ExecutionHandoff.Orca.RuntimeID != "runtime-1" || persisted.ExecutionHandoff.Orca.WorktreeInstanceID != "inst-1" {
				t.Fatalf("ambiguous runtime restart adopted partial identity: %#v", persisted.ExecutionHandoff)
			}
			if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
				t.Fatalf("ambiguous runtime restart invoked a mutation: trace=%v", client.trace)
			}
		})
	}
}

func TestHandoffStartRuntimeRestartAcceptsEqualCurrentWorktreeInstance(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Title: "dynamic Codex title", Connected: true, Writable: true,
	}}
	client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", record.ExecutionHandoff.Orca.WorktreeInstanceID)}
	client.dispatch.AssigneeHandle = "term-recovered"
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.RuntimeID != "runtime-2" || got.Orca.WorktreeInstanceID != "inst-1" || client.terminalCreates != 0 {
		t.Fatalf("equal current worktree instance was rejected or duplicated: result=%#v trace=%v", got, client.trace)
	}
}

func TestHandoffRuntimeRefreshCompletionRevalidatesJournalAndFilesystem(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*testing.T, string, IssueOpsRecord)
		wantErr string
	}{
		{
			name: "durable journal drift",
			mutate: func(t *testing.T, stateRoot string, record IssueOpsRecord) {
				current, err := ReadIssueOps(stateRoot, record.ID)
				if err != nil {
					t.Fatal(err)
				}
				current.Feedback = append(current.Feedback, IssueOpsFeedbackItem{Source: "review", Body: "inventory completed under an older snapshot", CreatedAt: "2026-07-11T02:03:05Z"})
				if _, err := WriteIssueOps(stateRoot, current); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: "handoff changed during runtime-refresh inventory",
		},
		{
			name: "context source drift",
			mutate: func(t *testing.T, _ string, record IssueOpsRecord) {
				writeIssueOpsFile(t, record.WorktreePath, "plans/plan.md", "# changed after runtime inventory\n")
			},
			wantErr: "stale handoff context source fingerprint",
		},
		{
			name: "dirty exact checkpoint",
			mutate: func(t *testing.T, _ string, record IssueOpsRecord) {
				writeIssueOpsFile(t, record.WorktreePath, "runtime-refresh-drift.txt", "uncommitted drift\n")
			},
			wantErr: "clean worker worktree",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffDispatchRecord(t)
			record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
			record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
			record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
			record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			client := handoffDispatchFake(record)
			client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
			client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")}
			client.terminals = []port.OrcaTerminal{{
				RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
				WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
				Connected: true, Writable: true,
			}}
			client.beforeTerminalList = func() { tt.mutate(t, stateRoot, record) }

			_, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("runtime refresh drift error = %v, want %q", err, tt.wantErr)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			h := persisted.ExecutionHandoff
			if h == nil || h.State != handoff.StateRecoveryRequired || h.PendingOperation == nil || h.PendingOperation.Kind != handoff.OperationRuntimeRefresh || h.Orca == nil {
				t.Fatalf("runtime refresh drift did not preserve recovery journal: %#v", h)
			}
			if h.Orca.RuntimeID != "runtime-1" || h.Orca.WorktreeInstanceID != "inst-1" || h.Orca.WorkerTerminalHandle != "term-stale" || h.Orca.WorkerPTYID != "pty-stale" || h.Orca.WorkerTabID != "tab-stable" || h.Orca.WorkerLeafID != "leaf-stable" {
				t.Fatalf("runtime refresh drift partially adopted inventory identity: %#v", h.Orca)
			}
			if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
				t.Fatalf("runtime refresh drift reached a later mutation: trace=%v", client.trace)
			}
		})
	}
}

func TestHandoffStartRuntimeRestartLegacyUsesStableVisualTabMarker(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-now-observed", LeafID: "leaf-now-observed",
		Title: "dynamic Codex title", StableTabTitle: issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
		Connected: true, Writable: true,
	}}
	client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")}
	client.dispatch.AssigneeHandle = "term-recovered"
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || got.Orca == nil || got.Orca.WorkerTabID != "tab-now-observed" || got.Orca.WorkerLeafID != "leaf-now-observed" {
		t.Fatalf("legacy stable visual marker did not seal newly observed tab/leaf: %#v", got)
	}
}

func TestHandoffStartRuntimeRestartLegacyRejectsDynamicTitleMarkerAlone(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	marker := issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-untrusted", PTYID: "pty-untrusted",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-untrusted", LeafID: "leaf-untrusted",
		Title: marker, StableTabTitle: "unrelated stable tab", Connected: true, Writable: true,
	}}
	client.worktrees = []port.OrcaWorktree{runtimeRestartWorktree(record, "runtime-2", "inst-2")}
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("dynamic terminal title alone must not authorize legacy runtime recovery")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.State != handoff.StateRecoveryRequired || persisted.ExecutionHandoff.Orca.WorkerTerminalHandle != "term-stale" || persisted.ExecutionHandoff.Orca.WorkerTabID != "" {
		t.Fatalf("dynamic title fallback adopted untrusted terminal: %#v", persisted.ExecutionHandoff)
	}
}

func TestHandoffRuntimeRestartReconcileResumesToDispatchWithoutDuplicateCreate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Title: "dynamic Codex title", Connected: true, Writable: true,
	}}
	worktree := runtimeRestartWorktree(record, "runtime-2", "inst-2")
	worktree.Comment = "temporarily incomplete"
	client.worktrees = []port.OrcaWorktree{worktree}
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("first ambiguous runtime inventory must require recovery")
	}
	client.worktrees[0].Comment = issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	reconciled, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.State != handoff.StateCoordinatorPreparing || reconciled.Orca == nil || reconciled.Orca.RuntimeID != "runtime-2" || reconciled.Orca.WorktreeInstanceID != "inst-2" || reconciled.Orca.WorkerMailboxHandle != "" || reconciled.Orca.WorkerTerminalHandle != "term-recovered" {
		t.Fatalf("runtime reconciliation did not atomically adopt current identities: %#v", reconciled)
	}
	client.terminalRefreshErr = nil
	client.refreshedTerminal = client.terminals[0]
	client.dispatch.AssigneeHandle = "term-recovered"
	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || client.taskCreates != 1 || client.dispatchCalls != 1 {
		t.Fatalf("reconciled runtime did not resume exactly once: result=%#v trace=%v", got, client.trace)
	}
}

func TestHandoffExplicitRuntimeRefreshReconcileRevalidatesFilesystem(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-stale"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-stale"
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.terminalRefreshErr = &port.OrcaError{Code: "terminal_not_found"}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-2", Handle: "term-recovered", PTYID: "pty-recovered",
		WorktreeID: "wt-1", WorktreePath: record.WorktreePath, TabID: "tab-stable", LeafID: "leaf-stable",
		Connected: true, Writable: true,
	}}
	worktree := runtimeRestartWorktree(record, "runtime-2", "inst-2")
	worktree.Comment = "temporarily incomplete"
	client.worktrees = []port.OrcaWorktree{worktree}
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock()); err == nil {
		t.Fatal("incomplete inventory must first enter runtime recovery")
	}
	client.worktrees[0].Comment = issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	client.beforeTerminalList = func() {
		writeIssueOpsFile(t, record.WorktreePath, "explicit-runtime-reconcile-drift.txt", "uncommitted drift\n")
	}
	_, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "clean worker worktree") {
		t.Fatalf("explicit runtime reconcile drift error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	h := persisted.ExecutionHandoff
	if h == nil || h.State != handoff.StateRecoveryRequired || h.PendingOperation == nil || h.PendingOperation.Kind != handoff.OperationRuntimeRefresh || h.Orca == nil || h.Orca.RuntimeID != "runtime-1" || h.Orca.WorktreeInstanceID != "inst-1" || h.Orca.WorkerTerminalHandle != "term-stale" || h.Orca.WorkerPTYID != "pty-stale" {
		t.Fatalf("explicit runtime reconcile adopted identity after filesystem drift: %#v", h)
	}
	if client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 0 {
		t.Fatalf("explicit runtime reconcile drift reached a later mutation: trace=%v", client.trace)
	}
}

func runtimeRestartWorktree(record IssueOpsRecord, runtimeID, instanceID string) port.OrcaWorktree {
	return port.OrcaWorktree{
		RuntimeID: runtimeID, ID: record.ExecutionHandoff.Orca.WorktreeID, InstanceID: instanceID,
		RepoID: record.ExecutionHandoff.Orca.RepoID, Path: record.WorktreePath,
		Head: record.ExecutionHandoff.AttemptBaseHead, Branch: "refs/heads/" + record.Branch,
		Comment: issueOpsHandoffMarker(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt),
		BaseRef: record.ExecutionHandoff.Orca.BaseRef, Issue: 16,
	}
}

func TestHandoffStartContinuesAfterTaskCreateReconcileWithoutDuplicate(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.ExecutionHandoff.Orca.TerminalBaselinePTYIDs = []string{"pty-old"}
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
	setRecoveryRequiredForTest(&record, IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationTaskCreate, BaselineTaskIDs: []string{"task-old"},
	})
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	client.tasks = []port.OrcaTask{
		{ID: "task-1", Title: mustHandoffTaskTitle(t, record), DisplayName: mustHandoffTaskDisplay(t, record), Status: "ready"},
	}
	if _, err := RecoverIssueOpsHandoff(context.Background(), stateRoot, IssueOpsHandoffRecoverRequest{ID: record.ID, Action: "reconcile"}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}

	got, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != handoff.StateDispatched || client.terminalCreates != 0 || client.taskCreates != 0 || client.dispatchCalls != 1 {
		t.Fatalf("reconciled terminal or task was replayed: result=%#v mutations=%d/%d/%d trace=%v", got, client.terminalCreates, client.taskCreates, client.dispatchCalls, client.trace)
	}
}

func TestHandoffStartTerminalDeltaRequiresExactlyOne(t *testing.T) {
	tests := []struct {
		name string
		rows []port.OrcaTerminal
		ok   bool
	}{
		{name: "zero", rows: []port.OrcaTerminal{{PTYID: "old"}}},
		{name: "one", rows: []port.OrcaTerminal{{PTYID: "old"}, {PTYID: "new", Handle: "term-new", WorktreeID: "wt-1", WorktreePath: "/worker", Connected: true, Writable: true}}, ok: true},
		{name: "disconnected", rows: []port.OrcaTerminal{{PTYID: "new", Handle: "term-new", WorktreeID: "wt-1"}}},
		{name: "multiple", rows: []port.OrcaTerminal{{PTYID: "new-1"}, {PTYID: "new-2"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReconcileIssueOpsHandoffTerminal([]string{"old"}, "wt-1", "/worker", tt.rows)
			if tt.ok && (err != nil || got.PTYID != "new") {
				t.Fatalf("got %#v err=%v", got, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

func TestHandoffStartTaskMarkerRequiresExactlyOne(t *testing.T) {
	marker := issueOpsHandoffMarker("io-demo", "epoch-1", 1)
	rows := []port.OrcaTask{{ID: "old", Title: "old"}, {ID: "new", Title: marker, DisplayName: "16-demo", Status: "ready"}}
	got, err := ReconcileIssueOpsHandoffTask([]string{"old"}, marker, "16-demo", rows)
	if err != nil || got.ID != "new" {
		t.Fatalf("got %#v err=%v", got, err)
	}
	if _, err := ReconcileIssueOpsHandoffTask(nil, marker, "16-demo", append(rows, port.OrcaTask{ID: "another", Title: marker, DisplayName: "16-demo", Status: "ready"})); err == nil {
		t.Fatal("multiple marker candidates must fail closed")
	}
}

func TestHandoffStartDispatchRecoveryRequiresPersistedTask(t *testing.T) {
	client := handoffDispatchFake()
	if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "", "term-1", "inject", client, testCoordinatorRecipient); err == nil {
		t.Fatal("missing persisted task must fail")
	}
	got, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", "term-1", "inject", client, testCoordinatorRecipient)
	if err != nil || got.ID != "dispatch-1" {
		t.Fatalf("got %#v err=%v", got, err)
	}
}

func TestHandoffStartDispatchRecoveryRequiresSealedCoordinatorAndExactPreamble(t *testing.T) {
	client := handoffDispatchFake()
	if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", "term-1", "inject", client, ""); err == nil {
		t.Fatal("missing sealed coordinator recipient must fail")
	}
	client.dispatch.Preamble = "Your coordinator's terminal handle is: term_attacker\nYour task ID is: task-1\n  --task-id task-1 --dispatch-id dispatch-1"
	if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", "term-1", "inject", client, testCoordinatorRecipient); err == nil {
		t.Fatal("dispatch recovery must validate the exact sealed preamble authority")
	}
}

func TestHandoffStartDispatchRecoveryRejectsInvalidCoordinatorBeforeObservation(t *testing.T) {
	for _, recipient := range []string{"@all", "term_1;rm", "term_" + strings.Repeat("a", 252)} {
		t.Run(recipient[:min(len(recipient), 32)], func(t *testing.T) {
			client := handoffDispatchFake()
			if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-1", "term-1", "inject", client, recipient); err == nil || !strings.Contains(err.Error(), "concrete bounded Orca terminal handle") {
				t.Fatalf("invalid coordinator recovery error = %v", err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("invalid coordinator reached ShowDispatchFrom: %v", client.trace)
			}
		})
	}
}

func TestFinalizeHandoffDispatchRejectsInconsistentV4MailboxAuthority(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	h := record.ExecutionHandoff
	h.CoordinatorMailboxHandle = testCoordinatorRecipient
	h.Orca.WorkerTerminalHandle = "term-live"
	h.Orca.WorkerMailboxHandle = "term-stale-sealed"
	h.Orca.DispatchID = ""
	h.PendingOperation = &IssueOpsExecutionHandoffPendingOperation{
		Kind: handoff.OperationDispatch, StartedAt: "2026-07-11T00:00:01Z", ExpectedAssigneeHandle: "term-live", DeliveryMode: "inject",
	}
	putRawIssueOpsRecordForTest(t, stateRoot, record)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	fence := handoff.Fence{Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch, ContextSHA256: h.ContextSHA256}
	if _, err := finalizeHandoffDispatch(stateRoot, record.ID, fence, "dispatch-1", "term-live", "2026-07-11T00:00:02Z"); err == nil || !strings.Contains(err.Error(), "dispatch id and worker mailbox") {
		t.Fatalf("finalize dispatch error = %v, want unpaired sealed worker mailbox rejection", err)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("rejected inconsistent v4 dispatch authority was overwritten")
	}
}

type dispatchOrcaFake struct {
	worktrees               []port.OrcaWorktree
	terminals               []port.OrcaTerminal
	terminalsAfterCreate    []port.OrcaTerminal
	tasks                   []port.OrcaTask
	dispatchedTasks         []port.OrcaTask
	dispatchByTask          map[string]port.OrcaDispatch
	terminal                port.OrcaTerminal
	refreshedTerminal       port.OrcaTerminal
	task                    port.OrcaTask
	dispatch                port.OrcaDispatch
	terminalErr             error
	taskErr                 error
	dispatchErr             error
	dispatchShowErr         error
	worktreeListErr         error
	terminalListErr         error
	taskListErr             error
	dispatchedTaskListErr   error
	beforeWorktreeList      func()
	beforeTerminalList      func()
	beforeTerminalCreate    func()
	beforeTaskCreate        func()
	beforeDispatch          func()
	terminalCreates         int
	terminalListCalls       int
	terminalRefreshes       int
	terminalRefreshErr      error
	taskCreates             int
	dispatchCalls           int
	dispatchedTaskListCalls int
	dispatchRequests        []port.OrcaDispatchRequest
	terminalRequests        []port.OrcaCreateTerminalRequest
	trace                   []string
}

type lockedDispatchOrcaFake struct {
	mu   sync.Mutex
	fake *dispatchOrcaFake
}

func (f *lockedDispatchOrcaFake) ListWorktrees(ctx context.Context, repo string) ([]port.OrcaWorktree, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ListWorktrees(ctx, repo)
}

func (f *lockedDispatchOrcaFake) ListTerminals(ctx context.Context, worktreeID string) ([]port.OrcaTerminal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ListTerminals(ctx, worktreeID)
}

func (f *lockedDispatchOrcaFake) CreateTerminal(ctx context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.CreateTerminal(ctx, req)
}

func (f *lockedDispatchOrcaFake) RefreshTerminal(ctx context.Context, worktreeID, ptyID string) (port.OrcaTerminal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.RefreshTerminal(ctx, worktreeID, ptyID)
}

func (f *lockedDispatchOrcaFake) ListTasks(ctx context.Context) ([]port.OrcaTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ListTasks(ctx)
}

func (f *lockedDispatchOrcaFake) ListDispatchedTasks(ctx context.Context) ([]port.OrcaTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ListDispatchedTasks(ctx)
}

func (f *lockedDispatchOrcaFake) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.CreateTask(ctx, req)
}

func (f *lockedDispatchOrcaFake) Dispatch(ctx context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.Dispatch(ctx, req)
}

func (f *lockedDispatchOrcaFake) ShowDispatch(ctx context.Context, taskID string) (port.OrcaDispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ShowDispatch(ctx, taskID)
}

func (f *lockedDispatchOrcaFake) ShowDispatchFrom(ctx context.Context, taskID, fromHandle string) (port.OrcaDispatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fake.ShowDispatchFrom(ctx, taskID, fromHandle)
}

func (f *dispatchOrcaFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-list")
	if f.beforeWorktreeList != nil {
		f.beforeWorktreeList()
	}
	return append([]port.OrcaWorktree(nil), f.worktrees...), f.worktreeListErr
}

func handoffDispatchFake(records ...IssueOpsRecord) *dispatchOrcaFake {
	workerRoot := ""
	if len(records) > 0 && records[0].ExecutionHandoff != nil {
		workerRoot = records[0].ExecutionHandoff.WorkerRoot
	}
	fake := &dispatchOrcaFake{
		terminal: port.OrcaTerminal{Handle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", WorktreePath: workerRoot, Connected: true, Writable: true},
		task:     port.OrcaTask{ID: "task-1", Status: "ready"},
		dispatch: port.OrcaDispatch{ID: "dispatch-1", TaskID: "task-1", AssigneeHandle: "term-1", Status: "dispatched", Injected: true},
	}
	fake.terminalsAfterCreate = []port.OrcaTerminal{fake.terminal}
	if len(records) > 0 && records[0].ExecutionHandoff != nil && records[0].ExecutionHandoff.Orca != nil && records[0].ExecutionHandoff.Orca.WorkerPTYID != "" {
		identity := records[0].ExecutionHandoff.Orca
		fake.terminal = port.OrcaTerminal{
			Handle: identity.WorkerTerminalHandle, PTYID: identity.WorkerPTYID, WorktreeID: identity.WorktreeID,
			WorktreePath: workerRoot, Connected: true, Writable: true,
		}
		fake.terminals = []port.OrcaTerminal{fake.terminal}
	}
	return fake
}

func attestedCodexStart(t *testing.T, stateRoot, id string) IssueOpsHandoffStartRequest {
	t.Helper()
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	options := handoff.ContextOptions{}
	if record.ExecutionHandoff != nil && record.ExecutionHandoff.ContextOptions != nil {
		options = handoff.ContextOptionsFromModel(*record.ExecutionHandoff.ContextOptions)
	}
	if record.ExecutionHandoff != nil && strings.EqualFold(record.ExecutionHandoff.Agent, "codex") {
		options.AllowCodexHookTrustBypass = true
	}
	record.ExecutionHandoff.CoordinatorMailboxHandle = testCoordinatorRecipient
	record.ExecutionHandoff.CoordinatorSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator-session", AgentID: "coordinator-agent"}
	packet, err := handoff.BuildContext(record, options)
	if err != nil {
		t.Fatal(err)
	}
	return IssueOpsHandoffStartRequest{ID: id, CoordinatorRecipient: testCoordinatorRecipient, Confirm: true, ExpectedContextSHA256: packet.SHA256, Context: options, CoordinatorHost: "codex", CoordinatorSessionID: "coordinator-session", CoordinatorAgentID: "coordinator-agent", SourceCWD: record.Repo}
}

func coordinatorStartIdentity(record IssueOpsRecord, req IssueOpsHandoffStartRequest) IssueOpsHandoffStartRequest {
	req.CoordinatorHost = "codex"
	req.CoordinatorSessionID = "coordinator-session"
	req.CoordinatorAgentID = "coordinator-agent"
	req.SourceCWD = record.Repo
	return req
}

func mustHandoffTaskTitle(t *testing.T, record IssueOpsRecord) string {
	t.Helper()
	title, _, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	return title
}

func mustHandoffTaskDisplay(t *testing.T, record IssueOpsRecord) string {
	t.Helper()
	_, display, err := issueOpsHandoffTaskIdentity(record.ID, record.ExecutionHandoff.OwnershipEpoch, record.ExecutionHandoff.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	return display
}

func (f *dispatchOrcaFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-list")
	f.terminalListCalls++
	if f.beforeTerminalList != nil {
		f.beforeTerminalList()
	}
	return append([]port.OrcaTerminal(nil), f.terminals...), f.terminalListErr
}

func (f *dispatchOrcaFake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-create")
	f.terminalCreates++
	f.terminalRequests = append(f.terminalRequests, req)
	if f.beforeTerminalCreate != nil {
		f.beforeTerminalCreate()
	}
	if f.terminalsAfterCreate != nil && !externalMutationNotInvoked(f.terminalErr) {
		f.terminals = append([]port.OrcaTerminal(nil), f.terminalsAfterCreate...)
	}
	return f.terminal, f.terminalErr
}

func (f *dispatchOrcaFake) RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error) {
	f.trace = append(f.trace, "terminal-refresh")
	f.terminalRefreshes++
	if f.terminalRefreshErr != nil {
		return port.OrcaTerminal{}, f.terminalRefreshErr
	}
	if f.refreshedTerminal.PTYID != "" {
		f.terminals = []port.OrcaTerminal{f.refreshedTerminal}
		return f.refreshedTerminal, nil
	}
	return f.terminal, nil
}

func (f *dispatchOrcaFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	f.trace = append(f.trace, "task-list")
	return append([]port.OrcaTask(nil), f.tasks...), f.taskListErr
}

func (f *dispatchOrcaFake) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	f.trace = append(f.trace, "dispatched-task-list")
	f.dispatchedTaskListCalls++
	return append([]port.OrcaTask(nil), f.dispatchedTasks...), f.dispatchedTaskListErr
}

func (f *dispatchOrcaFake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.trace = append(f.trace, "task-create")
	f.taskCreates++
	if f.beforeTaskCreate != nil {
		f.beforeTaskCreate()
	}
	task := f.task
	if task.Title == "" {
		task.Title = req.Title
	}
	if task.DisplayName == "" {
		task.DisplayName = req.DisplayName
	}
	return task, f.taskErr
}

func (f *dispatchOrcaFake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch")
	f.dispatchCalls++
	f.dispatchRequests = append(f.dispatchRequests, req)
	if f.beforeDispatch != nil {
		f.beforeDispatch()
	}
	result := f.dispatch
	if result.Preamble == "" {
		result.Preamble = fmt.Sprintf("Your coordinator's terminal handle is: %s\nYour task ID is: %s\n  --task-id %s --dispatch-id %s", req.FromHandle, result.TaskID, result.TaskID, result.ID)
	}
	return result, f.dispatchErr
}

func (f *dispatchOrcaFake) ShowDispatch(_ context.Context, taskID string) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch-show")
	if f.dispatchByTask != nil {
		if dispatch, ok := f.dispatchByTask[taskID]; ok {
			return dispatch, f.dispatchShowErr
		}
	}
	return f.dispatch, f.dispatchShowErr
}

func (f *dispatchOrcaFake) ShowDispatchFrom(_ context.Context, taskID, fromHandle string) (port.OrcaDispatch, error) {
	f.trace = append(f.trace, "dispatch-show-from")
	result := f.dispatch
	if result.Preamble == "" {
		result.Preamble = fmt.Sprintf("Your coordinator's terminal handle is: %s\nYour task ID is: %s\n  --task-id %s --dispatch-id %s", fromHandle, taskID, taskID, result.ID)
	}
	return result, f.dispatchShowErr
}

func handoffDispatchRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "example")
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, "16-demo")
	plan := filepath.Join(worktree, "plans", "plan.md")
	writeIssueOpsFile(t, worktree, "plans/plan.md", "# plan\n")
	record := IssueOpsRecord{
		SchemaVersion:       IssueOpsCurrentSchemaVersion,
		ID:                  NewIssueOpsID(repo, "16-demo"),
		Repo:                repo,
		Branch:              "16-demo",
		IssueURL:            "https://github.com/acme/repo/issues/16",
		Phase:               IssueOpsPhaseCompatibilityReview,
		PlanPath:            plan,
		WorktreePath:        worktree,
		Intent:              issueOpsIntentContractForTest(),
		DesignReview:        issueOpsDesignReviewForTest(),
		ExecutionDecision:   issueOpsExecutionDecisionForTest(),
		CompatibilityReview: issueOpsCompatibilityReviewForTest(),
		DevilsAdvocateReview: &IssueOpsDevilsAdvocateReview{
			Verdict: "pass", RecordedAt: "2026-07-11T00:00:00Z",
		},
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: "16-demo", BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true},
		WorktreeTools: &IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: worktree, PreparedAt: "2026-07-11T00:00:00Z"},
		ExecutionHandoff: &IssueOpsExecutionHandoff{
			ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateCoordinatorPreparing, Attempt: 1, OwnershipEpoch: "epoch-1", Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worktree,
			AttemptBaseHead: strings.Repeat("a", 40),
			Orca: &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
				WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worktree},
		},
		CreatedAt: "2026-07-11T00:00:00Z", UpdatedAt: "2026-07-11T00:00:00Z",
	}
	materializeHandoffGit(t, &record)
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, got
}

func handoffStartTestClock() IssueOpsHandoffStartClock {
	return IssueOpsHandoffStartClock{Now: func() time.Time { return time.Date(2026, 7, 11, 2, 3, 4, 0, time.UTC) }}
}

func validCompletedHandoffResultForTest(record IssueOpsRecord) *IssueOpsExecutionHandoffResult {
	return &IssueOpsExecutionHandoffResult{
		Outcome: handoff.OutcomeCompleted, FinalHead: strings.Repeat("b", 40),
		ChangedFiles: []string{".agent-harness/research/report.md"}, TuringReportPath: ".agent-harness/research/report.md",
		Verification: []string{"focused verification passed"}, CleanupReceipts: []string{"worker resources handed off"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}

func validFailedHandoffResultForTest(record IssueOpsRecord) *IssueOpsExecutionHandoffResult {
	return &IssueOpsExecutionHandoffResult{
		Outcome: handoff.OutcomeFailed, Verification: []string{"failure reproduced"}, CleanupReceipts: []string{"worker resources stopped"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}

func setRecoveryRequiredForTest(record *IssueOpsRecord, pending IssueOpsExecutionHandoffPendingOperation) {
	if pending.StartedAt == "" {
		pending.StartedAt = "2026-07-11T00:00:01Z"
	}
	record.ExecutionHandoff.State = handoff.StateRecoveryRequired
	record.ExecutionHandoff.PendingOperation = &pending
	record.ExecutionHandoff.Failure = &IssueOpsExecutionHandoffFailure{
		Code: "test_recovery", Message: "reconcile the exact pending operation", At: pending.StartedAt,
	}
}
