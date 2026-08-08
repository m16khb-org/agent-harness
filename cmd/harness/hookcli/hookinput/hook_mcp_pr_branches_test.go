package hookinput

import (
	"strings"
	"testing"
)

// TestMCPPullRequestCommandPreservesBaseAndHead는 #263을 고정한다. connector의
// `create_pull_request` 입력에서 base/head가 hook이 검사하는 명령 표현으로
// 보존되지 않아, 올바른 대상 branch를 전달해도 PR target guard가 base 누락으로
// 오탐 차단했다.
func TestMCPPullRequestCommandPreservesBaseAndHead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{
			"표준 필드",
			`{"tool_name":"github__create_pull_request","tool_input":{
				"repository":"m16khb/agent-harness","title":"t","draft":true,
				"base":"258-orca-owner-sealed-claim","head":"262-orca-plan-readiness"}}`,
		},
		{
			"호환 별칭",
			`{"tool_name":"github__create_pull_request","tool_input":{
				"repository":"m16khb/agent-harness","title":"t","draft":true,
				"base_branch":"258-orca-owner-sealed-claim","head_branch":"262-orca-plan-readiness"}}`,
		},
		{
			"camelCase 별칭",
			`{"tool_name":"github__create_pull_request","tool_input":{
				"repository":"m16khb/agent-harness","title":"t",
				"baseBranch":"258-orca-owner-sealed-claim","headBranch":"262-orca-plan-readiness"}}`,
		},
		{
			"GitLab source/target",
			`{"tool_name":"gitlab__create_merge_request","tool_input":{
				"title":"t","target_branch":"258-orca-owner-sealed-claim",
				"source_branch":"262-orca-plan-readiness"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := CommandFromHookInput([]byte(tc.input))
			base, head := "--base", "--head"
			if strings.HasPrefix(command, "glab ") {
				base, head = "--target-branch", "--source-branch"
			}
			if !strings.Contains(command, base+" ") || !strings.Contains(command, "258-orca-owner-sealed-claim") {
				t.Fatalf("base가 명령 표현에 보존돼야 한다: %q", command)
			}
			if !strings.Contains(command, head+" ") || !strings.Contains(command, "262-orca-plan-readiness") {
				t.Fatalf("head가 명령 표현에 보존돼야 한다: %q", command)
			}
		})
	}
}

// TestMCPIssueCommandDoesNotInventBranches는 branch 개념이 없는 issue 생성에
// base/head를 지어내지 않음을 고정한다.
func TestMCPIssueCommandDoesNotInventBranches(t *testing.T) {
	command := CommandFromHookInput([]byte(`{"tool_name":"github__create_issue","tool_input":{
		"repository":"m16khb/agent-harness","title":"t","body":"b"}}`))
	if strings.Contains(command, "--base") || strings.Contains(command, "--head") {
		t.Fatalf("issue 생성에 branch flag를 지어내면 안 된다: %q", command)
	}
}

// TestMCPPullRequestCommandOmitsAbsentBranches는 값이 없으면 빈 flag를 만들지
// 않음을 고정한다 — 빈 값은 guard가 누락으로 판정해야 하고, 그 판정은
// 여전히 유효해야 한다.
func TestMCPPullRequestCommandOmitsAbsentBranches(t *testing.T) {
	command := CommandFromHookInput([]byte(`{"tool_name":"github__create_pull_request","tool_input":{
		"repository":"m16khb/agent-harness","title":"t","head":"262-orca-plan-readiness"}}`))
	if strings.Contains(command, "--base") {
		t.Fatalf("없는 base를 빈 flag로 만들면 안 된다: %q", command)
	}
	if !strings.Contains(command, "--head ") || !strings.Contains(command, "262-orca-plan-readiness") {
		t.Fatalf("있는 head는 보존돼야 한다: %q", command)
	}
}
