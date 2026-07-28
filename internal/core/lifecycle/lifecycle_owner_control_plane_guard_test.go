package lifecycle

import (
	"strings"
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

func TestExecutionAdmitsExactOrcaOwnerControlPlaneCommands(t *testing.T) {
	for _, leaseStatus := range []issueopsmodel.LeaseStatus{
		issueopsmodel.LeaseStatusClaimable,
		issueopsmodel.LeaseStatusActive,
	} {
		t.Run(string(leaseStatus), func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			_, record, worker := executionActiveLifecycleRecord(t)
			if leaseStatus == issueopsmodel.LeaseStatusClaimable {
				record.Execution.Lease = issueopsmodel.WriteLease{
					Generation:       3,
					Status:           issueopsmodel.LeaseStatusClaimable,
					ClaimTokenSHA256: strings.Repeat("a", 64),
				}
				if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
					t.Fatal(err)
				}
			}

			for name, command := range map[string]string{
				"heartbeat":   `orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject "alive" --task-id task-1 --dispatch-id ctx-1 --phase "reviewing"`,
				"worker done": "orca orchestration send --to term-coordinator --from term-worker --type worker_done --subject paused --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --files-modified '' --json",
				"escalation":  `orca orchestration send --to term-coordinator --from term-worker --type escalation --subject "Blocked: hook" --body "The exact read was denied." --task-id task-1 --dispatch-id ctx-1`,
				"ask":         `orca orchestration ask --to term-coordinator --from term-worker --question "Continue after repair?" --options yes,no --timeout-ms 600000`,
				"check":       "orca orchestration check --terminal term-worker --wait --timeout-ms 600000 --json",
			} {
				t.Run(name, func(t *testing.T) {
					req := executionRequest(record, worker, "codex", "owner-session", command)
					req.AgentID = "owner-agent"
					if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
						t.Fatalf("Orca owner 제어면은 %s lease에서도 실행 가능해야 한다: %+v", leaseStatus, got)
					}
				})
			}
		})
	}
}

func TestExecutionKeepsNearMissOrcaOwnerControlPlaneCommandsBlocked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"unknown message type":    "orca orchestration send --to term-coordinator --from term-worker --type delete --subject no --body no",
		"unknown flag":            "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase reviewing --force",
		"missing heartbeat phase": "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1",
		"shell substitution":      "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject $(whoami) --task-id task-1 --dispatch-id ctx-1 --phase reviewing",
		"unrelated mutation":      "orca orchestration task-update --id task-1 --status completed --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "codex", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s 명령이 Orca mutation 표면을 넓히면 안 된다: %+v", name, got)
			}
		})
	}
}
