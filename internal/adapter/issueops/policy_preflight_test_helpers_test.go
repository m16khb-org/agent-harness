package issueops

import (
	cleanupstatusppdeps "issueops/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "issueops/internal/adapter/issueops/implementation"
	preflightadapter "issueops/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	GitCmd = preflightadapter.GitCmd
	GitCmdRaw = preflightadapter.GitCmdRaw
	GitOut = preflightadapter.GitOut
	cleanupstatusppdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusppdeps.GitOut = preflightadapter.GitOut
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
}
