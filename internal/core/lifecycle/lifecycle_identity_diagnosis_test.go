package lifecycle

import (
	"strings"
	"testing"
)

// 이슈 #90 발견 4: active holder가 있는데 훅 관측 identity가 어긋나면 deny가
// 어떤 축(session_id/agent_id 등)이 다른지와 훅이 관측한 값을 에코해야 한다.
// 에코가 없으면 owner는 어떤 값을 고쳐야 하는지 알 수 없어 추측 재시도만 반복한다.
func TestExecutionActiveHolderMismatchEchoesObservedIdentity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	req := executionRequest(record, worker, "claude", "different-session", "")
	req.AgentID, req.Tool, req.Command = "", "Bash", "go test ./..."
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || got.Deny == nil {
		t.Fatalf("mismatched identity must be denied: %+v", got)
	}
	if got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("active-lease identity mismatch must carry its own code: %+v", got.Deny)
	}
	if got.Deny.IdentityMismatch == "" || !strings.Contains(got.Deny.ObservedActor, "different-session") {
		t.Fatalf("deny must echo the mismatch axis and the observed actor: %+v", got.Deny)
	}
}
