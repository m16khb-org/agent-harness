package issueopscli

import (
	"context"

	"agent-harness/internal/adapter/issueops/loopgate"
	"agent-harness/internal/adapter/issueops/orphancleanup"
	issueopscontract "agent-harness/internal/contract/issueops"
	orphancontract "agent-harness/internal/contract/issueopsorphancleanup"
)

// 프로덕션에서는 harnessapp이 주입한다. 이 계약 테스트는 실제 게이트 판정과
// 고아 정리를 검증하므로 같은 배선을 재현한다.
func wireOrphanAndLoopGateForTests() {
	ConfigureOrphanCleanup(OrphanCleanupDeps{
		Preview: func(ctx context.Context, req orphancontract.Request, deps OrphanDependencies) (orphancontract.Result, error) {
			return orphancleanup.Preview(ctx, req, orphancleanup.Dependencies{Collect: deps.Collect, VerifyMerged: deps.VerifyMerged})
		},
		Apply: func(ctx context.Context, req orphancontract.Request, apply orphancontract.ApplyRequest, deps OrphanDependencies) (orphancontract.Result, error) {
			return orphancleanup.Apply(ctx, req, apply, orphancleanup.Dependencies{Collect: deps.Collect, VerifyMerged: deps.VerifyMerged})
		},
	})
	ConfigureLoopGate(LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return loopgate.AdvancePhaseWithActor(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: loopgate.StrictPRReadinessWithState,
	})
}
