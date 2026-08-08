package hookcatalog

import (
	hookpromptiodeps "agent-harness/internal/adapter/hookprompt"
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 IssueOps reader를 설치한다.
func init() {
	hookpromptiodeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	hookpromptiodeps.ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	hookpromptiodeps.ReadIssueOps = issueopsadapter.ReadIssueOps
}
