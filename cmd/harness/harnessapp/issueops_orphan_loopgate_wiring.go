package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/internal/adapter/issueops/gatesgate"
	"agent-harness/internal/adapter/issueops/orphancleanup"
	issueopscontract "agent-harness/internal/contract/issueops"
	orphancontract "agent-harness/internal/contract/issueopsorphancleanup"
	"context"
)

// issueops CLI는 고아 정리와 루프 게이트 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsOrphanAndLoopGate() {
	issueopscli.ConfigureOrphanCleanup(issueopscli.OrphanCleanupDeps{
		Preview: func(ctx context.Context, req orphancontract.Request, deps issueopscli.OrphanDependencies) (orphancontract.Result, error) {
			return orphancleanup.Preview(ctx, req, orphancleanup.Dependencies{Collect: deps.Collect, VerifyMerged: deps.VerifyMerged})
		},
		Apply: func(ctx context.Context, req orphancontract.Request, apply orphancontract.ApplyRequest, deps issueopscli.OrphanDependencies) (orphancontract.Result, error) {
			return orphancleanup.Apply(ctx, req, apply, orphancleanup.Dependencies{Collect: deps.Collect, VerifyMerged: deps.VerifyMerged})
		},
	})
	issueopscli.ConfigureLoopGate(issueopscli.LoopGateDeps{
		AdvancePhaseWithActor: func(stateRoot, id, to string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return gatesgate.AdvancePhaseWithActor(stateRoot, id, to, actor)
		},
		StrictPRReadinessWithState: gatesgate.StrictPRReadinessWithState,
	})
}
