package lifecycle

import (
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

func TestOwnershipCoordinatorCanOnlyResumeSealedOwner(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	request := handoffEditRequest(record, repo, "claude", "coordinator-session", "")
	request.AgentID = "coordinator-agent"
	request.Tool = "Bash"
	request.Command = "orca terminal send --terminal term-1 --text '계속 진행' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("sealed coordinator resume blocked: %#v", got)
	}
	enterOnly := request
	enterOnly.Command = "orca terminal send --terminal term-1 --enter --json"
	if got := BuildLifecyclePreToolUseDecision(enterOnly); got.Decision != "allow" {
		t.Fatalf("sealed coordinator enter blocked: %#v", got)
	}

	otherTerminal := request
	otherTerminal.Command = "orca terminal send --terminal term-other --text '계속 진행' --enter --json"
	if allowedSourceOwnerContinue(otherTerminal, record) {
		t.Fatal("coordinator resume predicate accepted a different terminal")
	}

	for name, mutate := range map[string]func(*HookToolUseLifecycleRequest){
		"arbitrary prompt": func(req *HookToolUseLifecycleRequest) {
			req.Command = "orca terminal send --terminal term-1 --text 'change scope' --enter --json"
		},
		"worker caller": func(req *HookToolUseLifecycleRequest) {
			req.Repo, req.CWD = worker, worker
			req.SessionID, req.AgentID = "owner-session", "owner-agent"
		},
		"different source session": func(req *HookToolUseLifecycleRequest) {
			req.SessionID = "other-source"
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := request
			mutate(&candidate)
			if got := BuildLifecyclePreToolUseDecision(candidate); got.Decision != "block" {
				t.Fatalf("unsafe owner steering allowed: %#v", got)
			}
		})
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
				t.Fatalf("ordinary source work must not select ownership state %s: selected=%v reason=%q", state, selected, reason)
			}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("ordinary source work must remain allowed in ownership state %s: %#v", state, got)
			}
		})
	}
}

func TestOwnershipFenceNeverCapturesNewSourceCycle(t *testing.T) {
	repo, record, _ := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	second := record
	second.ID = newIssueOpsID(repo, "2589-other-active-cycle")
	second.Branch = "2589-other-active-cycle"
	second.ExecutionHandoff = cloneOwnershipHandoffForTest(record.ExecutionHandoff)
	second.ExecutionHandoff.OwnershipEpoch = "ownership-epoch-2"
	second.ExecutionHandoff.WorkerRoot = filepath.Join(filepath.Dir(record.ExecutionHandoff.WorkerRoot), second.Branch)
	second.ExecutionWorkspace = nil
	workspace := *record.ExecutionWorkspace
	workspace.WorkerRoot = second.ExecutionHandoff.WorkerRoot
	second.ExecutionWorkspace = &workspace
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}

	requests := []HookToolUseLifecycleRequest{
		{
			Repo: repo, CWD: repo, Host: "codex", SessionID: "source-session", AgentID: "source-agent",
			Tool: "mcp__agent_harness__issueops_start", ToolInput: map[string]any{"repo": repo, "branch": "2598-add-firebase-anonymous-guest-module"},
			EnforceWorktree: true, SourceCheckout: repo,
		},
		{
			Repo: repo, CWD: repo, Host: "codex", SessionID: "source-session", AgentID: "source-agent",
			Tool: "Bash", Command: "agent-harness issueops start --repo " + shellQuote(repo) + " --branch 2598-add-firebase-anonymous-guest-module --json",
			EnforceWorktree: true, SourceCheckout: repo,
		},
	}
	for _, req := range requests {
		if _, selected, reason := selectSupervisedHandoffRecord(req); selected || reason != "" {
			t.Fatalf("new source cycle must not inherit an existing ownership fence: tool=%s selected=%v reason=%q", req.Tool, selected, reason)
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("new source cycle must remain independent from active owner sessions: tool=%s result=%#v", req.Tool, got)
		}
	}
}

func TestOwnershipOwnerExactIDRoutesIssueOpsGatesAcrossParallelCycles(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	second := record
	second.ID = newIssueOpsID(repo, "2589-other-active-cycle")
	second.Branch = "2589-other-active-cycle"
	second.ExecutionHandoff = cloneOwnershipHandoffForTest(record.ExecutionHandoff)
	second.ExecutionHandoff.OwnershipEpoch = "ownership-epoch-2"
	second.ExecutionHandoff.WorkerRoot = filepath.Join(filepath.Dir(worker), second.Branch)
	workspace := *record.ExecutionWorkspace
	workspace.WorkerRoot = second.ExecutionHandoff.WorkerRoot
	second.ExecutionWorkspace = &workspace
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}

	requests := []HookToolUseLifecycleRequest{
		{
			Repo: worker, CWD: worker, Host: "claude", SessionID: "owner-session", AgentID: "owner-agent",
			Tool: "mcp__agent_harness__issueops_record_intent", ToolInput: map[string]any{
				"id": record.ID, "raw_request": "implement issue", "interpreted_intent": "implement only this issue", "success_criteria": []any{"verified implementation"},
			},
			EnforceWorktree: true, SourceCheckout: repo,
		},
		{
			Repo: worker, CWD: worker, Host: "claude", SessionID: "owner-session", AgentID: "owner-agent",
			Tool: "mcp__agent_harness__issueops_status", ToolInput: map[string]any{"id": record.ID},
			EnforceWorktree: true, SourceCheckout: repo,
		},
	}
	for _, req := range requests {
		selected, ok, reason := selectSupervisedHandoffRecord(req)
		if !ok || reason != "" || selected.ID != record.ID {
			t.Fatalf("exact owner lifecycle id must select only its own cycle: tool=%s selected=%s ok=%v reason=%q", req.Tool, selected.ID, ok, reason)
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact owner IssueOps gate/observation blocked by another cycle: tool=%s result=%#v", req.Tool, got)
		}
	}
}

func TestParallelOwnershipCyclesDoNotCaptureExactPrepOnlyWorkerCycle(t *testing.T) {
	repo, record, _ := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	second := record
	second.ID = newIssueOpsID(repo, "2589-other-active-cycle")
	second.Branch = "2589-other-active-cycle"
	second.ExecutionHandoff = cloneOwnershipHandoffForTest(record.ExecutionHandoff)
	second.ExecutionHandoff.OwnershipEpoch = "ownership-epoch-2"
	second.ExecutionHandoff.WorkerRoot = filepath.Join(filepath.Dir(record.ExecutionHandoff.WorkerRoot), second.Branch)
	workspace := *record.ExecutionWorkspace
	workspace.WorkerRoot = second.ExecutionHandoff.WorkerRoot
	second.ExecutionWorkspace = &workspace
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}
	prep := linkIssueOpsWorktreeForGuardTest(t, repo, "2598-prep-only-cycle")

	requests := []HookToolUseLifecycleRequest{
		{
			Repo: prep.path, CWD: prep.path, Host: "codex", SessionID: "prep-owner", AgentID: "prep-agent",
			Tool: "mcp__agent_harness__issueops_record_intent", ToolInput: map[string]any{
				"id": prep.id, "raw_request": "implement issue", "interpreted_intent": "implement only this issue", "success_criteria": []any{"verified implementation"},
			},
			EnforceWorktree: true, SourceCheckout: repo,
		},
		{
			Repo: prep.path, CWD: prep.path, Host: "codex", SessionID: "prep-owner", AgentID: "prep-agent",
			Tool: "mcp__agent_harness__issueops_status", ToolInput: map[string]any{"id": prep.id},
			EnforceWorktree: true, SourceCheckout: repo,
		},
	}
	for _, req := range requests {
		if selected, ok, reason := selectSupervisedHandoffRecord(req); ok || reason != "" || selected.ID != "" {
			t.Fatalf("prep-only exact cycle must stay outside unrelated ownership fences: tool=%s selected=%s ok=%v reason=%q", req.Tool, selected.ID, ok, reason)
		}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("prep-only worker IssueOps call was captured by another cycle: tool=%s result=%#v", req.Tool, got)
		}
	}
	foreign := requests[0]
	foreign.CWD = filepath.Join(filepath.Dir(prep.path), "2600-foreign-cycle")
	foreign.Repo = repo
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" || !strings.Contains(got.Reason, "does not match the current source or worker context") {
		t.Fatalf("foreign worktree reused prep-only lifecycle id: %#v", got)
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
	verifyArtifact := handoffEditRequest(record, worker, "claude", "owner-session", "")
	verifyArtifact.AgentID, verifyArtifact.Tool = "owner-agent", "Bash"
	verifyArtifact.Command = "agent-harness issueops remote verify-artifact --id " + record.ID + " --provider github --kind pr --url https://github.com/example/repo/pull/63 --target-branch " + record.BranchPrepare.BaseBranch + " --label bug --assignee octocat --json"
	if got := BuildLifecyclePreToolUseDecision(verifyArtifact); got.Decision != "allow" {
		t.Fatalf("exact owner existing PR verification blocked: %#v", got)
	}

	for _, command := range []string{
		"git push origin " + record.Branch,
		"gh pr create --head " + record.Branch,
		"glab mr create --source-branch " + record.Branch,
		"/usr/bin/git push origin " + record.Branch,
	} {
		directController := handoffEditRequest(record, worker, "claude", "owner-session", "")
		directController.AgentID, directController.Tool, directController.Command = "owner-agent", "Bash", command
		if got := BuildLifecyclePreToolUseDecision(directController); got.Decision != "block" {
			t.Fatalf("direct remote controller must be blocked: command=%q result=%#v", command, got)
		}
	}
	for _, command := range []string{
		"bash -c 'touch internal/x.go; git push origin " + record.Branch + "'",
		"bash -c 'touch internal/x.go; gh pr create --head " + record.Branch + "'",
	} {
		nestedController := handoffEditRequest(record, worker, "claude", "owner-session", "")
		nestedController.AgentID, nestedController.Tool, nestedController.Command = "owner-agent", "Bash", command
		if got := BuildLifecyclePreToolUseDecision(nestedController); got.Decision != "block" {
			t.Fatalf("nested remote controller must not bypass the sealed publication lifecycle: command=%q result=%#v", command, got)
		}
	}

	source := handoffEditRequest(record, repo, "claude", "coordinator-session", "")
	source.AgentID, source.Tool, source.ToolInput = "coordinator-agent", "mcp__agent_harness__issueops_handoff", owner.ToolInput
	if got := BuildLifecyclePreToolUseDecision(source); got.Decision != "block" {
		t.Fatalf("source session regained owner publish authority: %#v", got)
	}
	verifyArtifact.Repo, verifyArtifact.CWD = repo, repo
	verifyArtifact.SessionID, verifyArtifact.AgentID = "coordinator-session", "coordinator-agent"
	if got := BuildLifecyclePreToolUseDecision(verifyArtifact); got.Decision != "block" {
		t.Fatalf("source session recorded owner remote artifact: %#v", got)
	}
}

func TestOwnershipOwnerCanAdvanceAndRecordAISlopClean(t *testing.T) {
	repo, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	commands := []string{
		"agent-harness issueops phase --id " + record.ID + " --to implement --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json",
		"agent-harness issueops ai-slop-clean record --id " + record.ID + " --category dead-code --verification 'go test ./...' --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json",
		"agent-harness issueops feedback mark-issue-updated --id " + record.ID + " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json",
		"agent-harness issueops feedback resolve --id " + record.ID + " --index 0 --resolution valid-defect --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json",
	}
	for _, command := range commands {
		owner := handoffEditRequest(record, worker, "claude", "owner-session", "")
		owner.AgentID, owner.Tool, owner.Command = "owner-agent", "Bash", command
		if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
			t.Fatalf("exact owner recorder blocked: command=%q result=%#v", command, got)
		}

		source := owner
		source.Repo, source.CWD = repo, repo
		source.SessionID, source.AgentID = "coordinator-session", "coordinator-agent"
		if got := BuildLifecyclePreToolUseDecision(source); got.Decision != "block" {
			t.Fatalf("source session used owner recorder: command=%q result=%#v", command, got)
		}

		otherOwner := owner
		otherOwner.SessionID = "other-owner"
		if got := BuildLifecyclePreToolUseDecision(otherOwner); got.Decision != "block" {
			t.Fatalf("different worker session used owner recorder: command=%q result=%#v", command, got)
		}
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

	_, record, worker := ownershipLifecycleRecord(t, handoff.StateOwnerActive)
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: strings.Repeat("a", 64),
		Branch: record.Branch, Base: record.BranchPrepare.BaseBranch, RemoteRef: "refs/heads/" + record.Branch, FinalHead: strings.Repeat("f", 40), VerifiedAt: "2026-07-20T00:00:00Z",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	guidance = BuildIssueOpsHandoffSessionGuidance(worker, "claude", "owner-session", "owner-agent")
	for _, expected := range []string{"issueops phase", "ai-slop-clean record", "remote verify-artifact", "owner"} {
		if !strings.Contains(guidance, expected) {
			t.Fatalf("published owner guidance must name %q: %s", expected, guidance)
		}
	}
}

func ownershipLifecycleRecord(t *testing.T, state string) (string, IssueOpsRecord, string) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-demo", IssueOpsPhaseImplement)
	record, ok := ActiveIssueOpsCycleForBranch(repo, "1-demo")
	if !ok {
		t.Fatal("active record missing")
	}
	worker := makeIssueOpsGuardWorktreeForTest(t, repo, "1-demo")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-demo")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worker); err != nil {
		t.Fatal(err)
	}
	record, _ = ReadIssueOps(IssueOpsStateRoot(), record.ID)
	baseHead := strings.Repeat("b", 40)
	orca := &issueopsmodel.IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/1-demo", WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worker, WorkerPTYID: "pty-1", WorkerTerminalHandle: "term-1", WorkerMailboxHandle: "term-1", TaskID: "task-1", DispatchID: "dispatch-1"}
	h := &issueopsmodel.IssueOpsExecutionHandoff{
		State: state, Attempt: 1, OwnershipEpoch: "ownership-epoch-1", WorkspaceEpoch: "workspace-epoch-1", WorkspaceSHA256: strings.Repeat("c", 64),
		AttemptBaseHead: baseHead, ContextSHA256: strings.Repeat("a", 64), ContextSourceSHA256: strings.Repeat("d", 64), ContextVersion: handoff.ContextVersion,
		Driver: "orca", Agent: "claude", DeliveryMode: "inject", CoordinatorRoot: repo, CoordinatorMailboxHandle: "term-coordinator", WorkerRoot: worker, Orca: orca,
		CoordinatorSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"},
	}
	record.ExecutionHandoff = h
	workspaceOrca := *orca
	workspaceOrca.WorkerPTYID, workspaceOrca.WorkerTerminalHandle, workspaceOrca.WorkerMailboxHandle = "", "", ""
	workspaceOrca.TaskID, workspaceOrca.DispatchID = "", ""
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: h.WorkspaceEpoch, Driver: "orca", Agent: "claude",
		CoordinatorRoot: repo, WorkerRoot: worker,
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"},
		BaseHead:           baseHead, Orca: &workspaceOrca,
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

func cloneOwnershipHandoffForTest(value *issueopsmodel.IssueOpsExecutionHandoff) *issueopsmodel.IssueOpsExecutionHandoff {
	cloned := *value
	if value.OwnerSession != nil {
		owner := *value.OwnerSession
		cloned.OwnerSession = &owner
	}
	if value.Orientation != nil {
		orientation := *value.Orientation
		cloned.Orientation = &orientation
	}
	if value.Orca != nil {
		orca := *value.Orca
		cloned.Orca = &orca
	}
	return &cloned
}

func handoffEditRequest(record IssueOpsRecord, cwd, host, session, target string) HookToolUseLifecycleRequest {
	paths := []string(nil)
	if target != "" {
		paths = []string{target}
	}
	return HookToolUseLifecycleRequest{
		Repo: cwd, CWD: cwd, Host: host, SessionID: session, AgentID: "worker-1", Tool: "Edit", Paths: paths,
		EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: record.Repo,
	}
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
