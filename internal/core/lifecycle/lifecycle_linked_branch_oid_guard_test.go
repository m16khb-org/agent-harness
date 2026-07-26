package lifecycle

import "testing"

// GraphQL 변수(`$issueId` 등)는 셸에서 단일 인용해야 한다. 인용하지 않으면
// HasActiveParameterOrTildeExpansion이 파라미터 확장으로 판정한다(tokens.go의
// `$` 검사). 단일 인용 안의 문자는 그 검사가 건너뛰므로 통과한다 — 실행 주체가
// 인용을 붙이는 것이 전제이고, branch prepare의 Description이 그것을 안내한다.
const linkedBranchMutation = "'query=mutation($issueId:ID!,$oid:GitObjectID!,$name:String!)" +
	"{createLinkedBranch(input:{issueId:$issueId,oid:$oid,name:$name}){linkedBranch{ref{name target{oid}}}}}'"

// #176이 도입한 base-pinned 링크 경로는 두 명령으로 이뤄진다. branch prepare가
// 그것을 안내하므로 가드도 분류해야 한다 — 그러지 않으면 #177이 고친 것과 같은
// "안내와 실행 가능성의 어긋남"이 재발한다.
func TestLinkedBranchOIDPathIsAdmitted(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"node id read": "gh api repos/acme/repo/issues/176 --jq .node_id",
		"mutation": "gh api graphql -f " + linkedBranchMutation +
			" -F issueId=I_kwDOabc -F oid=2a56f2cc4d2e6b7b4fa99e3cdd71e3673ae060d2 -F name=176-demo",
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

// 임의 GraphQL과 임의 gh api는 계속 막힌다. createLinkedBranch 하나를 열면서
// API 표면 전체가 열려서는 안 된다.
func TestOtherGHAPICallsStayBlocked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"arbitrary graphql":  "gh api graphql -f query=mutation{deleteIssue(input:{issueId:x}){clientMutationId}} -F a=1 -F b=2 -F c=3",
		"repo delete":        "gh api -X DELETE repos/acme/repo",
		"ref delete":         "gh api -X DELETE repos/acme/repo/git/refs/heads/176-demo",
		"issue body read":    "gh api repos/acme/repo/issues/176 --jq .body",
		"node id wrong path": "gh api repos/acme/repo/pulls/176 --jq .node_id",
		"mutation extra flag": "gh api graphql -f " + linkedBranchMutation +
			" -F issueId=I_kwDOabc -F oid=2a56f2cc -F name=176-demo -F extra=1",
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
