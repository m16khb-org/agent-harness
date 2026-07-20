package lifecycle

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
)

// TestCoordinatorPreparingGuidanceEmitsRunnableDispatchCommand proves the
// coordinator-dispatch reachability fix (Task G1): at coordinator_preparing the
// source-checkout guidance now hands the coordinator a harness-authored,
// identity-filled `issueops handoff start` preview command. The identity flags
// are copied verbatim from the authenticated session, never guessed, and the
// emitted command still satisfies the unchanged hook fence.
func TestCoordinatorPreparingGuidanceEmitsRunnableDispatchCommand(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)

	guidance := BuildIssueOpsHandoffSessionGuidance(repo, "codex", "coordinator", "worker-1")
	if !strings.Contains(guidance, "role=coordinator") {
		t.Fatalf("coordinator guidance lost role marker: %s", guidance)
	}
	command := buildCoordinatorDispatchCommand(record, "codex", "coordinator", "worker-1")
	if !strings.Contains(guidance, command) {
		t.Fatalf("coordinator guidance did not surface runnable dispatch command %q: %s", command, guidance)
	}
	for _, want := range []string{"handoff start", "--coordinator-host", "--coordinator-session-id", "--coordinator-agent-id", "--source-cwd", "--expected-context-sha256", "--confirm"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("coordinator dispatch guidance missing %q: %s", want, guidance)
		}
	}

	// Identity is copied verbatim from the authenticated event, never guessed.
	for _, want := range []string{"--coordinator-host 'codex'", "--coordinator-session-id 'coordinator'", "--coordinator-agent-id 'worker-1'"} {
		if !strings.Contains(command, want) {
			t.Fatalf("dispatch command missing exact identity flag %q: %s", want, command)
		}
	}

	// Feeding the emitted command verbatim through the hook is allowed:
	// reachability without weakening the fence.
	base := handoffEditRequest(record, repo, "codex", "coordinator", "")
	base.Tool = "Bash"
	base.Command = command
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
		t.Fatalf("harness-authored dispatch command must pass the hook: command=%q got=%#v", command, got)
	}
	if !handoff.CoordinatorIdentityMatches(record, issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator", AgentID: "worker-1"}, repo) {
		t.Fatalf("coordinator identity must match the sealed session for the emitted command")
	}
}

// TestCoordinatorPreparingGuidanceOmitsAgentFlagWhenAgentless confirms the
// emitted command matches an agentless native session (no --coordinator-agent-id),
// mirroring the eventIdentityFlagsMatch agentless rule.
func TestCoordinatorPreparingGuidanceOmitsAgentFlagWhenAgentless(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	record.ExecutionHandoff.CoordinatorSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator"}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	command := buildCoordinatorDispatchCommand(record, "codex", "coordinator", "")
	if strings.Contains(command, "--coordinator-agent-id") {
		t.Fatalf("agentless dispatch command must omit --coordinator-agent-id: %s", command)
	}
	base := handoffEditRequest(record, repo, "codex", "coordinator", "")
	base.AgentID = ""
	base.Tool = "Bash"
	base.Command = command
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
		t.Fatalf("agentless dispatch command must pass the hook: command=%q got=%#v", command, got)
	}
}

func TestCoordinatorPreparingBootstrapProbeReturnsAuthenticatedRunnableCommand(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	record.ExecutionHandoff.CoordinatorSession = nil
	record.ExecutionHandoff.CoordinatorMailboxHandle = ""
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	probe := handoffEditRequest(record, repo, "codex", "fresh-coordinator", "")
	probe.AgentID = "agent-31"
	probe.Tool = "Bash"
	probe.Command = "agent-harness issueops handoff start --id " + record.ID + " --coordinator-recipient term_bootstrap_31 --source-cwd " + repo + " --json"
	got := BuildLifecyclePreToolUseDecision(probe)
	if got.Decision != "block" || !strings.Contains(got.Reason, "harness-authored preview command") {
		t.Fatalf("bootstrap probe must be blocked with runnable guidance: %#v", got)
	}
	want := "agent-harness issueops handoff start --id '" + record.ID + "' --coordinator-recipient 'term_bootstrap_31' --coordinator-host 'codex' --coordinator-session-id 'fresh-coordinator' --coordinator-agent-id 'agent-31' --source-cwd '" + repo + "' --json"
	if !strings.Contains(got.Reason, want) {
		t.Fatalf("bootstrap guidance missing authenticated command %q: %s", want, got.Reason)
	}
	probe.Command = want
	if got := BuildLifecyclePreToolUseDecision(probe); got.Decision != "allow" {
		t.Fatalf("returned bootstrap command must pass the lifecycle fence: %#v", got)
	}

	for _, command := range []string{
		"agent-harness issueops handoff start --id " + record.ID + " --coordinator-recipient terminal_bootstrap_31 --source-cwd " + repo,
		"agent-harness issueops handoff start --id " + record.ID + " --coordinator-recipient term_bootstrap_31 --source-cwd /wrong",
		"agent-harness issueops handoff start --id " + record.ID + " --coordinator-recipient term_bootstrap_31 --source-cwd " + repo + " --confirm",
	} {
		probe.Command = command
		if got := BuildLifecyclePreToolUseDecision(probe); got.Decision != "block" || strings.Contains(got.Reason, "harness-authored preview command") {
			t.Fatalf("invalid bootstrap probe must remain normally blocked: command=%q got=%#v", command, got)
		}
	}
}

func TestOwnershipTransferBootstrapRendersRunnableNativeStart(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	workspaceOrca := *record.ExecutionHandoff.Orca
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "workspace-epoch-1", Driver: "orca", Agent: "codex",
		CoordinatorRoot: repo, WorkerRoot: record.WorktreePath,
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "fresh-coordinator", AgentID: "agent-31"},
		BaseHead:           record.ExecutionHandoff.AttemptBaseHead, Orca: &workspaceOrca,
	}
	record.ExecutionHandoff = nil
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	probe := handoffEditRequest(record, repo, "codex", "fresh-coordinator", "")
	probe.AgentID = "agent-31"
	probe.Tool = "Bash"
	probe.Command = "agent-harness issueops handoff start --id " + record.ID + " --source-cwd " + repo + " --json"
	got := BuildLifecyclePreToolUseDecision(probe)
	if got.Decision != "block" || !strings.Contains(got.Reason, "harness-authored preview command") {
		t.Fatalf("ownership bootstrap probe must be blocked with runnable guidance: %#v", got)
	}
	want := "agent-harness issueops handoff start --id '" + record.ID + "' --coordinator-host 'codex' --coordinator-session-id 'fresh-coordinator' --coordinator-agent-id 'agent-31' --source-cwd '" + repo + "' --workspace-epoch 'workspace-epoch-1' --json"
	if !strings.Contains(got.Reason, want) {
		t.Fatalf("ownership bootstrap guidance missing authenticated command %q: %s", want, got.Reason)
	}
	probe.Command = want
	if got := BuildLifecyclePreToolUseDecision(probe); got.Decision != "allow" {
		t.Fatalf("ownership bootstrap replacement must pass the lifecycle fence: %#v", got)
	}
}

func TestOwnershipTransferPreparationAllowsCLIAndMCPBeforeDispatch(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	workspaceOrca := *record.ExecutionHandoff.Orca
	record.ExecutionWorkspace = &issueopsmodel.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "workspace-epoch-1", Driver: "orca", Agent: "codex",
		CoordinatorRoot: repo, WorkerRoot: record.WorktreePath,
		PreparationSession: &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "preparer", AgentID: "agent-1"},
		BaseHead:           record.ExecutionHandoff.AttemptBaseHead, Orca: &workspaceOrca,
	}
	record.ExecutionHandoff = nil
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	request := handoffEditRequest(record, repo, "codex", "other-source-session", "")
	request.Tool = "Bash"
	request.Command = "agent-harness issueops link-plan --id " + record.ID + " --plan-path plans/ownership.md --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "block" {
		t.Fatalf("different source session must not link the plan: %#v", got)
	}
	request.SessionID = "preparer"
	request.AgentID = "agent-1"
	request.Command = "agent-harness issueops link-plan --id " + record.ID + " --plan-path plans/ownership.md --host codex --session-id preparer --agent-id agent-1 --cwd " + repo + " --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("sealed source coordinator link-plan must remain allowed before ownership dispatch: %#v", got)
	}
	request.Command = "agent-harness issueops phase --id " + record.ID + " --to implement --host codex --session-id preparer --agent-id agent-1 --cwd " + repo + " --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("sealed source coordinator phase advance must remain allowed before ownership dispatch: %#v", got)
	}
	request.Command = "agent-harness issueops phase --id " + record.ID + " --to implement --host codex --session-id other --agent-id agent-1 --cwd " + repo + " --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "block" {
		t.Fatalf("different source session gained phase authority: %#v", got)
	}
	request.Tool = "mcp__agent_harness__issueops_link_plan"
	request.Command = ""
	request.ToolInput = map[string]any{"id": record.ID, "plan_path": "plans/ownership.md", "host": "codex", "session_id": "preparer", "agent_id": "agent-1", "cwd": repo}
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("sealed source coordinator MCP plan link must remain allowed before ownership dispatch: %#v", got)
	}
	request.ToolInput["session_id"] = "other"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "block" {
		t.Fatalf("different MCP actor gained plan link authority: %#v", got)
	}
}

func TestCoordinatorPreparingAllowsExactCodexTrustObservationBeforeCycleSelection(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	request := handoffEditRequest(record, repo, "codex", "fresh-coordinator", "")
	request.Tool = "Bash"
	request.Command = "codex --help"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("exact Codex trust observation must remain read-only: %#v", got)
	}
}

// TestCoordinatorPreparingDispatchStillDeniesMismatchedIdentity keeps the fence
// intact: the emitted skeleton with a mismatched coordinator session is denied.
func TestCoordinatorPreparingDispatchStillDeniesMismatchedIdentity(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	command := buildCoordinatorDispatchCommand(record, "codex", "coordinator", "worker-1")
	base := handoffEditRequest(record, repo, "codex", "impostor", "")
	base.Tool = "Bash"
	base.Command = command
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "block" {
		t.Fatalf("dispatch command from a mismatched session must fail closed: %#v", got)
	}
}
