package issueopscli

import (
	hookprompticodeps "agent-harness/internal/adapter/hookprompt"
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 IssueOps reader를 설치한다.
func init() {
	hookprompticodeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	hookprompticodeps.ScanReadableIssueOps = issueopsadapter.ScanReadableIssueOps
	hookprompticodeps.ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	hookprompticodeps.ReadIssueOps = issueopsadapter.ReadIssueOps
}
