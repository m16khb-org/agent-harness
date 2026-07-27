package lifecycle

import (
	"strings"
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func claimableExecutionGuardFixture(t *testing.T) (IssueOpsRecord, string) {
	t.Helper()
	_, record, worker := executionActiveLifecycleRecord(t)
	record.Execution.Lease = issueopsmodel.WriteLease{
		Generation: 1, Status: issueopsmodel.LeaseStatusClaimable,
		ClaimTokenSHA256: strings.Repeat("a", 64),
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return record, worker
}

// The injected Orca contract requires these messages before and after the
// write lease is claimed. They mutate the Orca coordination ledger, not the
// sealed Git worktree, so a claimable lease must not make the contract
// impossible to follow.
func TestClaimableExecutionAdmitsExactOrcaOwnerControlPlaneCommands(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, worker := claimableExecutionGuardFixture(t)

	for name, command := range map[string]string{
		"heartbeat":   "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase investigating",
		"worker done": "orca orchestration send --to term-coordinator --from term-worker --type worker_done --subject 'Blocked safely' --body 'No bytes changed; exact blocker recorded; owner stopped.' --task-id task-1 --dispatch-id ctx-1 --files-modified ''",
		"escalation":  "orca orchestration send --to term-coordinator --from term-worker --type escalation --subject 'Blocked: hook' --body 'The exact read was denied.' --task-id task-1",
		"ask":         "orca orchestration ask --to term-coordinator --from term-worker --question 'Continue after repair?' --options yes,no --timeout-ms 600000",
		"check":       "orca orchestration check --terminal term-worker --wait --timeout-ms 600000",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("the injected owner contract must remain executable before claim: %+v", got)
			}
		})
	}
}

func TestClaimableExecutionKeepsNearMissOrcaControlCommandsBlocked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, worker := claimableExecutionGuardFixture(t)

	for name, command := range map[string]string{
		"unknown message type":   "orca orchestration send --to term-coordinator --from term-worker --type delete --subject no --body no",
		"unknown flag":           "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase investigating --force",
		"missing worker summary": "orca orchestration send --to term-coordinator --from term-worker --type worker_done --subject done --task-id task-1 --dispatch-id ctx-1",
		"shell substitution":     "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject $(whoami) --task-id task-1 --dispatch-id ctx-1 --phase investigating",
		"unrelated mutation":     "orca orchestration task-update --id task-1 --status completed --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s must not widen the Orca mutation surface: %+v", name, got)
			}
		})
	}
}
