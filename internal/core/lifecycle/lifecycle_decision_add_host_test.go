package lifecycle

import (
	"testing"

	issueopsmodel "agent-harness/internal/core/issueops/model"
)

// #158이 `decision add`를 owner mutation allowlist에 넣었지만 그 테스트는 claude
// holder만 검증했다. 이 저장소는 first-party로 codex·claude를 대등하게 지원하므로
// codex holder에서도 같게 동작해야 한다(#164).
//
// host는 홀더 정체의 한 축이다(`executionActorMismatchAxis`). 그 축이 codex일 때
// 분류가 달라지면 codex 사이클은 구현 중 결정을 기록할 수 없게 된다.
func TestDecisionAddReachesHolderFenceForCodexHost(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "69-codex-decision", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, repo, "69-codex-decision")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.Execution{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.Workspace{
			SourceRoot: repo, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-26T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLease{
			Generation: 1, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-26T00:00:00Z",
			Holder: &issueopsmodel.NativeActor{
				Host: "codex", SessionID: "codex-session", AgentID: "codex-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceipt{PID: 4321, StartedAt: "2026-07-26T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	command := "agent-harness issueops decision add --id " + record.ID +
		" --title 구현중결정 --body 근거 --kind implementation" +
		" --host codex --session-id codex-session --agent-id codex-agent --cwd " + linked.path
	req := executionRequest(record, linked.path, "codex", "codex-session", command)
	req.AgentID = "codex-agent"

	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision == "block" {
		t.Fatalf("codex holder도 자기 사이클의 결정을 기록할 수 있어야 한다: %+v (deny=%+v)", got, got.Deny)
	}
}

// codex 사이클에서도 비홀더는 거부된다. allowlist가 host에 따라 느슨해지지 않는다.
func TestDecisionAddFromNonHolderStaysBlockedForCodexHost(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "69-codex-decision-deny", IssueOpsPhasePlan)
	linked := linkIssueOpsWorktreeForGuardTest(t, repo, "69-codex-decision-deny")
	record, err := ReadIssueOps(IssueOpsStateRoot(), linked.id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopsmodel.Execution{
		Mode: issueopsmodel.ExecutionModeDirect,
		Workspace: issueopsmodel.Workspace{
			SourceRoot: repo, Root: linked.path, Branch: record.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-26T00:00:00Z",
		},
		Lease: issueopsmodel.WriteLease{
			Generation: 1, Status: issueopsmodel.LeaseStatusActive, ClaimedAt: "2026-07-26T00:00:00Z",
			Holder: &issueopsmodel.NativeActor{
				Host: "codex", SessionID: "codex-session", AgentID: "codex-agent",
				SessionProcess: &issueopsmodel.NativeProcessReceipt{PID: 4321, StartedAt: "2026-07-26T00:00:00Z", Executable: "codex"},
			},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	command := "agent-harness issueops decision add --id " + record.ID +
		" --title 구현중결정 --body 근거 --kind implementation" +
		" --host codex --session-id other-session --agent-id codex-agent --cwd " + linked.path
	req := executionRequest(record, linked.path, "codex", "other-session", command)
	req.AgentID = "codex-agent"

	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("codex 비홀더도 write lease 뒤에 남아야 한다: %+v", got)
	}
}
