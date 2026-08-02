package lifecycle

import (
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestExecutionAdmitsExactOrcaOwnerControlPlaneCommands(t *testing.T) {
	for _, leaseStatus := range []issueopscontract.LeaseStatus{
		issueopscontract.LeaseStatusClaimable,
		issueopscontract.LeaseStatusActive,
	} {
		t.Run(string(leaseStatus), func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			_, record, worker := executionActiveLifecycleRecord(t)
			if leaseStatus == issueopscontract.LeaseStatusClaimable {
				record.Execution.Lease = issueopscontract.WriteLease{
					Generation:       3,
					Status:           issueopscontract.LeaseStatusClaimable,
					ClaimTokenSHA256: strings.Repeat("a", 64),
				}
				if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
					t.Fatal(err)
				}
			}

			for name, command := range map[string]string{
				"heartbeat":              `orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject "alive" --task-id task-1 --dispatch-id ctx-1 --phase "reviewing"`,
				"worker done":            "orca orchestration send --to term-coordinator --from term-worker --type worker_done --subject paused --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --files-modified '' --json",
				"escalation":             `orca orchestration send --to term-coordinator --from term-worker --type escalation --subject "Blocked: hook" --body "The exact read was denied." --task-id task-1 --dispatch-id ctx-1`,
				"ask":                    `orca orchestration ask --to term-coordinator --from term-worker --question "Continue after repair?" --options yes,no --timeout-ms 600000`,
				"capability heartbeat":   `orca orchestration send --from term_worker --dispatch-capability dcap_test --type heartbeat --subject "alive" --task-id task-1 --dispatch-id ctx-1 --phase "reviewing"`,
				"capability worker done": "orca orchestration send --from term_worker --dispatch-capability dcap_test --type worker_done --subject paused --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --outcome failed --files-modified '' --json",
				"capability escalation":  `orca orchestration send --from term_worker --dispatch-capability dcap_test --type escalation --subject "Blocked: hook" --body "The exact read was denied." --task-id task-1 --dispatch-id ctx-1`,
				"capability ask":         `orca orchestration ask --from term_worker --dispatch-capability dcap_test --question "Continue after repair?" --options yes,no --timeout-ms 600000`,
				"check":                  "orca orchestration check --terminal term-worker --wait --timeout-ms 600000 --json",
				"check unread":           "orca orchestration check --unread --inject --json",
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
		"unknown message type":       "orca orchestration send --to term-coordinator --from term-worker --type delete --subject no --body no",
		"unknown flag":               "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase reviewing --force",
		"missing heartbeat phase":    "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1",
		"mixed recipient authority":  "orca orchestration send --to term-coordinator --from term_worker --dispatch-capability dcap_test --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase reviewing",
		"capability missing sender":  "orca orchestration send --dispatch-capability dcap_test --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase reviewing",
		"capability missing outcome": "orca orchestration send --from term_worker --dispatch-capability dcap_test --type worker_done --subject stopped --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1",
		"capability bad outcome":     "orca orchestration send --from term_worker --dispatch-capability dcap_test --type worker_done --subject stopped --body safe-checkpoint --task-id task-1 --dispatch-id ctx-1 --outcome pending",
		"outcome on heartbeat":       "orca orchestration send --from term_worker --dispatch-capability dcap_test --type heartbeat --subject alive --task-id task-1 --dispatch-id ctx-1 --phase reviewing --outcome succeeded",
		"ask mixed authority":        "orca orchestration ask --to term-coordinator --from term_worker --dispatch-capability dcap_test --question continue",
		"ask capability no sender":   "orca orchestration ask --dispatch-capability dcap_test --question continue",
		"shell substitution":         "orca orchestration send --to term-coordinator --from term-worker --type heartbeat --subject $(whoami) --task-id task-1 --dispatch-id ctx-1 --phase reviewing",
		"inject without unread":      "orca orchestration check --inject --json",
		"unrelated mutation":         "orca orchestration task-update --id task-1 --status completed --json",
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

func TestExecutionAdmitsExactGenerationBoundResumeControlPlane(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	record.Execution.Mode = issueopscontract.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Lease = issueopscontract.WriteLease{
		Generation: 3, Status: issueopscontract.LeaseStatusClaimable,
		ClaimTokenSHA256: strings.Repeat("a", 64),
	}
	record.Execution.Orca = &issueopscontract.OrcaBinding{
		RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1",
		LeaseGeneration: 2, OwnerHost: "codex", OwnerModel: "gpt-5.6-terra",
		TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	command := "agent-harness issueops execution resume --id " + record.ID +
		" --expected-generation 3 --host codex --session-id resume-session --session-pid 42" +
		" --session-started-at 2026-07-30T00:00:00Z --session-executable /bin/codex" +
		" --cwd " + worker + " --confirm --json"
	req := executionRequest(record, worker, "codex", "resume-session", command)
	if !executionTypedControlPlane(req) {
		t.Fatal("exact resume did not reach the typed control plane")
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact resume was blocked by holderless authority: %+v", got)
	}
}

func TestExecutionKeepsNearMissResumeControlPlaneCommandsBlocked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	record.Execution.Mode = issueopscontract.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Lease = issueopscontract.WriteLease{
		Generation: 3, Status: issueopscontract.LeaseStatusClaimable,
		ClaimTokenSHA256: strings.Repeat("a", 64),
	}
	record.Execution.Orca = &issueopscontract.OrcaBinding{
		RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1",
		LeaseGeneration: 2, OwnerHost: "codex", OwnerModel: "gpt-5.6-terra",
		TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	base := "agent-harness issueops execution resume --id " + record.ID +
		" --expected-generation 3 --host codex --session-id resume-session --session-pid 42" +
		" --session-started-at 2026-07-30T00:00:00Z --session-executable /bin/codex" +
		" --cwd " + worker
	for name, command := range map[string]string{
		"missing confirm":      base + " --json",
		"missing generation":   strings.Replace(base, " --expected-generation 3", "", 1) + " --confirm --json",
		"unknown snapshot":     base + " --issue-snapshot-file /tmp/issue.json --confirm --json",
		"active substitution":  strings.Replace(base, "--expected-generation 3", "--expected-generation $(date +%s)", 1) + " --confirm --json",
		"empty lifecycle id":   strings.Replace(base, "--id "+record.ID, "--id ''", 1) + " --confirm --json",
		"duplicate generation": base + " --expected-generation 4 --confirm --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "codex", "resume-session", command)
			if executionTypedControlPlane(req) {
				t.Fatalf("%s near miss reached the typed control plane", name)
			}
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "unsafe_mutation" {
				t.Fatalf("%s near miss was not fail-closed: %+v", name, got)
			}
		})
	}
}
