package gatesgate

import (
	issueopsppdeps "issueops/internal/adapter/issueops"
	cleanupstatusppdeps "issueops/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "issueops/internal/adapter/issueops/implementation"
	preflightadapter "issueops/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다.
func init() {
	cleanupstatusppdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusppdeps.GitOut = preflightadapter.GitOut
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitCmd = preflightadapter.GitCmd
	issueopsppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitOut = preflightadapter.GitOut
}
