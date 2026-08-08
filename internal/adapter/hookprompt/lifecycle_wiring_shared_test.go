package hookprompt

import (
	lifecycle "agent-harness/internal/adapter/lifecycle"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// harnessapp의 배선과 같은 조합이다. 훅 프롬프트 계약 테스트는 실제 lifecycle
// 동작을 검증하므로 여기서 재현한다.
func realLifecycleDeps() LifecycleDeps {
	return LifecycleDeps{
		ResolveProjectLifecycleState: lifecycle.ResolveProjectLifecycleState,
		ReadPendingDocUpkeepEvents:   lifecycle.ReadPendingDocUpkeepEvents,
		ReadStopNextActionRelay:      lifecycle.ReadStopNextActionRelay,
		ApproveCodexKubectlLiveAccess: func(repo, host, sessionID, prompt string) (bool, string) {
			result := lifecycle.ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt)
			return result.Handled, result.AdditionalContext
		},
		ActiveIssueOpsLinkedWorktreeCyclesForRepo: func(repo string) []issueopscontract.IssueOpsRecord {
			return lifecycle.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
		},
		IssueOpsPhaseExpectsWorktree: lifecycle.IssueOpsPhaseExpectsWorktree,
	}
}
