package issueopsapp

import (
	"issueops/internal/adapter/inbound/issueopslease"
	issueopscore "issueops/internal/adapter/issueops"
)

// lease inbound 어댑터는 next-command 렌더링 구현을 알지 않는다. 어댑터를 아는
// 곳은 composition root 하나뿐이다.
func configureIssueOpsLeaseNextCommands() {
	issueopslease.ConfigureNextCommands(issueopslease.NextCommandDeps{
		ReseedNextCommand: issueopscore.ExecutionReseedNextCommand,
		ResumeNextCommand: issueopscore.ExecutionResumeNextCommand,
	})
}
