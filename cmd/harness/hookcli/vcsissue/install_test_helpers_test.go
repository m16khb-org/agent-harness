package vcsissue

import (
	hookclideps "agent-harness/cmd/harness/hookcli"
	hookpromptdeps "agent-harness/internal/adapter/hookprompt"
	installadapter "agent-harness/internal/adapter/install"
	issueopsadapter "agent-harness/internal/adapter/issueops"
)

// production wiring과 같은 install reader를 설치한다. 이 package의 테스트는
// hookcli의 pre-tool-use 경로를 거쳐 native runtime 진단에 닿는다.
func init() {
	hookclideps.DiagnoseNativeRuntime = installadapter.DiagnoseNativeRuntime
	hookclideps.NativeRuntimeDiagnosticMessage = installadapter.NativeRuntimeDiagnosticMessage
	hookclideps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	hookclideps.IncrementIssueOpsSourceMisdirect = issueopsadapter.IncrementIssueOpsSourceMisdirect
	hookclideps.ObserveNativeProcessAncestry = issueopsadapter.ObserveNativeProcessAncestry
	hookpromptdeps.IssueOpsStateRoot = issueopsadapter.IssueOpsStateRoot
	hookpromptdeps.ScanReadableIssueOps = issueopsadapter.ScanReadableIssueOps
	hookpromptdeps.ListIssueOpsIDs = issueopsadapter.ListIssueOpsIDs
	hookpromptdeps.ReadIssueOps = issueopsadapter.ReadIssueOps
}
