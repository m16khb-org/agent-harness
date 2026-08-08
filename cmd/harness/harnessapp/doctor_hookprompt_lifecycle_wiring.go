package harnessapp

import (
	"agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/hookprompt"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// doctor와 hookprompt는 lifecycle 어댑터를 직접 알지 않는다. 실제 구현을 아는
// 곳은 composition root 하나뿐이다.
func configureDoctorHookPromptLifecycle() {
	doctor.ConfigureLifecycle(lifecycle.ValidateProjectLifecycleState)
	hookprompt.ConfigureLifecycle(hookprompt.LifecycleDeps{
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
	})
}
