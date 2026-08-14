package hookcli

import (
	hookpromptiodeps "agent-harness/internal/adapter/hookprompt"
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 IssueOps reader를 설치한다.
func init() {
	IncrementIssueOpsSourceMisdirect = issueopsadapter.IncrementIssueOpsSourceMisdirect
	IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	ObserveNativeProcessAncestry = issueopsadapter.ObserveNativeProcessAncestry
	hookpromptiodeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	hookpromptiodeps.ScanReadableIssueOps = issueopsadapter.ScanReadableIssueOps
	hookpromptiodeps.ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	hookpromptiodeps.ReadIssueOps = issueopsadapter.ReadIssueOps
}
