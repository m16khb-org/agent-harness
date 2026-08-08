package lifecycle

import (
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func linkWaitRecord() issueopscontract.IssueOpsRecord {
	return issueopscontract.IssueOpsRecord{
		OK: true, ID: "io-linkwait", Repo: "/repo",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/o/r/issues/7",
			Branch: "7-work", BaseBranch: "main", BaseSHA: "abc123",
		},
	}
}

// TestBranchLinkWaitIsAdmittedInsideThePreLinkWindow는 #319의 가드 경계를
// 고정한다. 이 창에서 owner가 쓸 수 있는 명령이 recorder 하나뿐이면, 링크가
// 아직 없는 시점에 시작한 owner는 기다릴 방법이 없어 실패로 종료한다.
func TestBranchLinkWaitIsAdmittedInsideThePreLinkWindow(t *testing.T) {
	record := linkWaitRecord()
	if !exactOrcaBranchLinkWait("agent-harness issueops branch await-link --id io-linkwait --json", record) {
		t.Fatal("이 lifecycle을 지목한 대기 reader는 허용돼야 한다")
	}
	if !exactOrcaBranchLinkWait("agent-harness issueops branch await-link --id io-linkwait --timeout 5m --json", record) {
		t.Fatal("--timeout이 붙어도 허용돼야 한다")
	}
}

// TestBranchLinkWaitRefusesAnythingButTheExactForm는 허용이 넓어지지 않음을
// 고정한다. 읽기 전용이라도 다른 lifecycle이나 다른 명령까지 열면 안 되고,
// 셸 제어 연산자가 붙은 형태는 더더욱 인정하면 안 된다.
func TestBranchLinkWaitRefusesAnythingButTheExactForm(t *testing.T) {
	record := linkWaitRecord()
	chained := "agent-harness issueops branch await-link --id io-linkwait --json && git push --force"
	for _, command := range []string{
		"agent-harness issueops branch await-link --id io-other --json",
		"agent-harness issueops branch await-link --json",
		"agent-harness issueops branch prepare --id io-linkwait --json",
		"agent-harness issueops execution status --id io-linkwait --json",
		chained,
		"gh issue develop --list 7",
	} {
		if exactOrcaBranchLinkWait(command, record) {
			t.Fatalf("허용하면 안 되는 형태다: %q", command)
		}
	}
}

// TestPreLinkDenyNamesTheWaitCommand는 진단이 다음 행동을 가리키는지
// 고정한다. 차단 사유만 있고 기다릴 방법이 없으면 owner는 실패로 종료한다 —
// 실측된 실패(task_e3946ef93086)가 정확히 그것이었다.
func TestPreLinkDenyNamesTheWaitCommand(t *testing.T) {
	record := linkWaitRecord()
	record.Execution = &issueopscontract.Execution{
		Mode:  issueopscontract.ExecutionModeOrca,
		Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive},
	}
	if !orcaBranchLinkVerificationRequired(record) {
		t.Fatal("link 미검증 Orca lifecycle은 pre-link 창에 있다")
	}
	reason := orcaBranchLinkDenyReason(record)
	for _, needle := range []string{"await-link --id io-linkwait", "waits for the coordinator"} {
		if !strings.Contains(reason, needle) {
			t.Fatalf("진단이 %q를 담아야 한다: %s", needle, reason)
		}
	}
}

// TestBranchLinkWaitCountsAsAnObservation은 이 수정이 무의미해지지 않도록
// 고정한다. 관찰로 인정되지 않으면 lease를 든 owner의 셸에서 미분류로
// 차단되고, 대기 명령이 있어도 실행할 수 없다.
func TestBranchLinkWaitCountsAsAnObservation(t *testing.T) {
	observe := func(command string) bool {
		return executionObservation(lifecyclecontract.HookToolUseLifecycleRequest{Tool: "Bash", Command: command})
	}
	if !observe("agent-harness issueops branch await-link --id io-linkwait --json") {
		t.Fatal("await-link는 읽기 전용 관찰이다")
	}
	if observe("agent-harness issueops branch await-link --json") {
		t.Fatal("--id 없는 형태는 관찰로 인정하지 않는다")
	}
	if observe("agent-harness issueops branch prepare --id io-linkwait --json") {
		t.Fatal("branch prepare는 관찰이 아니다")
	}
}
