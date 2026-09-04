package issueopsapp

import (
	issueopsartifactinbound "issueops/internal/adapter/inbound/issueopsartifact"
	issueopsartifactoutbound "issueops/internal/adapter/outbound/issueopsartifact"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsartifactapplication "issueops/internal/application/issueopsartifact"
	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
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
