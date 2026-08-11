package issueopscli

import (
	"context"
	"errors"
	"time"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"
	issueopsdecisioncontract "agent-harness/internal/contract/issueopsdecision"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
	issueopsretentioncontract "agent-harness/internal/contract/issueopsretention"
	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
	issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"
)

var errIssueOpsCLINotConfigured = errors.New("issueops runtime is not configured")

// IssueOps 사이클 연산은 상태 저장소를 다루는 I/O다. CLI는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var issueOpsCLIDeps = neutralIssueOpsCLIDeps()

// IssueOpsCLIDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type IssueOpsCLIDeps struct {
	AcceptIssueOpsChildWithActor                func(stateRoot, parentID, childID string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	AddIssueOpsDecisionWithActor                func(stateRoot, id string, req issueopsdecisioncontract.Request, actor issueopsdecisioncontract.Actor) (issueopsdecisioncontract.Record, error)
	DropIssueOpsChildWithActor                  func(stateRoot, parentID, childID, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	IssueOpsChildStatusWithActor                func(stateRoot, parentID string, repair bool, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStatusResult, error)
	IssueOpsPRReadiness                         func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness
	IssueOpsStateRoot                           func() string
	IssueOpsStatus                              func(stateRoot, id string) (issueopsstatuscontract.Record, error)
	LinkIssueOpsChildWithActor                  func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsIssueWithActor                  func(stateRoot, id, issueURL string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsPlanWithActor                   func(stateRoot, id, planPath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsRelatedWithActor                func(stateRoot, id, linkType, relatedURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsWorktreeWithActor               func(stateRoot, id, worktreePath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ListIssueOpsCycles                          func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error)
	ObserveNativeProcessAncestry                func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	PrepareIssueOpsBranchWithActor              func(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	AwaitIssueOpsBranchLink                     func(ctx context.Context, stateRoot string, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error)
	PruneIssueOps                               func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error)
	ReadIssueOps                                func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsAISlopCleanEvidenceWithActor  func(stateRoot, id string, categories, verification []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsCompatibilityReviewWithActor  func(stateRoot, id string, req issueopscontract.IssueOpsCompatibilityReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDesignReviewWithActor         func(stateRoot, id string, req issueopscontract.IssueOpsDesignReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDevilsAdvocateReviewWithActor func(stateRoot, id string, req issueopscontract.IssueOpsDevilsAdvocateReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDomainReviewWithActor         func(stateRoot, id string, req issueopscontract.IssueOpsDomainReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsImplementationReview          func(stateRoot, id string, req issueopscontract.IssueOpsImplementationReviewRequest) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsIntentWithActor               func(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsPlanPrepWithActor             func(stateRoot, id string, req issueopscontract.IssueOpsPlanPrepRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsRoutingWithActor              func(stateRoot, id, phase, skill string, actor issueopsroutingcontract.Actor) (issueopsroutingcontract.Record, error)
	RegressIssueOpsForReplanWithActor           func(stateRoot, id, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	RejectIssueOpsChildWithActor                func(stateRoot, parentID, childID, reason string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error)
	ResolveIssueOpsFeedbackWithActor            func(stateRoot, id string, index int, resolution string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ScoreLiveRoutingFidelity                    func(stateRoot, id string, expected []issueopsroutingcontract.Expected) (issueopsroutingcontract.Result, int, error)
	StageIssueOpsArtifact                       func(stateRoot, id, name string, content []byte) (issueopsartifactcontract.Record, error)
	StagedIssueOpsArtifactNames                 func(stateRoot, id string) ([]string, error)
	StartIssueOps                               func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error)
	StartIssueOpsChildWithActor                 func(stateRoot string, req issueopscontract.IssueOpsChildStartRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStartResult, error)
	UnstageIssueOpsArtifact                     func(stateRoot, id, name string) (issueopsartifactcontract.Record, error)
}

func ConfigureIssueOpsRuntime2(deps IssueOpsCLIDeps) {
	if deps.AcceptIssueOpsChildWithActor != nil {
		issueOpsCLIDeps.AcceptIssueOpsChildWithActor = deps.AcceptIssueOpsChildWithActor
	}
	if deps.AddIssueOpsDecisionWithActor != nil {
		issueOpsCLIDeps.AddIssueOpsDecisionWithActor = deps.AddIssueOpsDecisionWithActor
	}
	if deps.DropIssueOpsChildWithActor != nil {
		issueOpsCLIDeps.DropIssueOpsChildWithActor = deps.DropIssueOpsChildWithActor
	}
	if deps.IssueOpsChildStatusWithActor != nil {
		issueOpsCLIDeps.IssueOpsChildStatusWithActor = deps.IssueOpsChildStatusWithActor
	}
	if deps.IssueOpsPRReadiness != nil {
		issueOpsCLIDeps.IssueOpsPRReadiness = deps.IssueOpsPRReadiness
	}
	if deps.IssueOpsStateRoot != nil {
		issueOpsCLIDeps.IssueOpsStateRoot = deps.IssueOpsStateRoot
	}
	if deps.IssueOpsStatus != nil {
		issueOpsCLIDeps.IssueOpsStatus = deps.IssueOpsStatus
	}
	if deps.LinkIssueOpsChildWithActor != nil {
		issueOpsCLIDeps.LinkIssueOpsChildWithActor = deps.LinkIssueOpsChildWithActor
	}
	if deps.LinkIssueOpsIssueWithActor != nil {
		issueOpsCLIDeps.LinkIssueOpsIssueWithActor = deps.LinkIssueOpsIssueWithActor
	}
	if deps.LinkIssueOpsPlanWithActor != nil {
		issueOpsCLIDeps.LinkIssueOpsPlanWithActor = deps.LinkIssueOpsPlanWithActor
	}
	if deps.LinkIssueOpsRelatedWithActor != nil {
		issueOpsCLIDeps.LinkIssueOpsRelatedWithActor = deps.LinkIssueOpsRelatedWithActor
	}
	if deps.LinkIssueOpsWorktreeWithActor != nil {
		issueOpsCLIDeps.LinkIssueOpsWorktreeWithActor = deps.LinkIssueOpsWorktreeWithActor
	}
	if deps.ListIssueOpsCycles != nil {
		issueOpsCLIDeps.ListIssueOpsCycles = deps.ListIssueOpsCycles
	}
	if deps.ObserveNativeProcessAncestry != nil {
		issueOpsCLIDeps.ObserveNativeProcessAncestry = deps.ObserveNativeProcessAncestry
	}
	if deps.AwaitIssueOpsBranchLink != nil {
		issueOpsCLIDeps.AwaitIssueOpsBranchLink = deps.AwaitIssueOpsBranchLink
	}
	if deps.PrepareIssueOpsBranchWithActor != nil {
		issueOpsCLIDeps.PrepareIssueOpsBranchWithActor = deps.PrepareIssueOpsBranchWithActor
	}
	if deps.PruneIssueOps != nil {
		issueOpsCLIDeps.PruneIssueOps = deps.PruneIssueOps
	}
	if deps.ReadIssueOps != nil {
		issueOpsCLIDeps.ReadIssueOps = deps.ReadIssueOps
	}
	if deps.RecordIssueOpsAISlopCleanEvidenceWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsAISlopCleanEvidenceWithActor = deps.RecordIssueOpsAISlopCleanEvidenceWithActor
	}
	if deps.RecordIssueOpsCompatibilityReviewWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsCompatibilityReviewWithActor = deps.RecordIssueOpsCompatibilityReviewWithActor
	}
	if deps.RecordIssueOpsDesignReviewWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsDesignReviewWithActor = deps.RecordIssueOpsDesignReviewWithActor
	}
	if deps.RecordIssueOpsDevilsAdvocateReviewWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsDevilsAdvocateReviewWithActor = deps.RecordIssueOpsDevilsAdvocateReviewWithActor
	}
	if deps.RecordIssueOpsDomainReviewWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsDomainReviewWithActor = deps.RecordIssueOpsDomainReviewWithActor
	}
	if deps.RecordIssueOpsImplementationReview != nil {
		issueOpsCLIDeps.RecordIssueOpsImplementationReview = deps.RecordIssueOpsImplementationReview
	}
	if deps.RecordIssueOpsIntentWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsIntentWithActor = deps.RecordIssueOpsIntentWithActor
	}
	if deps.RecordIssueOpsPlanPrepWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsPlanPrepWithActor = deps.RecordIssueOpsPlanPrepWithActor
	}
	if deps.RecordIssueOpsRoutingWithActor != nil {
		issueOpsCLIDeps.RecordIssueOpsRoutingWithActor = deps.RecordIssueOpsRoutingWithActor
	}
	if deps.RegressIssueOpsForReplanWithActor != nil {
		issueOpsCLIDeps.RegressIssueOpsForReplanWithActor = deps.RegressIssueOpsForReplanWithActor
	}
	if deps.RejectIssueOpsChildWithActor != nil {
		issueOpsCLIDeps.RejectIssueOpsChildWithActor = deps.RejectIssueOpsChildWithActor
	}
	if deps.ResolveIssueOpsFeedbackWithActor != nil {
		issueOpsCLIDeps.ResolveIssueOpsFeedbackWithActor = deps.ResolveIssueOpsFeedbackWithActor
	}
	if deps.ScoreLiveRoutingFidelity != nil {
		issueOpsCLIDeps.ScoreLiveRoutingFidelity = deps.ScoreLiveRoutingFidelity
	}
	if deps.StageIssueOpsArtifact != nil {
		issueOpsCLIDeps.StageIssueOpsArtifact = deps.StageIssueOpsArtifact
	}
	if deps.StagedIssueOpsArtifactNames != nil {
		issueOpsCLIDeps.StagedIssueOpsArtifactNames = deps.StagedIssueOpsArtifactNames
	}
	if deps.StartIssueOps != nil {
		issueOpsCLIDeps.StartIssueOps = deps.StartIssueOps
	}
	if deps.StartIssueOpsChildWithActor != nil {
		issueOpsCLIDeps.StartIssueOpsChildWithActor = deps.StartIssueOpsChildWithActor
	}
	if deps.UnstageIssueOpsArtifact != nil {
		issueOpsCLIDeps.UnstageIssueOpsArtifact = deps.UnstageIssueOpsArtifact
	}
}

// 배선 누락이 패닉이 아니라 명시적 오류로 드러나도록 중립 기본값을 둔다.
func neutralIssueOpsCLIDeps() IssueOpsCLIDeps {
	return IssueOpsCLIDeps{
		AcceptIssueOpsChildWithActor: func(stateRoot, parentID, childID string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return issueopscontract.IssueOpsChildValidationResult{}, errIssueOpsCLINotConfigured
		},
		AddIssueOpsDecisionWithActor: func(stateRoot, id string, req issueopsdecisioncontract.Request, actor issueopsdecisioncontract.Actor) (issueopsdecisioncontract.Record, error) {
			return issueopsdecisioncontract.Record{}, errIssueOpsCLINotConfigured
		},
		DropIssueOpsChildWithActor: func(stateRoot, parentID, childID, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return issueopscontract.IssueOpsChildValidationResult{}, errIssueOpsCLINotConfigured
		},
		IssueOpsChildStatusWithActor: func(stateRoot, parentID string, repair bool, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStatusResult, error) {
			return issueopscontract.IssueOpsChildStatusResult{}, errIssueOpsCLINotConfigured
		},
		IssueOpsPRReadiness: func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
			return issueopscontract.IssueOpsReadiness{}
		},
		IssueOpsStateRoot: func() string { return "" },
		IssueOpsStatus: func(stateRoot, id string) (issueopsstatuscontract.Record, error) {
			return issueopsstatuscontract.Record{}, errIssueOpsCLINotConfigured
		},
		LinkIssueOpsChildWithActor: func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		LinkIssueOpsIssueWithActor: func(stateRoot, id, issueURL string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		LinkIssueOpsPlanWithActor: func(stateRoot, id, planPath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		LinkIssueOpsRelatedWithActor: func(stateRoot, id, linkType, relatedURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		LinkIssueOpsWorktreeWithActor: func(stateRoot, id, worktreePath string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		ListIssueOpsCycles: func(stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
			return issueopsinventorycontract.ListResult{}, errIssueOpsCLINotConfigured
		},
		ObserveNativeProcessAncestry: func(pid int) ([]issueopscontract.NativeProcessReceipt, error) {
			return nil, errIssueOpsCLINotConfigured
		},
		PrepareIssueOpsBranchWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		PruneIssueOps: func(stateRoot string, maxAge time.Duration, confirm bool) (issueopsretentioncontract.Result, error) {
			return issueopsretentioncontract.Result{}, errIssueOpsCLINotConfigured
		},
		ReadIssueOps: func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsAISlopCleanEvidenceWithActor: func(stateRoot, id string, categories, verification []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsCompatibilityReviewWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsCompatibilityReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsDesignReviewWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsDesignReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsDevilsAdvocateReviewWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsDevilsAdvocateReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsDomainReviewWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsDomainReviewRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsImplementationReview: func(stateRoot, id string, req issueopscontract.IssueOpsImplementationReviewRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsIntentWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsPlanPrepWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsPlanPrepRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RecordIssueOpsRoutingWithActor: func(stateRoot, id, phase, skill string, actor issueopsroutingcontract.Actor) (issueopsroutingcontract.Record, error) {
			return issueopsroutingcontract.Record{}, errIssueOpsCLINotConfigured
		},
		RegressIssueOpsForReplanWithActor: func(stateRoot, id, reason string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		RejectIssueOpsChildWithActor: func(stateRoot, parentID, childID, reason string, evidence []string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildValidationResult, error) {
			return issueopscontract.IssueOpsChildValidationResult{}, errIssueOpsCLINotConfigured
		},
		ResolveIssueOpsFeedbackWithActor: func(stateRoot, id string, index int, resolution string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		ScoreLiveRoutingFidelity: func(stateRoot, id string, expected []issueopsroutingcontract.Expected) (issueopsroutingcontract.Result, int, error) {
			return issueopsroutingcontract.Result{OK: false}, 0, errIssueOpsCLINotConfigured
		},
		StageIssueOpsArtifact: func(stateRoot, id, name string, content []byte) (issueopsartifactcontract.Record, error) {
			return issueopsartifactcontract.Record{}, errIssueOpsCLINotConfigured
		},
		StagedIssueOpsArtifactNames: func(stateRoot, id string) ([]string, error) { return nil, errIssueOpsCLINotConfigured },
		StartIssueOps: func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsCLINotConfigured
		},
		StartIssueOpsChildWithActor: func(stateRoot string, req issueopscontract.IssueOpsChildStartRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsChildStartResult, error) {
			return issueopscontract.IssueOpsChildStartResult{}, errIssueOpsCLINotConfigured
		},
		UnstageIssueOpsArtifact: func(stateRoot, id, name string) (issueopsartifactcontract.Record, error) {
			return issueopsartifactcontract.Record{}, errIssueOpsCLINotConfigured
		},
	}
}
