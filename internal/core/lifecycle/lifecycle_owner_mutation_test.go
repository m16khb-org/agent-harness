package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"
)

// AC-05: 하위 세션(owner)이 publication 전에 실행하는 implementation-review
// record는 4-flag 시그니처를 갖출 때만 owner mutation allowlist를 통과한다.
func TestExactIssueOpsOwnerMutationAdmitsImplementationReview(t *testing.T) {
	command := "agent-harness issueops implementation-review record --id io-000000000083 --verdict pass" +
		" --finding '경계 검토 완료' --evidence 'go test ok' --reviewer-host codex --reviewer-model gpt-5.6-sol" +
		" --host codex --session-id sess-1 --agent-id none --cwd /tmp/wt --json"
	if !exactIssueOpsOwnerMutation(command) {
		t.Fatalf("well-formed implementation-review record must pass the owner allowlist: %s", command)
	}
	for _, drop := range []string{"--session-id sess-1 ", "--host codex ", "--cwd /tmp/wt ", "--id io-000000000083 "} {
		broken := strings.Replace(command, drop, "", 1)
		if exactIssueOpsOwnerMutation(broken) {
			t.Fatalf("missing %s must fail the 4-flag signature: %s", strings.TrimSpace(drop), broken)
		}
	}
}

// 이슈 #90 도그푸드: handoff 시점에 branch_link_verified가 비어 있으면
// link-plan gate가 막히는데, active lease에서는 holder만 레코드를 고칠 수
// 있으므로 branch prepare도 4-flag owner mutation으로 admit되어야 한다.
func TestExactIssueOpsOwnerMutationAdmitsBranchPrepare(t *testing.T) {
	command := "agent-harness issueops branch prepare --id io-000000000089 --provider github" +
		" --issue-url 'https://github.com/acme/repo/issues/89' --branch 89-atomic --base-branch main" +
		" --base-sha 635303af758fae465d6e6fe30302fed9233180c5 --parent-worktree /tmp/repo.worktrees/main --link-verified" +
		" --host codex --session-id sess-1 --cwd /tmp/wt --json"
	if !exactIssueOpsOwnerMutation(command) {
		t.Fatalf("well-formed branch prepare must pass the owner allowlist: %s", command)
	}
	if exactIssueOpsOwnerMutation(strings.Replace(command, "--session-id sess-1 ", "", 1)) {
		t.Fatal("branch prepare without session-id must fail the 4-flag signature")
	}
}

// 현재 holder가 전달하는 session-executable은 native identity 영수증이지
// 워크트리 변경 대상이 아니다. 설치된 Codex/Claude 실행 파일은 보통 워크트리
// 밖에 있으므로 이 값을 경로 fence에 넣으면 정상 publication도 차단된다.
func TestRemoteCreatePRAllowsCurrentHolderWithExternalSessionExecutable(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	executable := filepath.Join(t.TempDir(), "codex", "bin", "codex")
	command := "agent-harness issueops remote create-pr --id " + record.ID +
		" --expected-generation 1 --title 'IssueOps lease release differential vertical 검증'" +
		" --head 191-issueops-lease-differential-spike --base 117-hexagonal-architecture-migration" +
		" --body '현재 holder의 governed preview 검증' --label enhancement --assignee m16khb" +
		" --host claude --session-id owner-session --session-pid 1234" +
		" --session-started-at 2026-07-22T00:00:00Z --session-executable " + executable +
		" --cwd " + worker + " --json"

	holder := executionRequest(record, worker, "claude", "owner-session", command)
	holder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("외부 session-executable 영수증을 가진 현재 holder의 create-pr preview가 차단됐다: %+v", got)
	}

	foreign := holder
	foreign.SessionID = "other-session"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" ||
		got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("같은 create-pr 명령을 실행한 비-holder는 identity fence에 차단돼야 한다: %+v", got)
	}
}

// execution prepare 이후에도 grill 재진입과 계획 보강에 필요한 레코더는
// 현재 holder가 사용할 수 있어야 한다. 각 명령은 등록된 플래그와 4-flag
// identity 시그니처를 모두 만족할 때만 owner mutation으로 분류한다.
func TestPlanningOwnerMutationsRemainAvailableAfterExecutionPrepare(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actorFlags := " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	commands := map[string]string{
		"intent record": "agent-harness issueops intent record --id " + record.ID +
			" --raw-request '관측성 보강' --interpreted-intent 'breaker 원인과 상태를 노출'" +
			" --success-criteria '원인 분류를 검증'" + actorFlags,
		"domain-review record": "agent-harness issueops domain-review record --id " + record.ID +
			" --model-fit '기존 breaker 상태 모델을 유지' --terminology 'open state'" +
			" --risk '고카디널리티 방지'" + actorFlags,
		"regress": "agent-harness issueops regress --id " + record.ID +
			" --reason 'Brooks revise 반영을 위해 grill로 복귀'" + actorFlags,
		"remote reflect-devils-advocate": "agent-harness issueops remote reflect-devils-advocate --id " + record.ID +
			" --provider gitlab --confirm" + actorFlags,
	}

	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			holder := executionRequest(record, worker, "claude", "owner-session", command)
			holder.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
				t.Fatalf("현재 holder의 %s 명령이 차단됐다: %+v", name, got)
			}

			foreign := holder
			foreign.SessionID = "other-session"
			if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" ||
				got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
				t.Fatalf("비-holder의 %s 명령은 identity fence에 차단돼야 한다: %+v", name, got)
			}
		})
	}
}
