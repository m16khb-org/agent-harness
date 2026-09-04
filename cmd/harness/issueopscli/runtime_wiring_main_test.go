package issueopscli

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"agent-harness/cmd/harness/issueopscli/executioncmd"
	issueopsartifactinbound "agent-harness/internal/adapter/inbound/issueopsartifact"
	issueopsdecisioninbound "agent-harness/internal/adapter/inbound/issueopsdecision"
	issueopsinventoryinbound "agent-harness/internal/adapter/inbound/issueopsinventory"
	issueopsretentioninbound "agent-harness/internal/adapter/inbound/issueopsretention"
	issueopsroutinginbound "agent-harness/internal/adapter/inbound/issueopsrouting"
	issueopsstatusinbound "agent-harness/internal/adapter/inbound/issueopsstatus"
	issueopscore "agent-harness/internal/adapter/issueops"
	issueopsartifactoutbound "agent-harness/internal/adapter/outbound/issueopsartifact"
	issueopsauthorizationoutbound "agent-harness/internal/adapter/outbound/issueopsauthorization"
	issueopsdecisionoutbound "agent-harness/internal/adapter/outbound/issueopsdecision"
	issueopsinventoryoutbound "agent-harness/internal/adapter/outbound/issueopsinventory"
	issueopsretentionoutbound "agent-harness/internal/adapter/outbound/issueopsretention"
	issueopsroutingoutbound "agent-harness/internal/adapter/outbound/issueopsrouting"
	issueopsstatusoutbound "agent-harness/internal/adapter/outbound/issueopsstatus"
	issueopsartifactapplication "agent-harness/internal/application/issueopsartifact"
	issueopsdecisionapplication "agent-harness/internal/application/issueopsdecision"
	issueopsinventoryapplication "agent-harness/internal/application/issueopsinventory"
	issueopsretentionapplication "agent-harness/internal/application/issueopsretention"
	issueopsroutingapplication "agent-harness/internal/application/issueopsrouting"
	issueopsstatusapplication "agent-harness/internal/application/issueopsstatus"
	issueopsstatusdomain "agent-harness/internal/domain/issueopsstatus"
	"agent-harness/internal/port"

	issueopsnextinbound "agent-harness/internal/adapter/inbound/issueopsnext"
	issueopsnextapplication "agent-harness/internal/application/issueopsnext"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

// 프로덕션에서는 harnessapp이 주입한다. IssueOps CLI 계약 테스트는 실제 사이클
// 저장소를 검증하므로 같은 배선을 재현한다.
func wireIssueOpsRuntimeForTests() {
	artifacts := issueopsartifactinbound.NewHandlers(
		issueopsartifactapplication.NewService(issueopsartifactoutbound.Repository{}),
	)
	decisions := issueopsdecisioninbound.NewHandlers(issueopsdecisionapplication.NewService(
		issueopsdecisionoutbound.Repository{},
		issueopsdecisionoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	))
	inventory := issueopsinventoryapplication.NewService(
		issueopsinventoryoutbound.Repository{},
		issueopsinventoryoutbound.SystemClock{},
		issueopsinventoryoutbound.CleanPath{},
	)
	retention := issueopsretentionapplication.NewService(
		issueopsretentionoutbound.Repository{},
		issueopsretentionoutbound.SystemClock{},
	)
	status := issueopsstatusapplication.NewService(
		issueopsstatusoutbound.Repository{},
		issueopsstatusdomain.NewProjector(issueopscore.IssueOpsPhaseCompletion),
	)
	routing := issueopsroutinginbound.NewHandlers(issueopsroutingapplication.NewService(
		issueopsroutingoutbound.Repository{},
		issueopsroutingoutbound.SystemClock{},
		issueopsauthorizationoutbound.CanonicalPaths{},
	))
	listCycles := issueopsinventoryinbound.NewListHandler(inventory)
	next := issueopsnextapplication.NewService(issueopsnextapplication.Ports{
		ListCycles: func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
			return listCycles(stateRoot, repo)
		},
		ReadRecord:        issueopscore.ReadIssueOps,
		Completion:        issueopscore.IssueOpsPhaseCompletion,
		LocalReadiness:    issueopscore.IssueOpsLocalPRReadiness,
		WriterlessCommand: issueopscore.ExecutionWriterAbsentRecoveryCommand,
		PlannerDefaults:   port.IssueOpsPlannerDefaults,
		StagedArtifacts:   artifacts.Names,
		Actor: func() (string, string, error) {
			host, sessionID, _, err := executioncmd.ResolveNativeSessionIdentity(os.Getenv)
			return host, sessionID, err
		},
		SourceRoot: issueopsinventoryoutbound.CleanPath{}.Normalize,
		CleanPath:  filepath.Clean,
		Env:        os.Getenv,
		Now:        time.Now,
	})
	ConfigureIssueOpsRuntime2(IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor:                issueopscore.AcceptIssueOpsChildWithActor,
		AddIssueOpsDecisionWithActor:                decisions.AddWithActor,
		DropIssueOpsChildWithActor:                  issueopscore.DropIssueOpsChildWithActor,
		IssueOpsChildStatusWithActor:                issueopscore.IssueOpsChildStatusWithActor,
		IssueOpsPRReadiness:                         issueopscore.IssueOpsPRReadiness,
		IssueOpsNext:                                issueopsnextinbound.NewNextHandler(next),
		IssueOpsStateRoot:                           issueopscore.IssueOpsStateRoot,
		IssueOpsStatus:                              issueopsstatusinbound.NewStatusHandler(status),
		LinkIssueOpsChildWithActor:                  issueopscore.LinkIssueOpsChildWithActor,
		LinkIssueOpsIssueWithActor:                  issueopscore.LinkIssueOpsIssueWithActor,
		LinkIssueOpsPlanWithActor:                   issueopscore.LinkIssueOpsPlanWithActor,
		LinkIssueOpsRelatedWithActor:                issueopscore.LinkIssueOpsRelatedWithActor,
		LinkIssueOpsWorktreeWithActor:               issueopscore.LinkIssueOpsWorktreeWithActor,
		ListIssueOpsCycles:                          listCycles,
		ObserveNativeProcessAncestry:                issueopscore.ObserveNativeProcessAncestry,
		PrepareIssueOpsBranchWithActor:              issueopscore.PrepareIssueOpsBranchWithActor,
		PruneIssueOps:                               issueopsretentioninbound.NewPruneHandler(retention),
		ReadIssueOps:                                issueopscore.ReadIssueOps,
		RecordIssueOpsAISlopCleanEvidenceWithActor:  issueopscore.RecordIssueOpsAISlopCleanEvidenceWithActor,
		RecordIssueOpsCompatibilityReviewWithActor:  issueopscore.RecordIssueOpsCompatibilityReviewWithActor,
		RecordIssueOpsDesignReviewWithActor:         issueopscore.RecordIssueOpsDesignReviewWithActor,
		RecordIssueOpsDevilsAdvocateReviewWithActor: issueopscore.RecordIssueOpsDevilsAdvocateReviewWithActor,
		RecordIssueOpsDomainReviewWithActor:         issueopscore.RecordIssueOpsDomainReviewWithActor,
		RecordIssueOpsImplementationReview:          issueopscore.RecordIssueOpsImplementationReview,
		RecordIssueOpsProjectDocsReview:             issueopscore.RecordIssueOpsProjectDocsReview,
		RecordIssueOpsSchemaEvidence:                issueopscore.RecordIssueOpsSchemaEvidence,
		RecordIssueOpsIntentWithActor:               issueopscore.RecordIssueOpsIntentWithActor,
		RecordIssueOpsPlanPrepWithActor:             issueopscore.RecordIssueOpsPlanPrepWithActor,
		RecordIssueOpsRoutingWithActor:              routing.Record,
		RegressIssueOpsForReplanWithActor:           issueopscore.RegressIssueOpsForReplanWithActor,
		RejectIssueOpsChildWithActor:                issueopscore.RejectIssueOpsChildWithActor,
		ResolveIssueOpsFeedbackWithActor:            issueopscore.ResolveIssueOpsFeedbackWithActor,
		ScoreLiveRoutingFidelity:                    routing.Score,
		StageIssueOpsArtifact:                       artifacts.Stage,
		StagedIssueOpsArtifactNames:                 artifacts.Names,
		StartIssueOps:                               issueopscore.StartIssueOps,
		StartIssueOpsChildWithActor:                 issueopscore.StartIssueOpsChildWithActor,
		UnstageIssueOpsArtifact:                     artifacts.Unstage,
	})
}
