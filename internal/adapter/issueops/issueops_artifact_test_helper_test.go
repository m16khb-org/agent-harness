package issueops

import (
	issueopsartifactinbound "issueops/internal/adapter/inbound/issueopsartifact"
	issueopsartifactoutbound "issueops/internal/adapter/outbound/issueopsartifact"
	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
	issueopscontract "issueops/internal/contract/issueops"
)

func stageIssueOpsArtifactForTest(
	stateRoot string,
	id string,
	name string,
	content []byte,
) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsArtifactHandlersForTest().Stage(stateRoot, id, name, content)
}

func issueOpsArtifactHandlersForTest() issueopsartifactinbound.Handlers {
	service := issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{})
	return issueopsartifactinbound.NewHandlers(service)
}
