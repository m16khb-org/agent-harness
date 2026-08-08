package operationalhealth

import (
	issueopscontract "agent-harness/internal/contract/issueops"
)

// IssueOps 상태 조회와 native process 관측은 파일시스템·프로세스를 읽는다.
// 그 구현은 composition root가 설치한다.
var (
	InspectNativeProcessReceipt func(receipt issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error)
	IssueOpsStateRoot           func() string
	ListIssueOpsIDs             func(stateRoot string) ([]string, error)
	ListLeaseHolderIndexes      func(stateRoot string) ([]issueopscontract.LeaseHolderIndex, error)
	ReadIssueOpsExisting        func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
)
