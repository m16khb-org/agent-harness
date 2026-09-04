package issueopsapp

import (
	installcliiodeps "issueops/cmd/issueops/installcli"
	issueopsadapter "issueops/internal/adapter/issueops"
	operationalhealthiodeps "issueops/internal/adapter/operationalhealth"
)

// configureIssueOpsReaders는 IssueOps 상태 조회와 native process 관측을 설치한다.
//
// install CLI와 operational health는 lifecycle을 읽기만 한다. 상태를 어디에 저장하고
// process를 어떻게 관측하는지는 composition root의 결정이다.
func configureIssueOpsReaders() {
	installcliiodeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	operationalhealthiodeps.InspectNativeProcessReceipt = issueopsadapter.InspectNativeProcessReceipt
	operationalhealthiodeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	operationalhealthiodeps.ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	operationalhealthiodeps.ListLeaseHolderIndexes = issueopsadapter.ListLeaseHolderIndexes
	operationalhealthiodeps.ReadIssueOpsExisting = issueopsadapter.ReadIssueOpsExisting
}
