package hookcli

import (
	"agent-harness/cmd/harness/hookcli/hookenv"
	"agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/hookprompt"
	issueopscore "agent-harness/internal/adapter/issueops"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	issueopscontract "agent-harness/internal/contract/issueops"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 훅 계약 테스트는 실제 lifecycle 동작을
// 검증하므로 같은 배선을 여기서 재현한다.
func TestMain(m *testing.M) {
	// 상속된 운영자 스위치를 먼저 지운다. 이 패키지의 테스트는 hook enforcement가
	// 켜져 있음을 전제하는데, dogfood 셸의 HARNESS_DISABLE_HOOKS=1이 그대로 새어
	// 들어오면 4개 테스트가 항상 실패한다(#395).
	hookenv.ClearInheritedOperatorSwitches()
	ConfigureLifecycle(LifecycleDeps{
		RecordLifecycleToolUse:            lifecycle.RecordLifecycleToolUse,
		SourceCheckoutMisdirectWarning:    lifecycle.SourceCheckoutMisdirectWarning,
		BuildLifecyclePreCompactCapsule:   lifecycle.BuildLifecyclePreCompactCapsule,
		BuildLifecycleStopReminder:        lifecycle.BuildLifecycleStopReminder,
		BuildLifecyclePreToolUseDecision:  lifecycle.BuildLifecyclePreToolUseDecision,
		RecordStopNextActionRelay:         lifecycle.RecordStopNextActionRelay,
		ClearStopNextActionRelay:          lifecycle.ClearStopNextActionRelay,
		BuildLifecyclePostCompactReminder: lifecycle.BuildLifecyclePostCompactReminder,
	})
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
	lifecycle.ConfigureIssueOps(lifecycle.IssueOpsDeps{
		ActiveIssueOpsCycleForBranch:              issueopscore.ActiveIssueOpsCycleForBranch,
		ActiveIssueOpsLinkedWorktreeCyclesForRepo: issueopscore.ActiveIssueOpsLinkedWorktreeCyclesForRepo,
		AdvanceIssueOpsPhase:                      issueopscore.AdvanceIssueOpsPhase,
		IssueOpsPhaseExpectsWorktree:              issueopscore.IssueOpsPhaseExpectsWorktree,
		IssueOpsStateRoot:                         issueopscore.IssueOpsStateRoot,
		LinkIssueOpsIssue:                         issueopscore.LinkIssueOpsIssue,
		LinkIssueOpsPlan:                          issueopscore.LinkIssueOpsPlan,
		LinkIssueOpsWorktree:                      issueopscore.LinkIssueOpsWorktree,
		ListIssueOpsIDs:                           issueopscore.ListIssueOpsIDs,
		ScanIssueOps:                              issueopscore.ScanIssueOps,
		NewIssueOpsID:                             issueopscore.NewIssueOpsID,
		PrepareIssueOpsBranch:                     issueopscore.PrepareIssueOpsBranch,
		ReadIssueOps:                              issueopscore.ReadIssueOps,
		RecordIssueOpsDesignReview:                issueopscore.RecordIssueOpsDesignReview,
		RecordIssueOpsIntent:                      issueopscore.RecordIssueOpsIntent,
		SealedOwnerContextPacketPath:              issueopscore.SealedOwnerContextPacketPath,
		StartIssueOps:                             issueopscore.StartIssueOps,
		WriteIssueOps:                             issueopscore.WriteIssueOps,
	})
	os.Exit(m.Run())
}
