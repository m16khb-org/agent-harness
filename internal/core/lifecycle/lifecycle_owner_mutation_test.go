package lifecycle

import (
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
