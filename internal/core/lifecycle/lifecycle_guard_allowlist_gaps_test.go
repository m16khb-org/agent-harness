package lifecycle

import (
	"strings"
	"testing"
)

// cleanup orphan의 대상은 정식 phase를 밟지 못한 사이클의 자원이다. 그런 사이클은
// 정의상 mutation authority가 활성인 채로 남으므로, 이 명령이 필요한 순간에
// 정확히 막혔다 — cleanup abandon이 그것을 안내하기까지 한다(이슈 #177).
//
// 통과는 "권한 승인"이 아니라 "core로 전달"이다. fingerprint와 --apply --confirm
// 게이트는 core가 본다(sync-base·switch-mode와 같은 계약).
func TestCleanupOrphanTypedControlPlaneIsAdmitted(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	base := "agent-harness issueops cleanup orphan --id " + record.ID +
		" --repo " + record.Repo + " --worktree " + worker + " --branch " + record.Branch +
		" --provider github --kind pr --artifact-url https://github.com/acme/repo/pull/1"

	for name, command := range map[string]string{
		"preview": base + " --json",
		"apply":   base + " --apply --confirm --fingerprint " + strings.Repeat("a", 64) + " --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("정리 명령이 막히면 그 사이클을 빠져나갈 수 없다: %+v", got)
			}
		})
	}
}

// spec 밖 플래그는 exact 파싱에서 떨어져 typed control plane으로 인정되지 않는다.
// 가드가 cleanup orphan 이름만으로 열리지 않았음을 증명한다.
func TestCleanupOrphanUnregisteredShapeStaysUnclassified(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	base := "agent-harness issueops cleanup orphan --id " + record.ID

	for name, command := range map[string]string{
		"unknown flag": base + " --force --json",
		"missing id":   "agent-harness issueops cleanup orphan --repo " + record.Repo + " --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if executionTypedControlPlane(req) {
				t.Fatalf("%s must not be admitted as a typed control-plane command", name)
			}
		})
	}
}

// #163이 정한 orca 순서는 orca 준비 뒤에 gh issue develop으로 링크를 붙이는
// 것이다. 그 시점에 lease는 활성이고 사용자는 canonical worktree에 있는데 그
// 명령이 막혔다. branch prepare의 fallback_api가 그것을 안내하므로 안내와 실행
// 가능성이 어긋나 있었다(이슈 #177).
func TestGHIssueDevelopIsAdmittedFromTheCanonicalWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"develop":      "gh issue develop 147 --repo acme/repo --base main --name 147-demo",
		"develop list": "gh issue develop --list 147 --repo acme/repo",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("branch prepare가 안내하는 명령이 막히면 그 안내가 거짓이 된다: %+v", got)
			}
		})
	}
}

// 임의 gh mutation은 계속 막힌다. develop 하나를 열면서 issue 표면 전체가
// 열리면 안 된다.
func TestOtherGHIssueMutationsStayBlocked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"create":        "gh issue create --repo acme/repo --title x --body y",
		"close":         "gh issue close 147 --repo acme/repo",
		"edit":          "gh issue edit 147 --repo acme/repo --body y",
		"comment":       "gh issue comment 147 --repo acme/repo --body y",
		"develop force": "gh issue develop 147 --repo acme/repo --base main --name 147-demo --force",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if executionObservation(req) {
				t.Fatalf("%s must not be admitted", name)
			}
		})
	}
}
