package harnessapp

import (
	issueopsartifactinbound "agent-harness/internal/adapter/inbound/issueopsartifact"
	issueopsartifactoutbound "agent-harness/internal/adapter/outbound/issueopsartifact"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsartifactapplication "agent-harness/internal/application/issueopsartifact"
	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"
)

func issueOpsArtifactHandlers(
	observers ...issueopsrecord.Observer,
) issueopsartifactinbound.Handlers {
	service := issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{
		Store: issueOpsRecordStore("artifact", observers...),
	})
	return issueopsartifactinbound.NewHandlers(service)
}

func stageIssueOpsArtifact(
	stateRoot string,
	id string,
	name string,
	content []byte,
) (issueopsartifactcontract.Record, error) {
	return issueOpsArtifactHandlers().Stage(stateRoot, id, name, content)
}
