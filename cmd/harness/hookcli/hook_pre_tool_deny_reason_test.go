package hookcli

import (
	"strings"
	"testing"

	"agent-harness/internal/core"
	lifecyclemodel "agent-harness/internal/core/lifecycle/model"
)

// hookDenyReason이 host hook으로 내보내는 문자열이 사용자가 실제로 보는 전부다.
// Deny가 있으면 result.Reason을 버리고 JSON만 내보내므로, 사유가 그 JSON 안에
// 실려 있지 않으면 사용자에게는 코드밖에 남지 않는다(이슈 #154).
func TestHookDenyReasonCarriesTheBlockingCause(t *testing.T) {
	const cause = "unclassified shell command is blocked while IssueOps mutation authority is active"
	encoded := hookDenyReason(core.HookPreToolUseDecisionResult{
		Decision: "block",
		Reason:   cause,
		Deny: &lifecyclemodel.IssueOpsDenyReason{
			Code: "unsafe_mutation", LifecycleID: "io-test", ExpectedRoot: "/tmp/wt",
			CurrentGeneration: 1, NextCommand: "agent-harness issueops execution status --id io-test --json",
			Reason: cause,
		},
	})

	if !strings.Contains(encoded, cause) {
		t.Fatalf("차단 사유가 host hook 출력에 실려야 한다: %s", encoded)
	}
	// 코드와 next_command는 기존 소비자의 계약이므로 그대로 남는다.
	for _, want := range []string{"unsafe_mutation", "execution status"} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("기존 필드 %q가 사라졌다: %s", want, encoded)
		}
	}
}

// Deny에 사유가 없는 경로에서도 출력은 여전히 유효한 JSON이어야 하고, 상위
// result.Reason으로 물러나지 않는다 — 구조화된 소비자가 파싱에 실패하면 안 된다.
func TestHookDenyReasonStaysStructuredWithoutAReason(t *testing.T) {
	encoded := hookDenyReason(core.HookPreToolUseDecisionResult{
		Decision: "block",
		Reason:   "some cause",
		Deny: &lifecyclemodel.IssueOpsDenyReason{
			Code: "holder_identity_mismatch", LifecycleID: "io-test",
		},
	})

	if !strings.HasPrefix(strings.TrimSpace(encoded), "{") {
		t.Fatalf("구조화된 deny는 JSON으로 남아야 한다: %s", encoded)
	}
	if !strings.Contains(encoded, "holder_identity_mismatch") {
		t.Fatalf("코드가 사라졌다: %s", encoded)
	}
}
