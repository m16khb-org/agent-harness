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

// 이슈 #90 도그푸드: handoff 시점에 branch_link_verified가 비어 있으면
// link-plan gate가 막히는데, active lease에서는 holder만 레코드를 고칠 수
// 있으므로 branch prepare도 4-flag owner mutation으로 admit되어야 한다.
func TestExactIssueOpsOwnerMutationAdmitsBranchPrepare(t *testing.T) {
	command := "agent-harness issueops branch prepare --id io-000000000089 --provider github" +
		" --issue-url 'https://github.com/acme/repo/issues/89' --branch 89-atomic --base-branch main" +
		" --base-sha 635303af758fae465d6e6fe30302fed9233180c5 --link-verified" +
		" --host codex --session-id sess-1 --cwd /tmp/wt --json"
	if !exactIssueOpsOwnerMutation(command) {
		t.Fatalf("well-formed branch prepare must pass the owner allowlist: %s", command)
	}
	if exactIssueOpsOwnerMutation(strings.Replace(command, "--session-id sess-1 ", "", 1)) {
		t.Fatal("branch prepare without session-id must fail the 4-flag signature")
	}
}
