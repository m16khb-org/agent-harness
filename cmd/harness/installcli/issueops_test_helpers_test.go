package installcli

import (
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 IssueOps reader를 설치한다.
func init() {
	IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
}
