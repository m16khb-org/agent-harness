package issueopsapp

import (
	issueopsinventoryinbound "issueops/internal/adapter/inbound/issueopsinventory"
	issueopsinventoryoutbound "issueops/internal/adapter/outbound/issueopsinventory"
	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventoryapplication "issueops/internal/application/issueopsinventory"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func issueOpsInventoryListHandler(
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
) (issueopsinventorycontract.ListResult, error) {
	service := issueopsinventoryapplication.NewService(
		issueopsinventoryoutbound.Repository{
			Store: issueOpsRecordStore("inventory", observers...),
		},
		issueopsinventoryoutbound.SystemClock{},
		issueopsinventoryoutbound.CleanPath{},
	)
	return issueopsinventoryinbound.NewListHandler(service)
}
