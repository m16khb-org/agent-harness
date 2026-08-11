package harnessapp

import (
	issueopsroutinginbound "agent-harness/internal/adapter/inbound/issueopsrouting"
	issueopsauthorizationoutbound "agent-harness/internal/adapter/outbound/issueopsauthorization"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsroutingoutbound "agent-harness/internal/adapter/outbound/issueopsrouting"
	issueopsroutingapplication "agent-harness/internal/application/issueopsrouting"
)

func issueOpsRoutingHandlers(
	observers ...issueopsrecord.Observer,
) issueopsroutinginbound.Handlers {
	service := issueopsroutingapplication.NewService(
		issueopsroutingoutbound.Repository{
			Store: issueOpsRecordStore("routing", observers...),
		},
		issueopsroutingoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	)
	return issueopsroutinginbound.NewHandlers(service)
}
