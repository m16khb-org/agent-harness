package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestOwnershipAcknowledgementGrantsOwnerAndRevokesSource(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	target := filepath.Join(worker, "internal", "owner.go")

	owner := handoffEditRequest(record, worker, "claude", "owner-session", target)
	owner.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("active transferred owner must be allowed to edit its worker root: %#v", got)
	}

	sourceTargetingWorker := handoffEditRequest(record, repo, "claude", "coordinator-session", target)
	sourceTargetingWorker.AgentID = "coordinator-agent"
	if got := BuildLifecyclePreToolUseDecision(sourceTargetingWorker); got.Decision != "block" || !strings.Contains(got.Reason, "owner") {
		t.Fatalf("source coordinator must lose worker mutation authority after transfer: %#v", got)
	}

	ordinarySource := handoffEditRequest(record, repo, "claude", "coordinator-session", filepath.Join(repo, "internal", "ordinary.go"))
	ordinarySource.AgentID = "coordinator-agent"
	if got := BuildLifecyclePreToolUseDecision(ordinarySource); got.Decision != "allow" {
		t.Fatalf("ordinary source work must remain outside ownership fence: %#v", got)
	}
}

func TestOwnershipFenceNeverCapturesOrdinarySourceMutation(t *testing.T) {
	states := []string{
		handoff.StateOwnershipDispatching,
		handoff.StateOwnershipDispatched,
		handoff.StateOwnerOrienting,
		handoff.StateOwnerActive,
		handoff.StateCleanupPendingHumanDecision,
		handoff.StateCleanupExecuting,
		handoff.StateClosed,
		handoff.StateRecoveryRequired,
	}
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			repo, record, _ := ownershipLifecycleRecord(t, state)
			req := handoffEditRequest(record, repo, "claude", "coordinator-session", filepath.Join(repo, "internal", "ordinary.go"))
			req.AgentID = "coordinator-agent"
			if _, selected, reason := selectSupervisedHandoffRecord(req); selected || reason != "" {
				t.Fatalf("ordinary source work must not select v2 state %s: selected=%v reason=%q", state, selected, reason)
			}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("ordinary source work must remain allowed in v2 state %s: %#v", state, got)
			}
		})
	}
}

func TestOwnershipOwnerOnlyPublishesAndCreatesRemotePR(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	record.Phase = IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: strings.Repeat("a", 64),
		Branch: record.Branch, Base: record.BranchPrepare.BaseBranch, RemoteRef: "refs/heads/" + record.Branch, FinalHead: strings.Repeat("f", 40), VerifiedAt: "2026-07-20T00:00:00Z",
	}
	record, _ = writeIssueOps(IssueOpsStateRoot(), record)

	owner := handoffEditRequest(record, worker, "claude", "owner-session", "")
	owner.AgentID, owner.Tool = "owner-agent", "mcp__agent_harness__issueops_handoff"
	owner.ToolInput = map[string]any{"action": "publish", "id": record.ID, "host": "claude", "session_id": "owner-session", "agent_id": "owner-agent", "cwd": worker, "confirm": true}
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("exact owner publish blocked: %#v", got)
	}
	owner.Tool = "mcp__agent_harness__issueops_remote_create_pr"
	owner.ToolInput = map[string]any{"id": record.ID, "title": "draft", "body": "rendered", "provider": "github", "head": record.Branch, "base": record.BranchPrepare.BaseBranch, "labels": []any{"bug"}, "assignees": []any{"octocat"}, "confirm": true, "host": "claude", "session_id": "owner-session", "agent_id": "owner-agent", "cwd": worker}
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("exact owner remote create blocked: %#v", got)
	}

	source := handoffEditRequest(record, repo, "claude", "coordinator-session", "")
	source.AgentID, source.Tool, source.ToolInput = "coordinator-agent", "mcp__agent_harness__issueops_handoff", owner.ToolInput
	if got := BuildLifecyclePreToolUseDecision(source); got.Decision != "block" {
		t.Fatalf("source session regained owner publish authority: %#v", got)
	}
}

func TestOwnershipOwnerOnlyCompletesIntoHumanCleanupBoundary(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	owner := handoffEditRequest(record, worker, "claude", "owner-session", "")
	owner.AgentID, owner.Tool = "owner-agent", "mcp__agent_harness__issueops_handoff"
	owner.ToolInput = map[string]any{
		"action": "complete", "id": record.ID, "attempt": 1, "ownership_epoch": record.ExecutionHandoff.OwnershipEpoch,
		"context_sha256": record.ExecutionHandoff.ContextSHA256, "host": "claude", "session_id": "owner-session", "agent_id": "owner-agent", "cwd": worker,
		"final_head": strings.Repeat("f", 40), "turing_report_path": "plans/owner.md", "verification": []any{"go test ./..."},
	}
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("exact owner completion blocked: %#v", got)
	}

	source := handoffEditRequest(record, repo, "claude", "coordinator-session", "")
	source.AgentID, source.Tool, source.ToolInput = "coordinator-agent", "mcp__agent_harness__issueops_handoff", owner.ToolInput
	if got := BuildLifecyclePreToolUseDecision(source); got.Decision != "block" {
		t.Fatalf("source session regained owner completion authority: %#v", got)
	}

	cli := handoffEditRequest(record, worker, "claude", "owner-session", "")
	cli.AgentID, cli.Tool = "owner-agent", "Bash"
	cli.Command = "agent-harness issueops handoff complete --id " + record.ID + " --attempt 1 --ownership-epoch " + record.ExecutionHandoff.OwnershipEpoch + " --context-sha256 " + record.ExecutionHandoff.ContextSHA256 + " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --final-head " + strings.Repeat("f", 40) + " --turing-report plans/owner.md --verification 'go test ./...'"
	if got := BuildLifecyclePreToolUseDecision(cli); got.Decision != "allow" {
		t.Fatalf("exact owner complete command blocked: %#v", got)
	}
}

func TestOwnershipCleanupAllowsFreshSourceButRejectsCompletedOwner(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateCleanupPendingHumanDecision)
	fresh := handoffEditRequest(record, repo, "codex", "fresh-source", "")
	fresh.AgentID, fresh.Tool = "source-agent", "mcp__agent_harness__issueops_handoff"
	fresh.ToolInput = map[string]any{"action": "cleanup-preview", "id": record.ID, "host": "codex", "session_id": "fresh-source", "agent_id": "source-agent", "source_cwd": repo}
	if got := BuildLifecyclePreToolUseDecision(fresh); got.Decision != "allow" {
		t.Fatalf("fresh exact source cleanup preview blocked: %#v", got)
	}
	fresh.ToolInput = map[string]any{"action": "cleanup-approve", "id": record.ID, "host": "codex", "session_id": "fresh-source", "agent_id": "source-agent", "source_cwd": repo, "inventory_fingerprint": strings.Repeat("a", 64), "disposition": "close-owner", "reason": "human selected retained workspace", "confirm": true}
	if got := BuildLifecyclePreToolUseDecision(fresh); got.Decision != "allow" {
		t.Fatalf("fresh exact source cleanup approval blocked: %#v", got)
	}

	owner := handoffEditRequest(record, worker, "claude", "owner-session", "")
	owner.AgentID, owner.Tool = "owner-agent", "mcp__agent_harness__issueops_handoff"
	owner.ToolInput = map[string]any{"action": "cleanup-preview", "id": record.ID, "host": "claude", "session_id": "owner-session", "agent_id": "owner-agent", "source_cwd": repo}
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "block" {
		t.Fatalf("completed owner became cleanup candidate: %#v", got)
	}
}

func TestOwnershipCleanupPendingStopsAtHumanSourceGate(t *testing.T) {
	repo, record, _ := ownershipLifecycleRecord(t, handoff.StateCleanupPendingHumanDecision)
	record.Phase = IssueOpsPhaseDone
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if id, ok := OwnershipCleanupHumanGate(HookToolUseLifecycleRequest{Repo: repo, CWD: repo, Host: "codex", SessionID: "fresh-source", AgentID: "source-agent"}); !ok || id != record.ID {
		t.Fatalf("fresh source must see human cleanup gate: id=%q ok=%v", id, ok)
	}
	if _, ok := OwnershipCleanupHumanGate(HookToolUseLifecycleRequest{Repo: repo, CWD: repo, Host: "claude", SessionID: "owner-session", AgentID: "owner-agent"}); ok {
		t.Fatal("completed owner must not receive cleanup authority through Stop")
	}
}

func TestOwnershipRoleAuthorityMatrix(t *testing.T) {
	_, orienting, worker := ownershipLifecycleRecord(t, handoff.StateOwnerOrienting)
	target := filepath.Join(worker, "internal", "owner.go")

	ownerEdit := handoffEditRequest(orienting, worker, "claude", "owner-session", target)
	ownerEdit.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(ownerEdit); got.Decision != "block" || !strings.Contains(got.Reason, "acknowledge") {
		t.Fatalf("orienting owner must acknowledge before mutation: %#v", got)
	}

	ack := ownershipAcknowledgementCommand(orienting, worker)
	ackReq := handoffEditRequest(orienting, worker, "claude", "owner-session", "")
	ackReq.AgentID, ackReq.Tool, ackReq.Command = "owner-agent", "Bash", ack
	if got := BuildLifecyclePreToolUseDecision(ackReq); got.Decision != "allow" {
		t.Fatalf("exact owner acknowledgement must be allowed while orienting: %#v", got)
	}
	ackMCP := handoffEditRequest(orienting, worker, "claude", "owner-session", "")
	ackMCP.AgentID, ackMCP.Tool = "owner-agent", "mcp__agent_harness__issueops_handoff"
	ackMCP.ToolInput = map[string]any{
		"action": "acknowledge-context", "id": orienting.ID, "attempt": 1, "ownership_epoch": orienting.ExecutionHandoff.OwnershipEpoch,
		"context_sha256": orienting.ExecutionHandoff.ContextSHA256, "host": "claude", "session_id": "owner-session", "agent_id": "owner-agent", "cwd": worker,
		"issue_url": orienting.IssueURL, "plan_sha256": strings.Repeat("d", 64), "understanding": "understood", "scope_confirmation": "scoped",
	}
	if got := BuildLifecyclePreToolUseDecision(ackMCP); got.Decision != "allow" {
		t.Fatalf("exact MCP acknowledgement must be allowed while orienting: %#v", got)
	}

	heartbeat := handoffEditRequest(orienting, worker, "claude", "owner-session", "")
	heartbeat.AgentID, heartbeat.Tool = "owner-agent", "Bash"
	heartbeat.Command = ownershipHeartbeatCommand(orienting)
	if got := BuildLifecyclePreToolUseDecision(heartbeat); got.Decision != "allow" {
		t.Fatalf("orienting owner heartbeat must be allowed: %#v", got)
	}

	_, active, activeWorker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	wrongSession := handoffEditRequest(active, activeWorker, "claude", "other-session", filepath.Join(activeWorker, "internal", "owner.go"))
	wrongSession.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(wrongSession); got.Decision != "block" || !strings.Contains(got.Reason, "owner session") {
		t.Fatalf("different native session must not gain owner mutation authority: %#v", got)
	}

	wrongRoot := handoffEditRequest(active, filepath.Dir(activeWorker), "claude", "owner-session", filepath.Join(activeWorker, "internal", "owner.go"))
	wrongRoot.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(wrongRoot); got.Decision != "block" || !strings.Contains(got.Reason, "canonical worker") {
		t.Fatalf("active owner must mutate from the canonical worker root: %#v", got)
	}
}

func TestOwnershipOrientingKeepsCoordinatorGuidanceAndWorkerEscalationOpen(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerOrienting)
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-owner"
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-owner"
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}

	coordinator := handoffEditRequest(record, repo, "codex", "coordinator", "")
	coordinator.AgentID, coordinator.Tool = "worker-1", "exec_command"
	coordinator.Command = "orca terminal send --terminal term-owner --text '# agent-harness guidance: run the exact acknowledgement command once' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(coordinator); got.Decision != "allow" {
		t.Fatalf("sealed source coordinator guidance must remain available while owner is orienting: %#v", got)
	}

	owner := handoffEditRequest(record, worker, "claude", "owner-session", "")
	owner.AgentID, owner.Tool = "owner-agent", "exec_command"
	owner.Command = "orca orchestration send --to term_coordinator --from term-owner --type escalation --subject blocked --body 'acknowledgement guidance required' --task-id task-1 --dispatch-id dispatch-1 --json"
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("orienting owner must be able to escalate through its sealed mailbox: %#v", got)
	}

	owner.Command = "orca orchestration send --to term_coordinator --from term-other --type escalation --subject blocked --body 'acknowledgement guidance required' --task-id task-1 --dispatch-id dispatch-1 --json"
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "block" {
		t.Fatalf("orienting owner escalation must reject a mismatched sender mailbox: %#v", got)
	}
}

func TestOwnershipFenceStillProtectsWorkerRootAndCycleControl(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	sourcePhase := handoffEditRequest(record, repo, "claude", "coordinator-session", "")
	sourcePhase.AgentID, sourcePhase.Tool = "coordinator-agent", "Bash"
	sourcePhase.Command = "agent-harness issueops phase --id " + record.ID + " --to implement"
	if got := BuildLifecyclePreToolUseDecision(sourcePhase); got.Decision != "block" {
		t.Fatalf("source coordinator must not regain exact-cycle control after ownership transfer: %#v", got)
	}

	unclaimed := handoffEditRequest(record, worker, "claude", "other-session", filepath.Join(worker, "internal", "owner.go"))
	unclaimed.AgentID = "other-agent"
	if got := BuildLifecyclePreToolUseDecision(unclaimed); got.Decision != "block" || !strings.Contains(got.Reason, "owner session") {
		t.Fatalf("unclaimed worker session must not mutate canonical worker root: %#v", got)
	}
}

func TestHandoffGuardNarrowDiagnostics(t *testing.T) {
	repo, record, worker := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	planRel := filepath.Join("plans", "io-plan.md")
	if err := os.MkdirAll(filepath.Join(worker, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worker, planRel), []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record.PlanPath = planRel
	updated, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	gitMismatch := handoffEditRequest(updated, repo, "codex", "coordinator", "")
	gitMismatch.Tool = "Bash"
	gitMismatch.Command = "git -C " + worker + " add -- " + planRel
	if got := BuildLifecyclePreToolUseDecision(gitMismatch); got.Decision != "block" || !strings.Contains(got.Reason, filepath.Join(worker, planRel)) {
		t.Fatalf("v1 relative plan Git path must name its exact absolute rewrite: %#v", got)
	}

	resumeBind := handoffEditRequest(updated, repo, "codex", "coordinator", "")
	resumeBind.Tool = "mcp__agent_harness__issueops_resume"
	resumeBind.ToolInput = map[string]any{"id": updated.ID, "bind": true}
	if got := BuildLifecyclePreToolUseDecision(resumeBind); got.Decision != "block" || !strings.Contains(got.Reason, "omit bind or set bind=false") {
		t.Fatalf("resume bind diagnostic must name the safe payload correction: %#v", got)
	}
}

func TestOwnershipSessionGuidanceRendersClaimAndOrientationBoundary(t *testing.T) {
	_, _, worker := ownershipLifecycleRecord(t, handoff.StateOwnershipDispatched)
	guidance := BuildIssueOpsHandoffSessionGuidance(worker, "claude", "owner-session", "owner-agent")
	if !strings.Contains(guidance, "handoff claim") || !strings.Contains(guidance, "ownership_dispatched") {
		t.Fatalf("ownership dispatched guidance must render the exact claim path: %s", guidance)
	}

	_, _, worker = ownershipLifecycleRecord(t, handoff.StateOwnerOrienting)
	guidance = BuildIssueOpsHandoffSessionGuidance(worker, "claude", "owner-session", "owner-agent")
	if !strings.Contains(guidance, "Acknowledge") || !strings.Contains(guidance, "read-only") {
		t.Fatalf("orienting owner guidance must name acknowledgement boundary: %s", guidance)
	}
}

func ownershipLifecycleRecord(t *testing.T, state string) (string, IssueOpsRecord, string) {
	t.Helper()
	repo, record, worker := lifecycleHandoffRecord(t, handoff.StateClaimed)
	h := record.ExecutionHandoff
	h.ProtocolVersion = handoff.OwnershipTransferProtocolVersion
	h.State = state
	h.WorkspaceEpoch = "workspace-epoch-1"
	h.WorkspaceSHA256 = strings.Repeat("c", 64)
	h.WorkerSession = nil
	h.Result = nil
	h.AcceptedAt = ""
	h.OwnerSession = nil
	h.Orientation = nil
	h.Completion = nil
	workspaceOrca := *h.Orca
	workspaceOrca.WorkerPTYID = ""
	workspaceOrca.WorkerTerminalHandle = ""
	workspaceOrca.WorkerMailboxHandle = ""
	workspaceOrca.TaskID = ""
	workspaceOrca.DispatchID = ""
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: h.WorkspaceEpoch, Driver: "orca", Agent: "claude",
		CoordinatorRoot: repo, WorkerRoot: worker,
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"},
		BaseHead:           h.AttemptBaseHead, Orca: &workspaceOrca,
	}
	if state == handoff.StateOwnerOrienting || state == handoff.StateOwnerActive || state == handoff.StateCleanupPendingHumanDecision || state == handoff.StateCleanupExecuting || state == handoff.StateClosed || state == handoff.StateRecoveryRequired {
		h.OwnerSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "owner-session", AgentID: "owner-agent"}
	}
	if state == handoff.StateOwnerActive || state == handoff.StateCleanupPendingHumanDecision || state == handoff.StateCleanupExecuting || state == handoff.StateClosed || state == handoff.StateRecoveryRequired {
		h.Orientation = &issueopsmodel.IssueOpsOwnershipOrientation{IssueURL: record.IssueURL, PlanSHA256: strings.Repeat("d", 64), Understanding: "understood", ScopeConfirmation: "worker root only", RecordedAt: "2026-07-20T00:00:00Z"}
	}
	if state == handoff.StateCleanupPendingHumanDecision || state == handoff.StateCleanupExecuting || state == handoff.StateClosed || state == handoff.StateRecoveryRequired {
		h.Completion = &issueopsmodel.IssueOpsOwnershipCompletion{FinalHead: strings.Repeat("e", 40), CompletedAt: "2026-07-20T00:00:01Z"}
	}
	if state == handoff.StateCleanupExecuting {
		h.Cleanup = &issueopsmodel.IssueOpsExecutionHandoffCleanup{Disposition: "close-owner", Reason: "human directed retained workspace", ApprovedAt: "2026-07-20T00:00:02Z", ApprovedBySession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"}, InventoryFingerprint: strings.Repeat("f", 64)}
	}
	updated, err := writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, updated, worker
}

func ownershipAcknowledgementCommand(record IssueOpsRecord, worker string) string {
	h := record.ExecutionHandoff
	return "agent-harness issueops handoff acknowledge-context --id " + record.ID +
		" --attempt 1 --ownership-epoch " + h.OwnershipEpoch + " --context-sha256 " + h.ContextSHA256 +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker +
		" --issue-url " + record.IssueURL + " --plan-sha256 " + strings.Repeat("d", 64) +
		" --understanding understood --scope-confirmation scoped"
}

func ownershipHeartbeatCommand(record IssueOpsRecord) string {
	h := record.ExecutionHandoff
	return "agent-harness issueops heartbeat --id " + record.ID +
		" --attempt 1 --ownership-epoch " + h.OwnershipEpoch + " --context-sha256 " + h.ContextSHA256 +
		" --host claude --session-id owner-session --agent-id owner-agent"
}
