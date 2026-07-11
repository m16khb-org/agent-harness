package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func stopSuppressionRequest(record IssueOpsRecord, worktree, host, session, agent string) HookToolUseLifecycleRequest {
	return HookToolUseLifecycleRequest{Repo: record.Repo, CWD: worktree, Host: host, SessionID: session, AgentID: agent, SourceCheckout: record.Repo}
}

// validWorkerDoneProjection builds a worker_done_projection that satisfies the
// envelope's cross-field evidence checks (validateWorkerDoneProjection in
// internal/core/issueops/handoff/envelope.go) for the given terminal state,
// mirroring the exact shape production's buildWorkerDoneProjection persists.
func validWorkerDoneProjection(t *testing.T, h *issueopsmodel.IssueOpsExecutionHandoff, state string) *issueopsmodel.IssueOpsExecutionHandoffWorkerDoneProjection {
	t.Helper()
	p := &issueopsmodel.IssueOpsExecutionHandoffWorkerDoneProjection{
		Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch,
		StartedAt: "2026-07-11T02:00:00Z", CompletedAt: "2026-07-11T02:00:01Z",
	}
	switch state {
	case "intent":
		p.State = "intent"
		p.DiagnosticCode = "intent_persisted"
		p.CompletedAt = ""
		p.FromHandle = h.Orca.WorkerMailboxHandle
		p.ToHandle = h.CoordinatorMailboxHandle
		p.TaskID = h.Result.TaskID
		p.DispatchID = h.Result.DispatchID
		p.FinalHead = h.Result.FinalHead
		p.ChangedFiles = h.Result.ChangedFiles
		p.ReportPath = filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(h.Result.TuringReportPath)))
		p.HostIdentity = h.WorkerSession.Host + "/" + h.WorkerSession.SessionID
		if h.WorkerSession.AgentID != "" {
			p.HostIdentity += "/" + h.WorkerSession.AgentID
		}
		payload, err := json.Marshal(struct {
			FromHandle   string   `json:"from_handle"`
			ToHandle     string   `json:"to_handle"`
			Subject      string   `json:"subject"`
			Body         string   `json:"body"`
			TaskID       string   `json:"task_id"`
			DispatchID   string   `json:"dispatch_id"`
			ChangedFiles []string `json:"changed_files"`
			ReportPath   string   `json:"report_path"`
		}{p.FromHandle, p.ToHandle, p.Subject, p.Body, p.TaskID, p.DispatchID, p.ChangedFiles, p.ReportPath})
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(payload)
		p.PayloadSHA256 = hex.EncodeToString(sum[:])
	case "failed":
		p.State = "failed"
		p.DiagnosticCode = "orca_send_failed"
	case "sent":
		p.State = "sent"
		p.DiagnosticCode = "sent"
		p.Invoked = true
		p.MessageID = "msg-1"
		p.MessageSequence = 1
		p.FromHandle = h.Orca.WorkerMailboxHandle
		p.ToHandle = h.CoordinatorMailboxHandle
		p.TaskID = h.Result.TaskID
		p.DispatchID = h.Result.DispatchID
		p.FinalHead = h.Result.FinalHead
		p.ChangedFiles = h.Result.ChangedFiles
		p.ReportPath = filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(h.Result.TuringReportPath)))
		p.HostIdentity = h.WorkerSession.Host + "/" + h.WorkerSession.SessionID
		if h.WorkerSession.AgentID != "" {
			p.HostIdentity += "/" + h.WorkerSession.AgentID
		}
		payload, err := json.Marshal(struct {
			FromHandle   string   `json:"from_handle"`
			ToHandle     string   `json:"to_handle"`
			Subject      string   `json:"subject"`
			Body         string   `json:"body"`
			TaskID       string   `json:"task_id"`
			DispatchID   string   `json:"dispatch_id"`
			ChangedFiles []string `json:"changed_files"`
			ReportPath   string   `json:"report_path"`
		}{p.FromHandle, p.ToHandle, p.Subject, p.Body, p.TaskID, p.DispatchID, p.ChangedFiles, p.ReportPath})
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(payload)
		p.PayloadSHA256 = hex.EncodeToString(sum[:])
	default:
		t.Fatalf("unsupported test projection state %q", state)
	}
	return p
}

func TestStopSuppressionExactSubmittedSentProjectionSuppresses(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	record, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	req := stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
	if !SuppressStopNextActionForCompletedWorker(req) {
		t.Fatalf("exact submitted+sent-projection worker must suppress relay/re-entry")
	}
	req.CWD = worktree + string(os.PathSeparator) + "."
	if !SuppressStopNextActionForCompletedWorker(req) {
		t.Fatalf("clean-path equivalent worker cwd must suppress relay/re-entry")
	}
}

func TestStopSuppressionTerminalHandleRolloverIndependence(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	// Terminal handle rollover (a new mailbox handle assigned after the worker
	// exits) must not affect the decision at all: this guard never compares
	// ORCA_TERMINAL_HANDLE against sealed mailbox handles.
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-rolled-over"
	record, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	req := stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
	if !SuppressStopNextActionForCompletedWorker(req) {
		t.Fatalf("terminal handle rollover must not change host/session-identity based suppression")
	}
}

func TestStopSuppressionFailedAndIntentProjectionsSuppressWithoutRetry(t *testing.T) {
	for _, state := range []string{"failed", "intent"} {
		t.Run(state, func(t *testing.T) {
			_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
			record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, state)
			record, err := writeIssueOps(IssueOpsStateRoot(), record)
			if err != nil {
				t.Fatal(err)
			}
			req := stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
			if !SuppressStopNextActionForCompletedWorker(req) {
				t.Fatalf("persisted terminal projection state %q must suppress without a retry loop", state)
			}
		})
	}
}

func TestStopSuppressionClosedAcceptedRaceSuppresses(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	record, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	req := stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
	if !SuppressStopNextActionForCompletedWorker(req) {
		t.Fatalf("closed+accepted race with persisted projection must still suppress")
	}
}

func TestStopSuppressionMismatchOrAmbiguityFailsClosed(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*IssueOpsRecord)
		req    func(record IssueOpsRecord, worktree string) HookToolUseLifecycleRequest
	}{
		{name: "no handoff", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff = nil }},
		{name: "missing projection", mutate: func(r *IssueOpsRecord) { r.ExecutionHandoff.WorkerDoneProjection = nil }},
		{name: "wrong host", req: func(record IssueOpsRecord, worktree string) HookToolUseLifecycleRequest {
			return stopSuppressionRequest(record, worktree, "claude", "session-1", "worker-1")
		}},
		{name: "wrong session", req: func(record IssueOpsRecord, worktree string) HookToolUseLifecycleRequest {
			return stopSuppressionRequest(record, worktree, "codex", "wrong-session", "worker-1")
		}},
		{name: "wrong agent", req: func(record IssueOpsRecord, worktree string) HookToolUseLifecycleRequest {
			return stopSuppressionRequest(record, worktree, "codex", "session-1", "wrong-agent")
		}},
		{name: "wrong worktree", req: func(record IssueOpsRecord, worktree string) HookToolUseLifecycleRequest {
			return stopSuppressionRequest(record, worktree+"-other", "codex", "session-1", "worker-1")
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
			record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
			if tt.mutate != nil {
				tt.mutate(&record)
			}
			written, err := writeIssueOps(IssueOpsStateRoot(), record)
			if err != nil {
				t.Fatalf("persist valid mismatch fixture: %v", err)
			}
			record = written
			var req HookToolUseLifecycleRequest
			if tt.req != nil {
				req = tt.req(record, worktree)
			} else {
				req = stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
			}
			if SuppressStopNextActionForCompletedWorker(req) {
				t.Fatalf("mismatch/ambiguity %q must fail closed (no suppression)", tt.name)
			}
		})
	}
}

func TestStopSuppressionStrictAgentIdentity(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	written, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	if SuppressStopNextActionForCompletedWorker(stopSuppressionRequest(written, worktree, "codex", "session-1", "")) {
		t.Fatalf("empty request agent must not match a nonempty persisted agent")
	}

	written.ExecutionHandoff.WorkerSession.AgentID = ""
	written.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, written.ExecutionHandoff, "sent")
	written, err = writeIssueOps(IssueOpsStateRoot(), written)
	if err != nil {
		t.Fatal(err)
	}
	if SuppressStopNextActionForCompletedWorker(stopSuppressionRequest(written, worktree, "codex", "session-1", "worker-1")) {
		t.Fatalf("nonempty request agent must not match an empty persisted agent")
	}
	if !SuppressStopNextActionForCompletedWorker(stopSuppressionRequest(written, worktree, "CODEX", "session-1", "")) {
		t.Fatalf("both empty agents with a case-insensitive exact host must match")
	}
}

func TestStopSuppressionRejectsCurrentBranchMismatch(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	written, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/1-other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if SuppressStopNextActionForCompletedWorker(stopSuppressionRequest(written, worktree, "codex", "session-1", "worker-1")) {
		t.Fatalf("worker branch drift must leave legacy Stop behavior unchanged")
	}
}

func TestStopSuppressionRejectsRealDuplicateActiveRecords(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
	written, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}

	duplicate := written
	duplicate.ID = newIssueOpsID(written.Repo, "1/demo")
	duplicate.Branch = "1/demo"
	duplicate.PlanPath = ""
	duplicate.ExecutionHandoff = cloneLifecycleHandoffForTest(t, written.ExecutionHandoff)
	if err := handoff.ValidateEnvelope(duplicate); err != nil {
		t.Fatalf("duplicate fixture must be a second valid active envelope: %v", err)
	}
	duplicate, err = writeIssueOps(IssueOpsStateRoot(), duplicate)
	if err != nil {
		t.Fatalf("persist duplicate active record: %v", err)
	}
	if err := issueops.BindScopedIssueOpsSession(written.Repo, duplicate.ID, duplicate.Branch, worktree); err != nil {
		t.Fatalf("bind duplicate active record: %v", err)
	}
	if _, ok := stopSuppressionRecord(stopSuppressionRequest(written, worktree, "codex", "session-1", "worker-1")); ok {
		t.Fatalf("two active records bound to one worker root must be ambiguous")
	}
	if SuppressStopNextActionForCompletedWorker(stopSuppressionRequest(written, worktree, "codex", "session-1", "worker-1")) {
		t.Fatalf("duplicate active record ambiguity must not suppress legacy Stop behavior")
	}
}

func TestStopSuppressionRejectsMalformedEnvelopeAndPathEvidence(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*IssueOpsRecord, string)
	}{
		{name: "malformed projection attempt", mutate: func(r *IssueOpsRecord, _ string) {
			r.ExecutionHandoff.WorkerDoneProjection.Attempt++
		}},
		{name: "nonterminal projection", mutate: func(r *IssueOpsRecord, _ string) {
			r.ExecutionHandoff.WorkerDoneProjection.State = "pending"
		}},
		{name: "missing completed result", mutate: func(r *IssueOpsRecord, _ string) {
			r.ExecutionHandoff.Result = nil
		}},
		{name: "worker root mismatch", mutate: func(r *IssueOpsRecord, worktree string) {
			r.ExecutionHandoff.WorkerRoot = worktree + "-other"
		}},
		{name: "linked worktree mismatch", mutate: func(r *IssueOpsRecord, worktree string) {
			other := worktree + "-other"
			if err := os.MkdirAll(filepath.Join(other, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			r.WorktreePath = other
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
			record.ExecutionHandoff.WorkerDoneProjection = validWorkerDoneProjection(t, record.ExecutionHandoff, "sent")
			tt.mutate(&record, worktree)
			if err := handoff.ValidateEnvelope(record); err == nil {
				t.Fatalf("fixture must be malformed before raw persistence")
			}
			putRawLifecycleIssueOpsRecord(t, record)
			req := stopSuppressionRequest(record, worktree, "codex", "session-1", "worker-1")
			if _, ok := stopSuppressionRecord(req); !ok {
				t.Fatalf("malformed fixture must remain directly selectable so envelope validation is exercised")
			}
			if SuppressStopNextActionForCompletedWorker(req) {
				t.Fatalf("malformed envelope must leave legacy Stop behavior unchanged")
			}
		})
	}
}

func TestStopSuppressionLegacyUnchangedWithNoHandoff(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	req := HookToolUseLifecycleRequest{Repo: t.TempDir(), CWD: t.TempDir(), Host: "codex", SessionID: "session-1"}
	if SuppressStopNextActionForCompletedWorker(req) {
		t.Fatalf("no active supervised handoff must never suppress legacy Stop behavior")
	}
}
