package issueopscli

import (
	reviewfilesppdeps "agent-harness/cmd/harness/apidoc/reviewfiles"
	mcpclippdeps "agent-harness/cmd/harness/mcpcli"
	resourcesppdeps "agent-harness/cmd/harness/mcpcli/resources"
	auditppdeps "agent-harness/internal/adapter/audit"
	issueopsppdeps "agent-harness/internal/adapter/issueops"
	cleanupstatusppdeps "agent-harness/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "agent-harness/internal/adapter/issueops/implementation"
	orphancleanupppdeps "agent-harness/internal/adapter/issueops/orphancleanup"
	policyadapter "agent-harness/internal/adapter/policy"
	preflightadapter "agent-harness/internal/adapter/preflight"
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
