package issueops

import (
	issueopsartifactinbound "agent-harness/internal/adapter/inbound/issueopsartifact"
	issueopsartifactoutbound "agent-harness/internal/adapter/outbound/issueopsartifact"
	issueopsartifactapplication "agent-harness/internal/application/issueopsartifact"
	issueopscontract "agent-harness/internal/contract/issueops"
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
