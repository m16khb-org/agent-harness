package hookcli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hookadapter "agent-harness/internal/adapter/hook"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/lifecycle"
)

func seedCompletedStopWorker(t *testing.T, agentID string) (string, string) {
	t.Helper()
	repo := t.TempDir()
	branch := "16-stop-suppression"
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(repo+".worktrees", branch)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	worktreeGitDir := filepath.Join(repo, ".git", "worktrees", "stop-worker")
	if err := os.MkdirAll(worktreeGitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+worktreeGitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reportRel := ".agent-harness/research/stop-report.md"
	if err := os.MkdirAll(filepath.Join(worktree, filepath.Dir(reportRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, filepath.FromSlash(reportRel)), []byte("# stop evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	record.Phase = core.IssueOpsPhasePR
	record.RemoteArtifact = &issueopsmodel.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/example/repo/pull/1",
		Labels: []string{"issueops"}, Assignees: []string{"habin"}, VerifiedAt: "2026-07-11T02:00:00Z",
	}
	record.WorktreePath = worktree
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion:          handoff.ProtocolVersion,
		State:                    handoff.StateClosed,
		ClosedDisposition:        handoff.DispositionAccepted,
		AcceptedAt:               "2026-07-11T02:00:00Z",
		Attempt:                  1,
		OwnershipEpoch:           "epoch-stop-1",
		ContextVersion:           handoff.ContextVersion,
		ContextSHA256:            strings.Repeat("a", 64),
		ContextSourceSHA256:      strings.Repeat("d", 64),
		ContextOptions:           &issueopsmodel.IssueOpsExecutionHandoffContextOptions{},
		AttemptBaseHead:          strings.Repeat("b", 40),
		Driver:                   "orca",
		Agent:                    "codex",
		CoordinatorRoot:          repo,
		CoordinatorMailboxHandle: "term_coordinator",
		WorkerRoot:               worktree,
		DeliveryMode:             "inject",
		WorkerSession:            &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-stop-1", AgentID: agentID},
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/" + branch,
			WorktreeID: "wt-1", WorktreeInstanceID: "instance-1", WorktreePath: worktree,
			WorkerPTYID: "pty-1", WorkerTerminalHandle: "term_worker", WorkerMailboxHandle: "term_worker",
			TaskID: "task-1", DispatchID: "dispatch-1",
		},
		Result: &issueopsmodel.IssueOpsExecutionHandoffResult{
			Outcome: handoff.OutcomeCompleted, FinalHead: strings.Repeat("b", 40),
			ChangedFiles: []string{reportRel}, TuringReportPath: reportRel,
			Verification: []string{"focused tests passed"}, CleanupReceipts: []string{"worker stopped"},
			TaskID: "task-1", DispatchID: "dispatch-1",
		},
	}
	h := record.ExecutionHandoff
	projection := &issueopsmodel.IssueOpsExecutionHandoffWorkerDoneProjection{
		State: "sent", Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch,
		DiagnosticCode: "sent", StartedAt: "2026-07-11T02:00:00Z", CompletedAt: "2026-07-11T02:00:01Z",
		Invoked: true, FromHandle: h.Orca.WorkerMailboxHandle, ToHandle: h.CoordinatorMailboxHandle,
		Subject: "completed", Body: "completed", TaskID: h.Result.TaskID, DispatchID: h.Result.DispatchID,
		FinalHead: h.Result.FinalHead, ChangedFiles: h.Result.ChangedFiles,
		ReportPath:   filepath.Clean(filepath.Join(worktree, filepath.FromSlash(reportRel))),
		HostIdentity: h.WorkerSession.Host + "/" + h.WorkerSession.SessionID, MessageID: "msg-1", MessageSequence: 1,
	}
	if h.WorkerSession.AgentID != "" {
		projection.HostIdentity += "/" + h.WorkerSession.AgentID
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
	}{projection.FromHandle, projection.ToHandle, projection.Subject, projection.Body, projection.TaskID, projection.DispatchID, projection.ChangedFiles, projection.ReportPath})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	projection.PayloadSHA256 = hex.EncodeToString(sum[:])
	h.WorkerDoneProjection = projection
	if err := handoff.ValidateEnvelope(record); err != nil {
		t.Fatalf("completed Stop fixture must be a valid envelope: %v", err)
	}
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	if err := issueops.BindIssueOpsSession(repo, record.ID, record.Branch, worktree); err != nil {
		t.Fatal(err)
	}
	record, err = core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), record.ID, string(core.IssueOpsPhaseDone))
	if err != nil {
		t.Fatal(err)
	}
	if binding, err := issueops.ReadIssueOpsSession(repo); err != nil || binding.CycleID != "" {
		t.Fatalf("done transition must remove the normal session binding: binding=%+v err=%v", binding, err)
	}
	return repo, worktree
}

func stopHookInput(t *testing.T, repo, cwd, host, session, agent, message string) string {
	t.Helper()
	input := map[string]any{
		"repo": repo, "cwd": cwd, "session_id": session, "agent_id": agent,
		"last_assistant_message": message,
	}
	if host != "" {
		input["host"] = host
	}
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hostlessCodexStopInput(t *testing.T, cwd, session, message string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"session_id": session, "turn_id": "turn-stop-1", "transcript_path": nil,
		"cwd": cwd, "hook_event_name": "Stop", "model": "gpt-5",
		"permission_mode": "default", "stop_hook_active": false,
		"last_assistant_message": message,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunHookStopDefaultsHostlessCodexDoneUnboundWorker(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	hookMetricDecision = ""
	t.Cleanup(func() { hookMetricDecision = "" })
	repo, worktree := seedCompletedStopWorker(t, "")
	t.Setenv("PWD", worktree)
	choices := "선택지:\n1. 검증을 계속합니다. (추천)\n2. 일부만 검증합니다.\n3. 보류합니다." + strings.ReplaceAll(hookChoiceQualityEvidenceEscaped, `\n`, "\n")

	out := runHookStopRaw(t, hostlessCodexStopInput(t, worktree, "session-stop-1", choices), "--enforce-numbered-next-actions", "--enforce-engelbart-canvas-sections", "--relay-next-action-judgement")
	if out != "{}\n" {
		t.Fatalf("hostless Codex Stop must suppress exact completed worker re-entry, got %q", out)
	}
	if _, found := lifecycle.ReadStopNextActionRelay(repo); found {
		t.Fatalf("hostless completed Codex worker must not mutate numbered-next-action relay state")
	}
}

func TestRunHookStopHostlessNoOrcaPreservesLegacyBytesWithoutCreatingState(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	hookMetricDecision = ""
	t.Cleanup(func() { hookMetricDecision = "" })
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".git", "HEAD"), []byte("ref: refs/heads/16-no-orca\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	message := "작업을 완료했습니다."
	decision := core.BuildNumberedNextActionsDecision(message, true, "stop")
	expected := indentedJSONLine(t, hookadapter.Resolve("").FormatStopBlock(decision.Reason))

	got := runHookStopRaw(t, hostlessCodexStopInput(t, cwd, "session-no-orca", message), "--enforce-numbered-next-actions", "--enforce-engelbart-canvas-sections", "--relay-next-action-judgement")
	if got != expected {
		t.Fatalf("hostless no-Orca Stop changed legacy bytes\nwant: %q\n got: %q", expected, got)
	}
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("hostless no-Orca Stop created state in an initially empty root: %v", names)
	}
}

func runHookStopRaw(t *testing.T, stdinJSON string, args ...string) string {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	go func() {
		_, _ = io.WriteString(w, stdinJSON)
		_ = w.Close()
	}()
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	return captureStdoutForTest(t, func() {
		if err := runHookStop(args); err != nil {
			t.Fatalf("runHookStop: %v", err)
		}
	})
}

func indentedJSONLine(t *testing.T, value any) string {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

func TestRunHookStopSuppressesOnlyCompletedWorkerNextActionBranches(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	hookMetricDecision = ""
	t.Cleanup(func() { hookMetricDecision = "" })
	repo, worktree := seedCompletedStopWorker(t, "worker-stop-1")
	choices := "선택지:\n1. 검증을 계속합니다. (추천)\n2. 일부만 검증합니다.\n3. 보류합니다." + strings.ReplaceAll(hookChoiceQualityEvidenceEscaped, `\n`, "\n")

	for _, tt := range []struct {
		name    string
		message string
		args    []string
	}{
		{name: "numbered relay", message: choices, args: []string{"--host", "CoDeX", "--relay-next-action-judgement"}},
		{name: "missing choices", message: "작업을 완료했습니다.", args: []string{"--host", "CoDeX", "--enforce-numbered-next-actions"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := runHookStopRaw(t, stopHookInput(t, repo, worktree, "codex", "session-stop-1", "worker-stop-1", tt.message), tt.args...)
			if out != "{}\n" {
				t.Fatalf("exact completed worker must receive the legacy no-op bytes, got %q", out)
			}
			if _, found := lifecycle.ReadStopNextActionRelay(repo); found {
				t.Fatalf("completed worker must not mutate numbered-next-action relay state")
			}
			if hookMetricDecision != "" {
				t.Fatalf("suppressed next-action re-entry must not fabricate a block metric: %q", hookMetricDecision)
			}
		})
	}
}

func TestRunHookStopCompletedWorkerPreservesRawJSONAndEngelbart(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	hookMetricDecision = ""
	t.Cleanup(func() { hookMetricDecision = "" })
	repo, worktree := seedCompletedStopWorker(t, "worker-stop-1")
	message := "Engelbart 회의록 Canvas를 생성했습니다.\n" + incompleteEngelbartCanvasContent

	raw := runHookCapture(t, stopHookInput(t, repo, worktree, "codex", "session-stop-1", "worker-stop-1", "작업을 완료했습니다."), func() error {
		return runHookStop([]string{"--host", "codex", "--enforce-numbered-next-actions", "--json"})
	})
	if _, ok := raw["lifecycle"]; !ok {
		t.Fatalf("completed suppression must preserve raw lifecycle JSON: %+v", raw)
	}
	next, _ := raw["numbered_next_actions"].(map[string]any)
	if next["decision"] != "block" {
		t.Fatalf("completed suppression must not rewrite raw numbered-next-action analysis: %+v", raw)
	}

	out := runHookCapture(t, stopHookInput(t, repo, worktree, "codex", "session-stop-1", "worker-stop-1", message), func() error {
		return runHookStop([]string{"--host", "codex", "--enforce-numbered-next-actions", "--enforce-engelbart-canvas-sections", "--relay-next-action-judgement"})
	})
	if out["decision"] != "block" || !strings.Contains(out["reason"].(string), "Engelbart") {
		t.Fatalf("Engelbart must still block an exact completed worker: %+v", out)
	}
	if hookMetricDecision != "block" {
		t.Fatalf("Engelbart block metric must be preserved, got %q", hookMetricDecision)
	}
}

func TestRunHookStopHostConflictPreservesLegacyBytesAndSideEffects(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	hookMetricDecision = ""
	t.Cleanup(func() { hookMetricDecision = "" })
	repo, worktree := seedCompletedStopWorker(t, "worker-stop-1")
	choices := "선택지:\n1. 검증을 계속합니다. (추천)\n2. 일부만 검증합니다.\n3. 보류합니다." + strings.ReplaceAll(hookChoiceQualityEvidenceEscaped, `\n`, "\n")
	trigger := core.BuildNextActionJudgementTrigger(choices)
	reason := core.BuildNextActionJudgementRelayReason(trigger)
	if facts := core.StopOrchestrationRelayFacts(repo); facts != "" {
		reason += " 관찰된 orchestration 상태: " + facts + "."
	}
	expected := indentedJSONLine(t, hookadapter.Resolve("claude").FormatStopBlock(reason))

	got := runHookStopRaw(t, stopHookInput(t, repo, worktree, "codex", "session-stop-1", "worker-stop-1", choices), "--host", "claude", "--relay-next-action-judgement")
	if got != expected {
		t.Fatalf("host conflict must fall back to exact legacy bytes\nwant: %q\n got: %q", expected, got)
	}
	relay, found := lifecycle.ReadStopNextActionRelay(repo)
	if !found || relay.Fingerprint == "" || relay.RecommendedIndex != trigger.RecommendedIndex || relay.RecommendedText != trigger.RecommendedText || len(relay.Candidates) != 3 {
		t.Fatalf("host conflict must preserve legacy relay mutation: %+v found=%v", relay, found)
	}
	if hookMetricDecision != "" {
		t.Fatalf("legacy relay path metric marker changed: %q", hookMetricDecision)
	}

	core.ClearStopNextActionRelay(repo)
	hookMetricDecision = ""
	message := "작업을 완료했습니다."
	decision := core.BuildNumberedNextActionsDecision(message, true, "stop")
	expected = indentedJSONLine(t, hookadapter.Resolve("claude").FormatStopBlock(decision.Reason))
	got = runHookStopRaw(t, stopHookInput(t, repo, worktree, "codex", "session-stop-1", "worker-stop-1", message), "--host", "claude", "--enforce-numbered-next-actions")
	if got != expected {
		t.Fatalf("host conflict missing-choice path must preserve exact legacy bytes\nwant: %q\n got: %q", expected, got)
	}
	if hookMetricDecision != "block" {
		t.Fatalf("host conflict must preserve the legacy missing-choice block metric, got %q", hookMetricDecision)
	}
}
