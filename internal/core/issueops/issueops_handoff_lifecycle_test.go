package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

type workerDoneProjectionFake struct {
	mu       sync.Mutex
	calls    int
	requests []port.OrcaWorkerDoneRequest
	result   port.OrcaWorkerDoneResult
	err      error
}

func (f *workerDoneProjectionFake) SendWorkerDone(_ context.Context, req port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func (f *workerDoneProjectionFake) snapshot() (int, []port.OrcaWorkerDoneRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, append([]port.OrcaWorkerDoneRequest(nil), f.requests...)
}

func TestHandoffFinishProjectsWorkerDoneOnceFromPersistedEvidence(t *testing.T) {
	stateRoot, record, _, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	t.Setenv("ORCA_TASK_ID", "task-attacker")
	t.Setenv("ORCA_DISPATCH_ID", "dispatch-attacker")
	t.Setenv("ORCA_TERMINAL_HANDLE", "term_attacker")
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-1", Sequence: 91}}

	first, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	calls, requests := client.snapshot()
	projection := first.ExecutionHandoff.WorkerDoneProjection
	if calls != 1 || len(requests) != 1 || projection == nil || projection.State != "sent" || !projection.Invoked || projection.MessageID != "msg-1" || projection.MessageSequence != 91 {
		t.Fatalf("worker_done projection = %#v calls=%d requests=%#v", projection, calls, requests)
	}
	request := requests[0]
	wantReport := filepath.Join(record.WorktreePath, filepath.FromSlash(finish.TuringReportPath))
	if request.FromHandle != first.ExecutionHandoff.Orca.WorkerMailboxHandle || request.ToHandle != first.ExecutionHandoff.CoordinatorMailboxHandle || request.TaskID != first.ExecutionHandoff.Result.TaskID || request.DispatchID != first.ExecutionHandoff.Result.DispatchID || request.ReportPath != wantReport || !reflect.DeepEqual(request.ChangedFiles, first.ExecutionHandoff.Result.ChangedFiles) || !strings.Contains(request.Body, finish.FinalHead) {
		t.Fatalf("worker_done payload was not derived from persisted evidence: request=%#v handoff=%#v", request, first.ExecutionHandoff)
	}

	second, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	if calls, _ := client.snapshot(); calls != 1 || !reflect.DeepEqual(first.ExecutionHandoff.WorkerDoneProjection, second.ExecutionHandoff.WorkerDoneProjection) {
		t.Fatalf("duplicate finish replayed worker_done: calls=%d first=%#v second=%#v", calls, first.ExecutionHandoff.WorkerDoneProjection, second.ExecutionHandoff.WorkerDoneProjection)
	}
}

func TestHandoffFinishProjectionFailureIsTerminalAndNeverRetries(t *testing.T) {
	stateRoot, _, _, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	client := &workerDoneProjectionFake{err: &port.OrcaError{Code: "command_timeout", Invoked: true, Timeout: true}}
	first, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	projection := first.ExecutionHandoff.WorkerDoneProjection
	if projection == nil || projection.State != "failed" || !projection.Invoked || projection.DiagnosticCode != "command_timeout" || first.ExecutionHandoff.State != handoff.StateSubmitted {
		t.Fatalf("ambiguous send did not remain submitted with terminal diagnostic: %#v", first.ExecutionHandoff)
	}
	if _, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client); err != nil {
		t.Fatal(err)
	}
	if calls, _ := client.snapshot(); calls != 1 {
		t.Fatalf("ambiguous send was retried: %d", calls)
	}
}

func TestHandoffFinishProjectionPreconditionsNeverCallOrca(t *testing.T) {
	stateRoot, record, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	submitted.ExecutionHandoff.CoordinatorMailboxHandle = ""
	if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
		t.Fatal(err)
	}
	client := &workerDoneProjectionFake{}
	got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	if calls, _ := client.snapshot(); calls != 0 || got.ExecutionHandoff.WorkerDoneProjection == nil || got.ExecutionHandoff.WorkerDoneProjection.State != "failed" || got.ExecutionHandoff.WorkerDoneProjection.Invoked {
		t.Fatalf("failed precondition called Orca or lacked diagnostic: calls=%d projection=%#v", calls, got.ExecutionHandoff.WorkerDoneProjection)
	}
	if got.ExecutionHandoff.State != handoff.StateSubmitted || got.ExecutionHandoff.Result.FinalHead != finish.FinalHead || record.ID != got.ID {
		t.Fatalf("projection precondition changed durable submit authority: %#v", got.ExecutionHandoff)
	}
}

func TestHandoffFinishWrongCoordinatorRecipientNeverCallsProjection(t *testing.T) {
	for name, recipient := range map[string]string{
		"group target":       "@all",
		"worker self target": "term_worker",
	} {
		t.Run(name, func(t *testing.T) {
			stateRoot, _, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
			if name == "worker self target" {
				recipient = submitted.ExecutionHandoff.Orca.WorkerMailboxHandle
			}
			submitted.ExecutionHandoff.CoordinatorMailboxHandle = recipient
			if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
				t.Fatal(err)
			}
			client := &workerDoneProjectionFake{}
			got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
			if err != nil {
				t.Fatal(err)
			}
			if calls, _ := client.snapshot(); calls != 0 || got.ExecutionHandoff.WorkerDoneProjection == nil || got.ExecutionHandoff.WorkerDoneProjection.State != "failed" || got.ExecutionHandoff.WorkerDoneProjection.Invoked {
				t.Fatalf("wrong coordinator recipient called Orca or lacked a terminal diagnostic: calls=%d projection=%#v", calls, got.ExecutionHandoff.WorkerDoneProjection)
			}
		})
	}
}

func TestHandoffFinishConcurrentIdenticalCallsProjectOnce(t *testing.T) {
	stateRoot, _, _, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-1", Sequence: 92}}
	const count = 8
	results := make(chan IssueOpsRecord, count)
	errs := make(chan error, count)
	var wait sync.WaitGroup
	for range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
			results <- got
			errs <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent identical finish failed: %v", err)
		}
	}
	for got := range results {
		if got.ExecutionHandoff == nil || got.ExecutionHandoff.WorkerDoneProjection == nil {
			t.Fatalf("concurrent finish returned incomplete projection: %#v", got)
		}
	}
	if calls, _ := client.snapshot(); calls != 1 {
		t.Fatalf("concurrent identical finishes projected %d times", calls)
	}
}

func TestHandoffFinishProjectionUsesSealedWorkerMailboxAfterLiveTerminalRollover(t *testing.T) {
	stateRoot, _, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	submitted.ExecutionHandoff.Orca.WorkerMailboxHandle = "term_sealed_worker"
	submitted.ExecutionHandoff.Orca.WorkerTerminalHandle = "term_live_worker"
	if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
		t.Fatal(err)
	}
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-rollover", Sequence: 93}}
	got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	_, requests := client.snapshot()
	if len(requests) != 1 || requests[0].FromHandle != "term_sealed_worker" || got.ExecutionHandoff.Orca.WorkerMailboxHandle != "term_sealed_worker" || got.ExecutionHandoff.Orca.WorkerTerminalHandle != "term_live_worker" {
		t.Fatalf("runtime rollover changed or bypassed the sealed worker mailbox: request=%#v orca=%#v", requests, got.ExecutionHandoff.Orca)
	}
}

func TestHandoffFinishPersistsSubmittedAndProjectionIntentAtCrashBoundary(t *testing.T) {
	stateRoot, _, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	submitted.ExecutionHandoff.State = handoff.StateClaimed
	submitted.ExecutionHandoff.Result = nil
	submitted.ExecutionHandoff.CompletedAt = ""
	submitted.ExecutionHandoff.WorkerDoneProjection = nil
	if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
		t.Fatal(err)
	}
	crash := fmt.Errorf("simulated crash after durable submit")
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "must-not-send", Sequence: 95}}
	_, err := finishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client, issueOpsHandoffProjectionHooks{
		AfterDurableSubmitAndProjectionIntent: func(record IssueOpsRecord) error {
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.ExecutionHandoff.State != handoff.StateSubmitted || persisted.ExecutionHandoff.Result == nil || persisted.ExecutionHandoff.WorkerDoneProjection == nil || persisted.ExecutionHandoff.WorkerDoneProjection.State != "intent" {
				t.Fatalf("crash boundary exposed submitted authority without projection intent: %#v", persisted.ExecutionHandoff)
			}
			return crash
		},
	})
	if !errors.Is(err, crash) {
		t.Fatalf("crash hook error = %v", err)
	}
	if calls, _ := client.snapshot(); calls != 0 {
		t.Fatalf("crash boundary invoked worker_done %d times", calls)
	}
	if _, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client); err != nil {
		t.Fatal(err)
	}
	if calls, _ := client.snapshot(); calls != 0 {
		t.Fatalf("persisted crash intent was retried %d times", calls)
	}
}

func TestHandoffFinishProjectionRevalidatesExactWorkerEvidenceInsideSubmitLock(t *testing.T) {
	stateRoot, record, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	submitted.ExecutionHandoff.State = handoff.StateClaimed
	submitted.ExecutionHandoff.Result = nil
	submitted.ExecutionHandoff.CompletedAt = ""
	submitted.ExecutionHandoff.WorkerDoneProjection = nil
	if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
		t.Fatal(err)
	}
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "must-not-send", Sequence: 96}}
	got, err := finishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client, issueOpsHandoffProjectionHooks{
		BeforeLockedRevalidation: func() {
			writeIssueOpsFile(t, record.WorktreePath, "late-uncommitted-drift.txt", "drift after optimistic validation\n")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := got.ExecutionHandoff.WorkerDoneProjection
	if calls, _ := client.snapshot(); calls != 0 || got.ExecutionHandoff.State != handoff.StateSubmitted || projection == nil || projection.State != "failed" || projection.Invoked || projection.DiagnosticCode != "worker_evidence_invalid" {
		t.Fatalf("locked evidence drift was not terminalized before send: calls=%d handoff=%#v", calls, got.ExecutionHandoff)
	}
}

func TestHandoffFinishCrashIntentRecoveryNeverRetries(t *testing.T) {
	stateRoot, _, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	_, projection, code, err := buildWorkerDoneProjection(submitted)
	if err != nil || code != "" {
		t.Fatalf("build projection intent: code=%q err=%v", code, err)
	}
	submitted.ExecutionHandoff.WorkerDoneProjection = &projection
	if _, err := WriteIssueOps(stateRoot, submitted); err != nil {
		t.Fatal(err)
	}
	client := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-must-not-send", Sequence: 94}}
	got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client)
	if err != nil {
		t.Fatal(err)
	}
	if calls, _ := client.snapshot(); calls != 0 || got.ExecutionHandoff.WorkerDoneProjection.State != "intent" || got.ExecutionHandoff.WorkerDoneProjection.Invoked {
		t.Fatalf("crash-intent recovery retried or rewrote ambiguous intent: calls=%d projection=%#v", calls, got.ExecutionHandoff.WorkerDoneProjection)
	}
}

func TestHandoffEnvelopeRejectsWorkerDoneProjectionWithoutStartedAt(t *testing.T) {
	stateRoot, _, _, _, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	_, projection, code, err := buildWorkerDoneProjection(submitted)
	if err != nil || code != "" {
		t.Fatalf("build projection intent: code=%q err=%v", code, err)
	}
	projection.StartedAt = ""
	submitted.ExecutionHandoff.WorkerDoneProjection = &projection
	if _, err := WriteIssueOps(stateRoot, submitted); err == nil || !strings.Contains(err.Error(), "worker_done projection") {
		t.Fatalf("projection without started_at was accepted: %v", err)
	}
}

func TestHandoffFinishProjectionPreservesGitLabProviderWithoutRemoteMutation(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	issueURL := "https://gitlab.example.com/acme/repo/-/issues/16"
	record.IssueURL = issueURL
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = issueURL
	record.ExecutionHandoff.Orca.ProviderIssueLinkStatus = handoff.ProviderIssueLinkGitLabExact
	var err error
	record, err = WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(t.TempDir(), "glab-invoked")
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "glab"), []byte("#!/bin/sh\nprintf invoked > \"$HARNESS_GITLAB_SENTINEL\"\nexit 97\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HARNESS_GITLAB_SENTINEL", sentinel)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	dispatchClient := handoffDispatchFake(record)
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), dispatchClient, handoffStartTestClock()); err != nil {
		t.Fatal(err)
	}
	dispatched, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforePrepare := *dispatched.BranchPrepare
	beforeIssueURL := dispatched.IssueURL
	beforeLinkStatus := dispatched.ExecutionHandoff.Orca.ProviderIssueLinkStatus
	claim := handoffClaimRequest(dispatched)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, dispatched.WorktreePath, "internal/gitlab-demo.go", "package internal\n")
	reportPath := filepath.Join(dispatched.WorktreePath, ".agent-harness", "research", "gitlab-report.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, []byte("# GitLab projection evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: GitLab worker result"}} {
		if code, _, stderr := preflight.GitCmd(dispatched.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.TrimSpace(preflight.GitOut(dispatched.WorktreePath, "rev-parse", "HEAD"))
	finish.ChangedFiles = []string{"internal/gitlab-demo.go", ".agent-harness/research/gitlab-report.md"}
	finish.TuringReportPath = ".agent-harness/research/gitlab-report.md"
	projectionClient := &workerDoneProjectionFake{result: port.OrcaWorkerDoneResult{MessageID: "msg-gitlab", Sequence: 97}}
	got, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, projectionClient)
	if err != nil {
		t.Fatal(err)
	}
	if calls, _ := projectionClient.snapshot(); calls != 1 || got.ExecutionHandoff.State != handoff.StateSubmitted || got.ExecutionHandoff.WorkerDoneProjection == nil || got.ExecutionHandoff.WorkerDoneProjection.State != "sent" {
		t.Fatalf("GitLab completed finish did not use sealed projection once: calls=%d handoff=%#v", calls, got.ExecutionHandoff)
	}
	if got.IssueURL != beforeIssueURL || !reflect.DeepEqual(*got.BranchPrepare, beforePrepare) || got.ExecutionHandoff.Orca.ProviderIssueLinkStatus != beforeLinkStatus {
		t.Fatalf("GitLab provider authority changed during finish: before=%#v after=%#v", beforePrepare, got.BranchPrepare)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("GitLab remote mutation command was invoked: %v", err)
	}
}

func TestHandoffFinishWrongTaskOrDispatchNeverCallsProjection(t *testing.T) {
	for _, field := range []string{"task", "dispatch"} {
		t.Run(field, func(t *testing.T) {
			stateRoot, _, _, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
			if field == "task" {
				finish.TaskID = "task-other"
			} else {
				finish.DispatchID = "dispatch-other"
			}
			client := &workerDoneProjectionFake{}
			if _, err := FinishIssueOpsHandoffWithProjection(context.Background(), stateRoot, finish, client); err == nil {
				t.Fatalf("wrong %s identity was accepted", field)
			}
			if calls, _ := client.snapshot(); calls != 0 {
				t.Fatalf("wrong %s identity called projection %d times", field, calls)
			}
		})
	}
}

func TestHandoffClaimRequiresMatchingWorkerTuple(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	base := handoffClaimRequest(record)
	tests := []struct {
		name   string
		mutate func(*IssueOpsHandoffClaimRequest)
	}{
		{name: "attempt", mutate: func(r *IssueOpsHandoffClaimRequest) { r.Attempt++ }},
		{name: "epoch", mutate: func(r *IssueOpsHandoffClaimRequest) { r.OwnershipEpoch = "stale" }},
		{name: "context", mutate: func(r *IssueOpsHandoffClaimRequest) { r.ContextSHA256 = "stale" }},
		{name: "session", mutate: func(r *IssueOpsHandoffClaimRequest) { r.SessionID = "" }},
		{name: "root", mutate: func(r *IssueOpsHandoffClaimRequest) { r.CWD = record.Repo }},
		{name: "worktree", mutate: func(r *IssueOpsHandoffClaimRequest) { r.OrcaWorktreeID = "wrong" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			tt.mutate(&req)
			if _, err := ClaimIssueOpsHandoff(stateRoot, req); err == nil {
				t.Fatal("expected fenced claim rejection")
			}
		})
	}
}

func TestHandoffClaimIsIdempotentForSameOwnerOnly(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	req := handoffClaimRequest(record)
	first, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/after-claim.go", "package internal\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: commit after claim"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	second, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ExecutionHandoff.State != handoff.StateClaimed || second.ExecutionHandoff.WorkerSession.SessionID != req.SessionID {
		t.Fatalf("unexpected claim result: %#v", second.ExecutionHandoff)
	}
	other := req
	other.SessionID = "other"
	if _, err := ClaimIssueOpsHandoff(stateRoot, other); err == nil {
		t.Fatal("different owner must not steal claim")
	}
}

func TestHandoffClaimRejectsRerenderedContextSourceDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string, *IssueOpsRecord)
	}{
		{name: "plan", mutate: func(t *testing.T, _ string, record *IssueOpsRecord) {
			if err := os.WriteFile(record.PlanPath, []byte("changed after dispatch\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "intent", mutate: func(t *testing.T, stateRoot string, record *IssueOpsRecord) {
			record.Intent.InterpretedIntent = "changed after dispatch"
			updated, err := WriteIssueOps(stateRoot, *record)
			if err != nil {
				t.Fatal(err)
			}
			*record = updated
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			tt.mutate(t, stateRoot, &record)
			if _, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(record)); err == nil || !strings.Contains(err.Error(), "context source") {
				t.Fatalf("claim must reject %s drift, got %v", tt.name, err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.State != handoff.StateDispatched {
				t.Fatalf("rejected claim advanced state: %#v", persisted.ExecutionHandoff)
			}
		})
	}
}

func TestHandoffClaimRevalidatesCleanExactCheckpointInsideLock(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	hooks := issueOpsHandoffLifecycleHooks{BeforeLockedRevalidation: func() {
		writeIssueOpsFile(t, record.WorktreePath, "claim-lock-drift.txt", "dirty after outer validation\n")
	}}
	if _, err := claimIssueOpsHandoff(stateRoot, handoffClaimRequest(record), hooks); err == nil || !strings.Contains(err.Error(), "clean worker worktree") {
		t.Fatalf("claim lock revalidation error = %v", err)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("failed claim lock revalidation mutated the lease")
	}
}

func TestHandoffFinishRejectsRerenderedContextSourceDrift(t *testing.T) {
	for _, source := range []string{"plan", "intent"} {
		t.Run(source, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			claim := handoffClaimRequest(record)
			if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
				t.Fatal(err)
			}
			if source == "plan" {
				if err := os.WriteFile(record.PlanPath, []byte("changed after claim\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				record, _ = ReadIssueOps(stateRoot, record.ID)
				record.Intent.InterpretedIntent = "changed after claim"
				if _, err := WriteIssueOps(stateRoot, record); err != nil {
					t.Fatal(err)
				}
			}
			_, err := finishIssueOpsHandoffWithoutProjection(stateRoot, IssueOpsHandoffFinishRequest{
				ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
				Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
				Verification: []string{"failure observed"}, CleanupReceipts: []string{"resources stopped"},
				TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
			})
			if err == nil || !strings.Contains(err.Error(), "context source") {
				t.Fatalf("finish must reject %s drift, got %v", source, err)
			}
		})
	}
}

func TestHandoffFinishRevalidatesContextSourceInsideLock(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	req := handoffFinishRequest(claim, claimed)
	req.Outcome = handoff.OutcomeFailed
	req.FinalHead = ""
	req.ChangedFiles = nil
	req.TuringReportPath = ""
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	hooks := issueOpsHandoffLifecycleHooks{BeforeLockedRevalidation: func() {
		if err := os.WriteFile(record.PlanPath, []byte("changed after outer finish validation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}}
	if _, err := finishIssueOpsHandoff(stateRoot, req, hooks); err == nil || !strings.Contains(err.Error(), "stale handoff context source fingerprint") {
		t.Fatalf("finish lock revalidation error = %v", err)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if string(after) != string(before) {
		t.Fatal("failed finish lock revalidation mutated the lease")
	}
}

func TestHandoffFinishRequiresExactOrcaTaskDispatchTuple(t *testing.T) {
	tests := []struct {
		name             string
		corruptPersisted bool
		mutate           func(*IssueOpsRecord, *IssueOpsHandoffFinishRequest)
	}{
		{name: "omitted task", mutate: func(_ *IssueOpsRecord, r *IssueOpsHandoffFinishRequest) { r.TaskID = "" }},
		{name: "omitted dispatch", mutate: func(_ *IssueOpsRecord, r *IssueOpsHandoffFinishRequest) { r.DispatchID = "" }},
		{name: "missing persisted task", corruptPersisted: true, mutate: func(record *IssueOpsRecord, _ *IssueOpsHandoffFinishRequest) {
			record.ExecutionHandoff.Orca.TaskID = ""
		}},
		{name: "missing persisted dispatch", corruptPersisted: true, mutate: func(record *IssueOpsRecord, _ *IssueOpsHandoffFinishRequest) {
			record.ExecutionHandoff.Orca.DispatchID = ""
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _ := dispatchedHandoffRecord(t)
			claim := handoffClaimRequest(record)
			claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
			if err != nil {
				t.Fatal(err)
			}
			req := handoffFinishRequest(claim, claimed)
			tt.mutate(&claimed, &req)
			if tt.corruptPersisted {
				putRawIssueOpsRecordForTest(t, stateRoot, claimed)
			} else if _, err := WriteIssueOps(stateRoot, claimed); err != nil {
				t.Fatal(err)
			}
			if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, req); err == nil {
				t.Fatal("finish must require exact nonempty persisted and submitted Orca identities")
			}
		})
	}
}

func TestHandoffHeartbeatFencesStaleWorker(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	req := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, req)
	if err != nil {
		t.Fatal(err)
	}
	hb := IssueOpsHeartbeatRequest{ID: claimed.ID, Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256, Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}
	updated, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, hb)
	if err != nil || updated.ExecutionHandoff.LastHeartbeatAt == "" {
		t.Fatalf("heartbeat failed: %#v err=%v", updated.ExecutionHandoff, err)
	}
	hb.SessionID = "stale"
	if _, err := RecordIssueOpsHeartbeatWithRequest(stateRoot, hb); err == nil {
		t.Fatal("stale worker heartbeat must fail")
	}
}

func TestHandoffFinishSubmitAcceptLifecycle(t *testing.T) {
	stateRoot, record, claim, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	if submitted.ExecutionHandoff.State != handoff.StateSubmitted {
		t.Fatalf("expected submitted: %#v", submitted.ExecutionHandoff)
	}
	accepted, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.ExecutionHandoff.State != handoff.StateClosed || accepted.ExecutionHandoff.ClosedDisposition != handoff.DispositionAccepted {
		t.Fatalf("expected accepted close: %#v", accepted.ExecutionHandoff)
	}
}

func TestHandoffAcceptRejectsRerenderedContextSourceDrift(t *testing.T) {
	for _, source := range []string{"plan", "intent"} {
		t.Run(source, func(t *testing.T) {
			stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
			if source == "plan" {
				if err := os.WriteFile(record.PlanPath, []byte("changed after submission\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				record.Intent.InterpretedIntent = "changed after submission"
				if _, err := WriteIssueOps(stateRoot, record); err != nil {
					t.Fatal(err)
				}
			}
			_, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
				ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
				ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
			})
			if err == nil || !strings.Contains(err.Error(), "context source") {
				t.Fatalf("accept must reject %s drift, got %v", source, err)
			}
		})
	}
}

func TestHandoffAcceptRevalidatesFilesystemEvidenceInsideLock(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, IssueOpsRecord, IssueOpsHandoffFinishRequest)
		want   string
	}{
		{
			name: "dirty worktree",
			mutate: func(t *testing.T, record IssueOpsRecord, _ IssueOpsHandoffFinishRequest) {
				writeIssueOpsFile(t, record.WorktreePath, "accept-lock-drift.txt", "dirty after outer validation\n")
			},
			want: "clean before accept",
		},
		{
			name: "moved head",
			mutate: func(t *testing.T, record IssueOpsRecord, _ IssueOpsHandoffFinishRequest) {
				writeIssueOpsFile(t, record.WorktreePath, "accept-head-drift.txt", "committed after outer validation\n")
				for _, args := range [][]string{{"add", "accept-head-drift.txt"}, {"commit", "-q", "-m", "test: move accepted head"}} {
					if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
						t.Fatalf("git %v failed: %s", args, stderr)
					}
				}
			},
			want: "current worktree head",
		},
		{
			name: "removed report",
			mutate: func(t *testing.T, record IssueOpsRecord, finish IssueOpsHandoffFinishRequest) {
				if err := os.Remove(filepath.Join(record.WorktreePath, finish.TuringReportPath)); err != nil {
					t.Fatal(err)
				}
			},
			want: "Turing report does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			hooks := issueOpsHandoffLifecycleHooks{BeforeLockedRevalidation: func() { tt.mutate(t, record, finish) }}
			_, err := acceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
				ID: record.ID, Attempt: submitted.ExecutionHandoff.Attempt, OwnershipEpoch: submitted.ExecutionHandoff.OwnershipEpoch,
				ContextSHA256: submitted.ExecutionHandoff.ContextSHA256, FinalHead: finish.FinalHead,
			}, hooks)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("accept lock revalidation error = %v, want %q", err, tt.want)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if string(after) != string(before) {
				t.Fatal("failed accept lock revalidation mutated the lease")
			}
		})
	}
}

func TestHandoffAcceptRequiresCleanWorkerAndCanonicalReport(t *testing.T) {
	t.Run("dirty worktree", func(t *testing.T) {
		stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
		writeIssueOpsFile(t, record.WorktreePath, "dirty.txt", "dirty\n")
		_, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
			ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
			ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
		})
		if err == nil || !strings.Contains(err.Error(), "clean") {
			t.Fatalf("dirty worker worktree must reject accept, got %v", err)
		}
	})

	for _, tt := range []struct {
		name, report string
		create       bool
	}{
		{name: "missing report", report: ".agent-harness/research/missing.md"},
		{name: "outside report", report: filepath.Join(t.TempDir(), "outside.md"), create: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := tt.report
			if filepath.IsAbs(report) {
				report = ".agent-harness/research/report.md"
			}
			stateRoot, record, claim, finish, submitted := submittedGitHandoff(t, report, tt.create)
			if filepath.IsAbs(tt.report) {
				submitted.ExecutionHandoff.Result.TuringReportPath = tt.report
				putRawIssueOpsRecordForTest(t, stateRoot, submitted)
			}
			_, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
				ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
				ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
			})
			if err == nil {
				t.Fatalf("%s must reject accept, got %v", tt.name, err)
			}
		})
	}
}

func TestHandoffFinishRequiresSafeRelativeTuringReportPath(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.Repeat("b", 40)
	finish.TuringReportPath = filepath.Join(t.TempDir(), "report.md")
	finish.ChangedFiles = []string{".agent-harness/research/report.md"}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err == nil || !strings.Contains(strings.ToLower(err.Error()), "relative") {
		t.Fatalf("absolute Turing report must fail at finish with a relative-path diagnostic, got %v", err)
	}
}

func TestHandoffAcceptRejectsCommittedLeafSymlinkReport(t *testing.T) {
	stateRoot, record := gitBackedDispatchedHandoff(t)
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/demo.go", "package internal\n")
	researchDir := filepath.Join(record.WorktreePath, ".agent-harness", "research")
	if err := os.MkdirAll(researchDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(researchDir, "actual.md"), []byte("# evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("actual.md", filepath.Join(researchDir, "report.md")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: symlink report"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))
	finish.TuringReportPath = ".agent-harness/research/report.md"
	finish.ChangedFiles = []string{".agent-harness/research/actual.md", ".agent-harness/research/report.md", "internal/demo.go"}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err != nil {
		t.Fatal(err)
	}
	_, err = AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
		ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("committed leaf symlink must not stand in for Turing report content, got %v", err)
	}
}

func TestHandoffAcceptValidatesPersistedCompletedEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IssueOpsExecutionHandoffResult)
	}{
		{name: "outcome", mutate: func(r *IssueOpsExecutionHandoffResult) { r.Outcome = "" }},
		{name: "verification", mutate: func(r *IssueOpsExecutionHandoffResult) { r.Verification = nil }},
		{name: "whitespace verification", mutate: func(r *IssueOpsExecutionHandoffResult) { r.Verification = []string{"   "} }},
		{name: "cleanup receipts", mutate: func(r *IssueOpsExecutionHandoffResult) { r.CleanupReceipts = nil }},
		{name: "whitespace cleanup receipts", mutate: func(r *IssueOpsExecutionHandoffResult) { r.CleanupReceipts = []string{" \t "} }},
		{name: "task identity", mutate: func(r *IssueOpsExecutionHandoffResult) { r.TaskID = "task-other" }},
		{name: "omitted task identity", mutate: func(r *IssueOpsExecutionHandoffResult) { r.TaskID = "" }},
		{name: "dispatch identity", mutate: func(r *IssueOpsExecutionHandoffResult) { r.DispatchID = "dispatch-other" }},
		{name: "omitted dispatch identity", mutate: func(r *IssueOpsExecutionHandoffResult) { r.DispatchID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record, claim, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
			if submitted.ExecutionHandoff.Result == nil || submitted.ExecutionHandoff.Result.TuringReportPath != finish.TuringReportPath || len(submitted.ExecutionHandoff.Result.Verification) == 0 || len(submitted.ExecutionHandoff.Result.CleanupReceipts) == 0 {
				t.Fatalf("finish did not preserve the completed evidence tuple: %#v", submitted.ExecutionHandoff.Result)
			}
			tt.mutate(submitted.ExecutionHandoff.Result)
			putRawIssueOpsRecordForTest(t, stateRoot, submitted)
			if _, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
				ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
				ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
			}); err == nil {
				t.Fatal("accept must reject a corrupted completed evidence tuple")
			}
		})
	}
}

func TestHandoffFinishFailureClosesWorkerFailed(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	claim := handoffClaimRequest(record)
	if _, err := ClaimIssueOpsHandoff(stateRoot, claim); err != nil {
		t.Fatal(err)
	}
	failed, err := finishIssueOpsHandoffWithoutProjection(stateRoot, IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeFailed,
		Verification: []string{"go test failed"}, CleanupReceipts: []string{"temp state removed"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if failed.ExecutionHandoff.ClosedDisposition != handoff.DispositionWorkerFailed {
		t.Fatalf("unexpected failure close: %#v", failed.ExecutionHandoff)
	}
}

func TestHandoffFinishAndAcceptIdempotency(t *testing.T) {
	stateRoot, record, claim, finish, first := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	second, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish)
	if err != nil || !reflect.DeepEqual(first.ExecutionHandoff.Result, second.ExecutionHandoff.Result) {
		t.Fatalf("finish not idempotent: err=%v", err)
	}
	accept := IssueOpsHandoffAcceptRequest{ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead}
	if _, err := AcceptIssueOpsHandoff(stateRoot, accept); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(record.WorktreePath); err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptIssueOpsHandoff(stateRoot, accept); err != nil {
		t.Fatalf("accepted retry after worktree cleanup not idempotent: %v", err)
	}
	conflictingAccept := accept
	conflictingAccept.FinalHead = "conflict"
	if _, err := AcceptIssueOpsHandoff(stateRoot, conflictingAccept); err == nil {
		t.Fatal("conflicting accepted head must fail")
	}
	conflictingAccept = accept
	conflictingAccept.Attempt++
	if _, err := AcceptIssueOpsHandoff(stateRoot, conflictingAccept); err == nil {
		t.Fatal("conflicting accepted fence must fail")
	}
	finish.FinalHead = "conflict"
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err == nil {
		t.Fatal("conflicting finish must fail")
	}
}

func TestHandoffFinishIdempotencySurvivesEvidenceCleanup(t *testing.T) {
	stateRoot, record, _, finish, submitted := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
	if err := os.RemoveAll(record.WorktreePath); err != nil {
		t.Fatal(err)
	}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err != nil {
		t.Fatalf("exact same-worker result retry after cleanup failed: %v", err)
	}
	finish.Verification = []string{"conflicting evidence"}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err == nil {
		t.Fatalf("conflicting finish repeat succeeded: %#v", submitted.ExecutionHandoff.Result)
	}
}

func TestHandoffResumeIsReadOnly(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := IssueOpsStateRoot()
	_, source, _ := dispatchedHandoffRecordAt(t, stateRoot)
	before, _ := json.Marshal(source)
	result := IssueOpsResume(source.Repo, source.ID)
	if !result.OK || result.ExecutionHandoff == nil || result.ExecutionHandoff.State != handoff.StateDispatched {
		t.Fatalf("unexpected resume projection: %#v", result)
	}
	afterRecord, err := ReadIssueOps(stateRoot, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("resume mutated durable state")
	}
}

func TestInlineHeartbeatAndResumeRemainCompatible(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "16-inline"})
	if err != nil {
		t.Fatal(err)
	}
	if updated, err := RecordIssueOpsHeartbeat(IssueOpsStateRoot(), record.ID); err != nil || updated.LastHeartbeatAt == "" {
		t.Fatalf("inline heartbeat changed: %#v err=%v", updated, err)
	}
	if result := IssueOpsResume(repo, record.ID); !result.OK {
		t.Fatalf("inline resume changed: %#v", result)
	}
}

func dispatchedHandoffRecord(t *testing.T) (string, IssueOpsRecord, *dispatchOrcaFake) {
	t.Helper()
	return dispatchedHandoffRecordAt(t, t.TempDir())
}

func dispatchedHandoffRecordAt(t *testing.T, stateRoot string) (string, IssueOpsRecord, *dispatchOrcaFake) {
	t.Helper()
	_, record := handoffDispatchRecord(t)
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(record)
	dispatched, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, record.ID), client, handoffStartTestClock())
	if err != nil {
		t.Fatal(err)
	}
	result, err := ReadIssueOps(stateRoot, dispatched.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, result, client
}

func handoffClaimRequest(record IssueOpsRecord) IssueOpsHandoffClaimRequest {
	return IssueOpsHandoffClaimRequest{
		ID: record.ID, Attempt: record.ExecutionHandoff.Attempt, OwnershipEpoch: record.ExecutionHandoff.OwnershipEpoch,
		ContextSHA256: record.ExecutionHandoff.ContextSHA256, Host: "codex", SessionID: "session-1", AgentID: "codex-worker",
		CWD: record.WorktreePath, OrcaWorktreeID: record.ExecutionHandoff.Orca.WorktreeID,
	}
}

func handoffFinishRequest(claim IssueOpsHandoffClaimRequest, record IssueOpsRecord) IssueOpsHandoffFinishRequest {
	return IssueOpsHandoffFinishRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch, ContextSHA256: claim.ContextSHA256,
		Host: claim.Host, SessionID: claim.SessionID, AgentID: claim.AgentID, Outcome: handoff.OutcomeCompleted,
		FinalHead: "head-1", ChangedFiles: []string{"internal/demo.go"}, TuringReportPath: ".agent-harness/research/report.md",
		Verification: []string{"go test ./...: pass"}, CleanupReceipts: []string{"temporary state removed"},
		TaskID: record.ExecutionHandoff.Orca.TaskID, DispatchID: record.ExecutionHandoff.Orca.DispatchID,
	}
}

func gitBackedDispatchedHandoff(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	return stateRoot, record
}

func materializeHandoffGit(t *testing.T, record *IssueOpsRecord) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(record.WorktreePath, ".git")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", record.Branch},
		{"config", "user.name", "Handoff Test"},
		{"config", "user.email", "handoff@example.test"},
		{"add", "-A"},
		{"commit", "-q", "-m", "test: prepare handoff"},
	} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	head := strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))
	record.BranchPrepare.BaseSHA = head
	record.ExecutionHandoff.AttemptBaseHead = head
}

func submittedGitHandoff(t *testing.T, reportPath string, createReport bool) (string, IssueOpsRecord, IssueOpsHandoffClaimRequest, IssueOpsHandoffFinishRequest, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := gitBackedDispatchedHandoff(t)
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/demo.go", "package internal\n")
	if createReport {
		path := reportPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(record.WorktreePath, path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# Turing evidence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: worker result"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))
	finish.TuringReportPath = reportPath
	finish.ChangedFiles = []string{"internal/demo.go"}
	if createReport {
		reportRelative := reportPath
		if filepath.IsAbs(reportRelative) {
			var relErr error
			reportRelative, relErr = filepath.Rel(record.WorktreePath, reportRelative)
			if relErr != nil {
				t.Fatal(relErr)
			}
		}
		finish.ChangedFiles = append(finish.ChangedFiles, filepath.ToSlash(filepath.Clean(reportRelative)))
	}
	submitted, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, claim, finish, submitted
}
