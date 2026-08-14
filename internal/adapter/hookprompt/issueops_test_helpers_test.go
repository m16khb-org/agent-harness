package hookprompt

import (
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 IssueOps reader를 설치한다.
func init() {
	IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	ScanReadableIssueOps = issueopsadapter.ScanReadableIssueOps
	ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	ReadIssueOps = issueopsadapter.ReadIssueOps
}
