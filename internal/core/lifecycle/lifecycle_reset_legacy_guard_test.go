package lifecycle

import "testing"

// reset-legacy --preview와 --status는 schema 상태를 읽기만 한다. 그런데 가드의
// 세 목록 어디에도 없어서 mutation authority가 활성인 동안 unclassified로
// 막혔다 — 상태를 진단할 수단이 하나 사라져 있었다(이슈 #170).
func TestResetLegacyObservationIsAdmittedWhileAuthorityIsActive(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"preview": "agent-harness issueops reset-legacy --target-schema 1 --preview --json",
		"status":  "agent-harness issueops reset-legacy --target-schema 1 --status --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("진단 명령이 막히면 갇힌 상태를 볼 수단이 없다: %+v", got)
			}
		})
	}
}

// mutation 경로는 열지 않는다. schema v0 사이클을 다루는 마이그레이션 조작이고
// v1 lease가 갇힌 상태를 풀지 못하므로, authority 활성 중 통과시킬 이유가 없다.
func TestResetLegacyMutationStaysUnclassified(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"confirm":      "agent-harness issueops reset-legacy --target-schema 1 --confirm --json",
		"drain":        "agent-harness issueops reset-legacy --target-schema 1 --drain-cycle --id io-1 --confirm --json",
		"reconcile":    "agent-harness issueops reset-legacy --target-schema 1 --reconcile-remote --id io-1 --claim-id c1 --confirm --json",
		"both modes":   "agent-harness issueops reset-legacy --target-schema 1 --preview --status --json",
		"no schema":    "agent-harness issueops reset-legacy --preview --json",
		"unknown flag": "agent-harness issueops reset-legacy --target-schema 1 --preview --force --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if executionObservation(req) {
				t.Fatalf("%s must not be admitted as a read-only observation", name)
			}
		})
	}
}
