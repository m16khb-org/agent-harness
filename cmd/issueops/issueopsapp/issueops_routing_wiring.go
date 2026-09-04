package issueopsapp

import (
	issueopsroutinginbound "issueops/internal/adapter/inbound/issueopsrouting"
	issueopsauthorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsroutingoutbound "issueops/internal/adapter/outbound/issueopsrouting"
	issueopsroutingapplication "issueops/internal/application/issueopsrouting"
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
