package hookcli

import (
	issueopscontract "agent-harness/internal/contract/issueops"
)

// IssueOps 상태 조회와 native process 관측은 파일시스템·프로세스를 읽는다.
// 그 구현은 composition root가 설치한다.
var (
	IncrementIssueOpsSourceMisdirect func(stateRoot, id string) (int, error)
	IssueOpsStateRoot                func() string
	ObserveNativeProcessAncestry     func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
)
