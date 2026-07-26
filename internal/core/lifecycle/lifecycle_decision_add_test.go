package lifecycle

import (
	"strings"
	"testing"
)

// 구현 중 설계 결정이 바뀌는 것은 정상이다. #152에서 preview 계약을 바꾸는 판단이
// implement 단계에 필요했는데, `decision add`가 owner mutation으로 분류되지 않아
// durable state에 기록할 수 없었다 — Turing 리포트와 PR 본문에만 남겼다.
//
// IssueOps는 결정을 durable state에 담아 나중 사이클이 plan-prep의 prior-decisions로
// 조회하게 설계됐다. 문서에만 남은 결정은 그 경로에 들어오지 않는다(#158).
func TestDecisionAddReachesCanonicalHolderFence(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worktree := executionActiveLifecycleRecord(t)

	command := "agent-harness issueops decision add --id " + record.ID +
		" --title 계약변경 --body 근거 --kind architecture" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worktree
	req := executionRequest(record, worktree, "claude", "owner-session", command)
	req.AgentID = "owner-agent"

	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision == "block" {
		t.Fatalf("holder는 자기 사이클의 결정을 기록할 수 있어야 한다: %+v (deny=%+v)", got, got.Deny)
	}
}

// 홀더가 아닌 세션은 여전히 거부된다. allowlist에 넣는 것이 홀더 검증을 무르게
// 만들지 않는다.
func TestDecisionAddFromNonHolderStaysBehindTheLease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worktree := executionActiveLifecycleRecord(t)

	command := "agent-harness issueops decision add --id " + record.ID +
		" --title 계약변경 --body 근거 --kind architecture" +
		" --host claude --session-id wrong-session --agent-id owner-agent --cwd " + worktree
	req := executionRequest(record, worktree, "claude", "wrong-session", command)
	req.AgentID = "owner-agent"

	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("비홀더의 결정 기록은 write lease 뒤에 남아야 한다: %+v", got)
	}
}

// actor 플래그를 빠뜨린 호출은 owner mutation으로 분류되지 않는다. 다른 owner
// mutation과 같은 규율이다.
func TestDecisionAddWithoutActorFlagsIsNotOwnerMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worktree := executionActiveLifecycleRecord(t)

	command := "agent-harness issueops decision add --id " + record.ID +
		" --title 계약변경 --body 근거 --kind architecture"
	req := executionRequest(record, worktree, "claude", "owner-session", command)
	req.AgentID = "owner-agent"

	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" {
		t.Fatalf("actor 플래그 없는 호출은 분류되지 않아 차단되어야 한다: %+v", got)
	}
	if got.Deny == nil || !strings.Contains(got.Deny.Reason, "unclassified") {
		t.Fatalf("차단 사유가 분류 실패를 지목해야 한다: %+v", got.Deny)
	}
}
