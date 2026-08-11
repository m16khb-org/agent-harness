package harnessapp

import (
	issueopsinventoryinbound "agent-harness/internal/adapter/inbound/issueopsinventory"
	issueopsinventoryoutbound "agent-harness/internal/adapter/outbound/issueopsinventory"
	"agent-harness/internal/adapter/outbound/issueopsrecord"
	issueopsinventoryapplication "agent-harness/internal/application/issueopsinventory"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
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
