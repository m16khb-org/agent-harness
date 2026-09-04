package issueopsapp

import (
	reviewfilesdeps "issueops/cmd/issueops/apidoc/reviewfiles"
	mcpclideps "issueops/cmd/issueops/mcpcli"
	resourcesdeps "issueops/cmd/issueops/mcpcli/resources"
	policyclideps "issueops/cmd/issueops/policycli"
	statusclideps "issueops/cmd/issueops/statuscli"
	preflightfuzzdeps "issueops/cmd/issueops/validationcli/preflightfuzz"
	auditdeps "issueops/internal/adapter/audit"
	gatesdeps "issueops/internal/adapter/gates"
	gitworktreedeps "issueops/internal/adapter/gitworktree"
	issueopsdeps "issueops/internal/adapter/issueops"
	cleanupstatusdeps "issueops/internal/adapter/issueops/cleanupstatus"
	implementationdeps "issueops/internal/adapter/issueops/implementation"
	orphancleanupdeps "issueops/internal/adapter/issueops/orphancleanup"
	policyadapter "issueops/internal/adapter/policy"
	preflightadapter "issueops/internal/adapter/preflight"
	workerdeps "issueops/internal/adapter/worker"
)

// configurePolicyAndGitObservers는 명령 정책 평가·실행과 git 관측을 설치한다.
//
// 두 기능 모두 프로세스를 띄운다. 어떤 실행기를 쓸지는 composition root의
// 결정이고, 소비자는 요청과 결과 형식만 안다.
func configurePolicyAndGitObservers() {
	auditdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	policyadapter.PreparedBaseBranchLookup = issueopsdeps.PreparedBaseBranchForWorkspace
	cleanupstatusdeps.GitCmd = preflightadapter.GitCmd
	cleanupstatusdeps.GitOut = preflightadapter.GitOut
	gatesdeps.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	gatesdeps.RunCommand = policyadapter.RunCommand
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
