package loopgate

import (
	issueopsppdeps "agent-harness/internal/adapter/issueops"
	cleanupstatusppdeps "agent-harness/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "agent-harness/internal/adapter/issueops/implementation"
	preflightadapter "agent-harness/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	cleanupstatusppdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusppdeps.GitOut = preflightadapter.GitOut
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitCmd = preflightadapter.GitCmd
	issueopsppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitOut = preflightadapter.GitOut
}
