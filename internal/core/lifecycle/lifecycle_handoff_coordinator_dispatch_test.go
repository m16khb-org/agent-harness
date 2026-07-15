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
