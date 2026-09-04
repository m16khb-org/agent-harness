package issueopscli

import (
	reviewfilesppdeps "issueops/cmd/issueops/apidoc/reviewfiles"
	mcpclippdeps "issueops/cmd/issueops/mcpcli"
	resourcesppdeps "issueops/cmd/issueops/mcpcli/resources"
	auditppdeps "issueops/internal/adapter/audit"
	issueopsppdeps "issueops/internal/adapter/issueops"
	cleanupstatusppdeps "issueops/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "issueops/internal/adapter/issueops/implementation"
	orphancleanupppdeps "issueops/internal/adapter/issueops/orphancleanup"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	auditppdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	cleanupstatusppdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusppdeps.GitOut = preflightadapter.GitOut
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitCmd = preflightadapter.GitCmd
	issueopsppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitOut = preflightadapter.GitOut
	mcpclippdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	mcpclippdeps.FakeRunCommand = policyadapter.FakeRunCommand
	orphancleanupppdeps.GitCmd = preflightadapter.GitCmd
	resourcesppdeps.CommandPolicySummary = policyadapter.CommandPolicySummary
	reviewfilesppdeps.GitCmd = preflightadapter.GitCmd
}
