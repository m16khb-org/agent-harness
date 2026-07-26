package lifecycle

import (
	"strings"
	"testing"
)

// 전환은 lease가 writer를 쥐고 있지 않을 때만 성립하지만, 다른 lifecycle의
// mutation authority가 활성이면 훅이 이 명령을 unclassified로 막는다. typed
// 등록이 없으면 사용자가 갇힌다 — #158에서 decision add가 그랬다(이슈 #167).
//
// 통과는 "권한 승인"이 아니라 "core로 전달"이다. lease 게이트는 core가 본다.
func TestExecutionSwitchModeTypedControlPlaneAdmitsPreviewAndApply(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actorFlags := " --host claude --session-id owner-session --agent-id owner-agent" +
		" --session-pid 1234 --session-started-at 2026-07-22T00:00:00Z --session-executable claude" +
		" --cwd " + worker + " --json"
	base := "agent-harness issueops execution switch-mode --id " + record.ID + " --mode orca"

	for name, command := range map[string]string{
		"preview": base + actorFlags,
		"apply":   base + " --apply --confirm --fingerprint " + strings.Repeat("a", 64) + actorFlags,
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("typed switch-mode %s must reach core from the canonical worktree: %+v", name, got)
			}
		})
	}

	// spec에 없는 플래그는 exact 파싱에서 떨어져 typed control plane으로 인정되지
	// 않는다 — 가드가 switch-mode 이름만으로 열리지 않았음을 증명한다.
	for name, command := range map[string]string{
		"unregistered flag": base + " --force" + actorFlags,
		"missing id":        "agent-harness issueops execution switch-mode --mode orca" + actorFlags,
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if executionTypedControlPlane(req) {
				t.Fatalf("%s must not be admitted as a typed control-plane command", name)
			}
		})
	}

	// 비-holder 세션이 spec 밖 플래그로 던지면 typed 우회가 없으니 훅이 막는다.
	foreign := executionRequest(record, worker, "claude", "wrong-session", base+" --force"+actorFlags)
	foreign.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" {
		t.Fatalf("non-holder unregistered switch-mode must stay blocked: %+v", got)
	}
}
