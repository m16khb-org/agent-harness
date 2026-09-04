package mcpcli

import (
	reviewfilesppdeps "issueops/cmd/issueops/apidoc/reviewfiles"
	resourcesppdeps "issueops/cmd/issueops/mcpcli/resources"
	auditppdeps "issueops/internal/adapter/audit"
	issueopsppdeps "issueops/internal/adapter/issueops"
	cleanupstatusppdeps "issueops/internal/adapter/issueops/cleanupstatus"
	implementationppdeps "issueops/internal/adapter/issueops/implementation"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
	workerppdeps "issueops/internal/adapter/worker"
)

// production wiring과 같은 실행기를 설치한다. 이 package가 실제로 의존하는
// 대상만 채운다.
func init() {
	EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	FakeRunCommand = policyadapter.FakeRunCommand
	auditppdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	cleanupstatusppdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusppdeps.GitOut = preflightadapter.GitOut
	implementationppdeps.GitCmd = preflightadapter.GitCmd
	implementationppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitCmd = preflightadapter.GitCmd
	issueopsppdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsppdeps.GitOut = preflightadapter.GitOut
	resourcesppdeps.CommandPolicySummary = policyadapter.CommandPolicySummary
	reviewfilesppdeps.GitCmd = preflightadapter.GitCmd
	workerppdeps.RunReadOnlyCommand = policyadapter.RunReadOnlyCommand
}
