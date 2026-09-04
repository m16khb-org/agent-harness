package issueopsapp

import (
	issueopsdecisioninbound "issueops/internal/adapter/inbound/issueopsdecision"
	issueopsauthorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "issueops/internal/adapter/outbound/issueopsdecision"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsdecisionapplication "issueops/internal/application/issueopsdecision"
)

func issueOpsDecisionHandlers(
	observers ...issueopsrecord.Observer,
) issueopsdecisioninbound.Handlers {
	service := issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{
			Store: issueOpsRecordStore("decision", observers...),
		},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	)
	return issueopsdecisioninbound.NewHandlers(service)
}
