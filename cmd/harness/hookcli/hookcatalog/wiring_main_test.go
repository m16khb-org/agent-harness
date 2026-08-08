package hookcatalog

import (
	"agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/hookprompt"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	issueopscontract "agent-harness/internal/contract/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 이 패키지 테스트는 실제 lifecycle
// 상태를 전제로 하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
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
	doctor.ConfigureLifecycle(lifecycle.ValidateProjectLifecycleState)
	os.Exit(m.Run())
}
