package harnessapp

import (
	reviewfilesdeps "agent-harness/cmd/harness/apidoc/reviewfiles"
	mcpclideps "agent-harness/cmd/harness/mcpcli"
	resourcesdeps "agent-harness/cmd/harness/mcpcli/resources"
	policyclideps "agent-harness/cmd/harness/policycli"
	statusclideps "agent-harness/cmd/harness/statuscli"
	preflightfuzzdeps "agent-harness/cmd/harness/validationcli/preflightfuzz"
	auditdeps "agent-harness/internal/adapter/audit"
	gitworktreedeps "agent-harness/internal/adapter/gitworktree"
	issueopsdeps "agent-harness/internal/adapter/issueops"
	cleanupstatusdeps "agent-harness/internal/adapter/issueops/cleanupstatus"
	implementationdeps "agent-harness/internal/adapter/issueops/implementation"
	orphancleanupdeps "agent-harness/internal/adapter/issueops/orphancleanup"
	policyadapter "agent-harness/internal/adapter/policy"
	preflightadapter "agent-harness/internal/adapter/preflight"
	workerdeps "agent-harness/internal/adapter/worker"
)

// configurePolicyAndGitObservers는 명령 정책 평가·실행과 git 관측을 설치한다.
//
// 두 기능 모두 프로세스를 띄운다. 어떤 실행기를 쓸지는 composition root의
// 결정이고, 소비자는 요청과 결과 형식만 안다.
func configurePolicyAndGitObservers() {
	auditdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	cleanupstatusdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusdeps.GitOut = preflightadapter.GitOut
	gitworktreedeps.GitCmd = preflightadapter.GitCmd
	gitworktreedeps.GitOut = preflightadapter.GitOut
	implementationdeps.GitCmd = preflightadapter.GitCmd
	implementationdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsdeps.GitCmd = preflightadapter.GitCmd
	issueopsdeps.GitCmdRaw = preflightadapter.GitCmdRaw
	issueopsdeps.GitOut = preflightadapter.GitOut
	mcpclideps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	mcpclideps.FakeRunCommand = policyadapter.FakeRunCommand
	orphancleanupdeps.GitCmd = preflightadapter.GitCmd
	policyclideps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	policyclideps.FakeRunCommand = policyadapter.FakeRunCommand
	policyclideps.RunReadOnlyCommand = policyadapter.RunReadOnlyCommand
	preflightfuzzdeps.GitCmd = preflightadapter.GitCmd
	resourcesdeps.CommandPolicySummary = policyadapter.CommandPolicySummary
	reviewfilesdeps.GitCmd = preflightadapter.GitCmd
	statusclideps.RunReadOnlyCommand = policyadapter.RunReadOnlyCommand
	workerdeps.RunReadOnlyCommand = policyadapter.RunReadOnlyCommand
}
