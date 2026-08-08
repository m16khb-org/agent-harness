package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// claimable은 홀더 부재가 강제되는 상태다(model/execution.go의 validateWriteLease).
// released와 그 성질을 공유하므로 abandon이 거부할 근거가 없다. 거부하면
// 운영자는 claim→release로 lease를 한 바퀴 돌리는 우회를 하게 되는데, 그 두
// 단계는 아무것도 정리하지 않는다(#140).
func TestAbandonAllowsHolderlessLease(t *testing.T) {
	for _, status := range []issueops.LeaseStatus{issueops.LeaseStatusClaimable, issueops.LeaseStatusReleased} {
		t.Run(string(status), func(t *testing.T) {
			stateRoot, record := abandonLeaseRecord(t, status)

			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
				abandonOrcaDeps(&fakeAbandonGit{}, &fakeOwnerInspector{}))
			if err != nil {
				t.Fatalf("a holderless lease must not block abandon: %v (%v)", err, result.Missing)
			}
			if containsString(result.Missing, "lease_terminal") {
				t.Fatalf("%s has no holder and must pass the lease gate: %v", status, result.Missing)
			}
		})
	}
}

// 홀더를 가진 상태는 계속 거부한다. active는 살아 있는 writer가 있고,
// revoking은 해제된 상태가 아니라 fenced holder를 여전히 보유한다.
func TestAbandonRejectsLeaseWithHolder(t *testing.T) {
	for _, status := range []issueops.LeaseStatus{issueops.LeaseStatusActive, issueops.LeaseStatusRevoking} {
		t.Run(string(status), func(t *testing.T) {
			stateRoot, record := abandonLeaseRecord(t, status)

			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
				abandonOrcaDeps(&fakeAbandonGit{}, &fakeOwnerInspector{}))
			if err == nil || !containsString(result.Missing, "lease_terminal") {
				t.Fatalf("%s still holds a writer and must block: %v %v", status, err, result.Missing)
			}
		})
	}
}

// claimable 허용이 다른 게이트의 보호를 우회하지 않는다. orca 자원이 살아
// 있으면 lease가 claimable이어도 막힌다.
func TestAbandonClaimableStillRespectsOrcaResidueGate(t *testing.T) {
	stateRoot, record := abandonLeaseRecord(t, issueops.LeaseStatusClaimable)
	record.Execution.Mode = issueops.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &issueops.OrcaBinding{
		RuntimeID: "runtime-140", RepoID: "repo-140", WorktreeID: "worktree-140",
		OwnerHost: "claude", OwnerModel: "claude-opus-5", TerminalPTYID: "pty-140",
		TaskID: "task-140", DispatchID: "dispatch-140",
	}
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	inspector := &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{TaskLive: true, TaskStatus: "dispatched"}}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, inspector))
	if err == nil || !containsString(result.Missing, "orca_resources_absent") {
		t.Fatalf("a claimable lease must not bypass the orca residue gate: %v %v", err, result.Missing)
	}
	if containsString(result.Missing, "lease_terminal") {
		t.Fatalf("the lease gate itself must pass for claimable: %v", result.Missing)
	}
}

// 게이트 ⑤가 차단할 때 운영자가 다음 명령을 알 수 있어야 한다. reconcile을
// 지시하는 것만으로는 부족하다 — reconcile을 완주한 뒤 무엇을 해야 하는지가
// 실측에서 가장 큰 장애물이었다(#139).
func TestAbandonPendingIntentBlockNamesTheFullPath(t *testing.T) {
	stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "dispatch", true)

	result, _ := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""),
		abandonOrcaDeps(&fakeAbandonGit{}, &fakeOwnerInspector{}))
	if !containsString(result.Missing, "pending_intent_safe") {
		t.Fatalf("a dispatch-stage intent must still block: %v", result.Missing)
	}
	for _, want := range []string{"reconcile", "worktree"} {
		if !strings.Contains(result.PendingIntentError, want) {
			t.Fatalf("the block reason %q must describe the remaining path (%s)", result.PendingIntentError, want)
		}
	}
}

func abandonLeaseRecord(t *testing.T, status issueops.LeaseStatus) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	lease := issueops.WriteLease{Generation: 1, Status: status}
	switch status {
	case issueops.LeaseStatusClaimable:
		lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	case issueops.LeaseStatusActive, issueops.LeaseStatusRevoking:
		holder := executionActor("claude", "abandon-lease-session")
		lease.Holder = &holder
		lease.ClaimedAt = "2026-07-26T00:00:00Z"
	case issueops.LeaseStatusReleased:
		lease.ReleasedAt = "2026-07-26T00:00:02Z"
	}
	record.Execution = &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: record.Repo, Root: record.Repo + ".worktrees/deleted-140",
			Branch: record.Branch, BaseHead: "0000000000000000000000000000000000000000",
			Driver: "git", LinkedAt: "2026-07-26T00:00:00Z",
		},
		Lease: lease,
	}
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}
