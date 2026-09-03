package remotecmd

import (
	"context"
	"errors"
	"fmt"

	issueopscontract "agent-harness/internal/contract/issueops"
	bodysynccontract "agent-harness/internal/contract/issueopsbodysync"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
	"agent-harness/internal/port"
)

var errRemoteNotConfigured = errors.New("issueops remote is not configured")

// 원격 이슈·PR 연산은 외부 제공자와 상태 저장소를 다루는 I/O다. CLI는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var remoteDeps = neutralRemoteDeps()

// RemoteDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type RemoteDeps struct {
	BeginIssueCreateIntent             func(stateRoot, id string, request issueopscontract.IssueOpsIssueCreateIntentRequest) (issueopscontract.IssueOpsRecord, error)
	CloseIssueOpsRemoteIssue           func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error)
	CompleteIssueCreateIntent          func(stateRoot, id, issueURL, completedAt string) (issueopscontract.IssueOpsRecord, error)
	CreateRemoteChild                  func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error)
	CreateRemoteIssue                  func(req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error)
	CreateRemoteIssueContext           func(ctx context.Context, req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error)
	CreateRemotePullRequestWithHandler func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error)
	DecodeIssueOpsRemoteJudgeJSON      func(out []byte) (issueopsremote.IssueOpsRemoteScoringResult, error)
	DecodeIssueOpsRemoteScoringRequest func(data []byte) (issueopsremote.IssueOpsRemoteScoringRequest, error)
	// InferProviderFromRepoRemotes는 record가 provider를 모를 때 저장소 remote로
	// 판별한다. 최초 이슈 생성의 bootstrap 순환을 끊는 유일한 경로다(#300).
	InferProviderFromRepoRemotes               func(repo string) (string, error)
	IssueOpsStateRoot                          func() string
	LinkIssueOpsChildWithActor                 func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ObserveNativeProcessAncestry               func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReadIssueOps                               func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	RecordIssueCreateOutcome                   func(stateRoot, id string, outcome issueopscontract.IssueOpsIssueCreateOutcome) (issueopscontract.IssueOpsRecord, error)
	ReflectDevilsAdvocateFindingsWithActor     func(stateRoot, id string, confirm bool, prov port.IssueProvider, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	ReflectIssueCompletion                     func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	RenderIssueOpsRemoteJudgePrompt            func(req issueopsremote.IssueOpsRemoteLLMJudgeRequest) (issueopsremote.IssueOpsRemoteJudgePromptResult, error)
	ResolveRecordProvider                      func(record issueopscontract.IssueOpsRecord) string
	ResolveProviderProjectAuthority            func(repo, provider string) (string, error)
	ScoreIssueOpsRemoteCandidates              func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error)
	SyncRemoteArtifactBody                     func(ctx context.Context, stateRoot, id string, cmd bodysynccontract.Command, prov port.IssueProvider, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error)
	SyncRemoteIssueGraph                       func(record issueopscontract.IssueOpsRecord) (map[string]any, error)
	UmbrellaBranchGateReason                   func(record issueopscontract.IssueOpsRecord) string
	ValidateIssueOpsMutationActor              func(stateRoot, id string, actor issueopscontract.IssueOpsActor) error
	ValidateIssueOpsRemoteArtifactVerification func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) (issueopscontract.IssueOpsRecord, error)
	VerifyIssueOpsRemoteArtifactWithActor      func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
}

// ConfigureRemote는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureRemote(deps RemoteDeps) {
	if deps.BeginIssueCreateIntent != nil {
		remoteDeps.BeginIssueCreateIntent = deps.BeginIssueCreateIntent
	}
	if deps.CloseIssueOpsRemoteIssue != nil {
		remoteDeps.CloseIssueOpsRemoteIssue = deps.CloseIssueOpsRemoteIssue
	}
	if deps.CompleteIssueCreateIntent != nil {
		remoteDeps.CompleteIssueCreateIntent = deps.CompleteIssueCreateIntent
	}
	if deps.InferProviderFromRepoRemotes != nil {
		remoteDeps.InferProviderFromRepoRemotes = deps.InferProviderFromRepoRemotes
	}
	if deps.CreateRemoteChild != nil {
		remoteDeps.CreateRemoteChild = deps.CreateRemoteChild
	}
	if deps.CreateRemoteIssue != nil {
		remoteDeps.CreateRemoteIssue = deps.CreateRemoteIssue
	}
	if deps.CreateRemoteIssueContext != nil {
		remoteDeps.CreateRemoteIssueContext = deps.CreateRemoteIssueContext
	}
	if deps.CreateRemotePullRequestWithHandler != nil {
		remoteDeps.CreateRemotePullRequestWithHandler = deps.CreateRemotePullRequestWithHandler
	}
	if deps.DecodeIssueOpsRemoteJudgeJSON != nil {
		remoteDeps.DecodeIssueOpsRemoteJudgeJSON = deps.DecodeIssueOpsRemoteJudgeJSON
	}
	if deps.DecodeIssueOpsRemoteScoringRequest != nil {
		remoteDeps.DecodeIssueOpsRemoteScoringRequest = deps.DecodeIssueOpsRemoteScoringRequest
	}
	if deps.IssueOpsStateRoot != nil {
		remoteDeps.IssueOpsStateRoot = deps.IssueOpsStateRoot
	}
	if deps.LinkIssueOpsChildWithActor != nil {
		remoteDeps.LinkIssueOpsChildWithActor = deps.LinkIssueOpsChildWithActor
	}
	if deps.ObserveNativeProcessAncestry != nil {
		remoteDeps.ObserveNativeProcessAncestry = deps.ObserveNativeProcessAncestry
	}
	if deps.ReadIssueOps != nil {
		remoteDeps.ReadIssueOps = deps.ReadIssueOps
	}
	if deps.RecordIssueCreateOutcome != nil {
		remoteDeps.RecordIssueCreateOutcome = deps.RecordIssueCreateOutcome
	}
	if deps.ReflectDevilsAdvocateFindingsWithActor != nil {
		remoteDeps.ReflectDevilsAdvocateFindingsWithActor = deps.ReflectDevilsAdvocateFindingsWithActor
	}
	if deps.ReflectIssueCompletion != nil {
		remoteDeps.ReflectIssueCompletion = deps.ReflectIssueCompletion
	}
	if deps.RenderIssueOpsRemoteJudgePrompt != nil {
		remoteDeps.RenderIssueOpsRemoteJudgePrompt = deps.RenderIssueOpsRemoteJudgePrompt
	}
	if deps.ResolveRecordProvider != nil {
		remoteDeps.ResolveRecordProvider = deps.ResolveRecordProvider
	}
	if deps.ResolveProviderProjectAuthority != nil {
		remoteDeps.ResolveProviderProjectAuthority = deps.ResolveProviderProjectAuthority
	}
	if deps.ScoreIssueOpsRemoteCandidates != nil {
		remoteDeps.ScoreIssueOpsRemoteCandidates = deps.ScoreIssueOpsRemoteCandidates
	}
	if deps.SyncRemoteArtifactBody != nil {
		remoteDeps.SyncRemoteArtifactBody = deps.SyncRemoteArtifactBody
	}
	if deps.SyncRemoteIssueGraph != nil {
		remoteDeps.SyncRemoteIssueGraph = deps.SyncRemoteIssueGraph
	}
	if deps.UmbrellaBranchGateReason != nil {
		remoteDeps.UmbrellaBranchGateReason = deps.UmbrellaBranchGateReason
	}
	if deps.ValidateIssueOpsMutationActor != nil {
		remoteDeps.ValidateIssueOpsMutationActor = deps.ValidateIssueOpsMutationActor
	}
	if deps.ValidateIssueOpsRemoteArtifactVerification != nil {
		remoteDeps.ValidateIssueOpsRemoteArtifactVerification = deps.ValidateIssueOpsRemoteArtifactVerification
	}
	if deps.VerifyIssueOpsRemoteArtifactWithActor != nil {
		remoteDeps.VerifyIssueOpsRemoteArtifactWithActor = deps.VerifyIssueOpsRemoteArtifactWithActor
	}
}

// 배선 누락이 패닉이 아니라 명시적 오류로 드러나도록 중립 기본값을 둔다.
func neutralRemoteDeps() RemoteDeps {
	return RemoteDeps{
		BeginIssueCreateIntent: func(stateRoot, id string, request issueopscontract.IssueOpsIssueCreateIntentRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		CloseIssueOpsRemoteIssue: func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderCloseIssueResult{}, errRemoteNotConfigured
		},
		CompleteIssueCreateIntent: func(stateRoot, id, issueURL, completedAt string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		CreateRemoteChild: func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error) {
			return port.IssueProviderCreateChildResult{}, errRemoteNotConfigured
		},
		CreateRemoteIssue: func(req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error) {
			return port.IssueProviderCreateIssueResult{}, errRemoteNotConfigured
		},
		CreateRemoteIssueContext: func(ctx context.Context, req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error) {
			return port.IssueProviderCreateIssueResult{}, errRemoteNotConfigured
		},
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{}, errRemoteNotConfigured
		},
		DecodeIssueOpsRemoteJudgeJSON: func(out []byte) (issueopsremote.IssueOpsRemoteScoringResult, error) {
			return issueopsremote.IssueOpsRemoteScoringResult{}, errRemoteNotConfigured
		},
		DecodeIssueOpsRemoteScoringRequest: func(data []byte) (issueopsremote.IssueOpsRemoteScoringRequest, error) {
			return issueopsremote.IssueOpsRemoteScoringRequest{}, errRemoteNotConfigured
		},
		IssueOpsStateRoot: func() string { return "" },
		LinkIssueOpsChildWithActor: func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		ObserveNativeProcessAncestry: func(pid int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, errRemoteNotConfigured },
		ReadIssueOps: func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		RecordIssueCreateOutcome: func(stateRoot, id string, outcome issueopscontract.IssueOpsIssueCreateOutcome) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		ReflectDevilsAdvocateFindingsWithActor: func(stateRoot, id string, confirm bool, prov port.IssueProvider, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, errRemoteNotConfigured
		},
		ReflectIssueCompletion: func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, errRemoteNotConfigured
		},
		RenderIssueOpsRemoteJudgePrompt: func(req issueopsremote.IssueOpsRemoteLLMJudgeRequest) (issueopsremote.IssueOpsRemoteJudgePromptResult, error) {
			return issueopsremote.IssueOpsRemoteJudgePromptResult{}, errRemoteNotConfigured
		},
		ResolveRecordProvider: func(record issueopscontract.IssueOpsRecord) string { return "" },
		ResolveProviderProjectAuthority: func(repo, provider string) (string, error) {
			return "", errRemoteNotConfigured
		},
		ScoreIssueOpsRemoteCandidates: func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error) {
			return issueopsremote.IssueOpsRemoteScoringResult{}, errRemoteNotConfigured
		},
		SyncRemoteArtifactBody: func(ctx context.Context, stateRoot, id string, cmd bodysynccontract.Command, prov port.IssueProvider, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error) {
			return issueopscontract.IssueOpsRecord{}, bodysynccontract.Result{}, errRemoteNotConfigured
		},
		SyncRemoteIssueGraph: func(record issueopscontract.IssueOpsRecord) (map[string]any, error) {
			return nil, errRemoteNotConfigured
		},
		UmbrellaBranchGateReason:      func(record issueopscontract.IssueOpsRecord) string { return "" },
		ValidateIssueOpsMutationActor: func(stateRoot, id string, actor issueopscontract.IssueOpsActor) error { return errRemoteNotConfigured },
		ValidateIssueOpsRemoteArtifactVerification: func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
		VerifyIssueOpsRemoteArtifactWithActor: func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errRemoteNotConfigured
		},
	}
}

// inferProviderFromRepoRemotes는 주입된 관측이 있으면 그것으로 provider를
// 판별하고, 없으면 원래의 bootstrap 오류를 그대로 돌려준다. 관측 없이
// 추측하지 않는다.
func inferProviderFromRepoRemotes(repo string) (string, error) {
	if remoteDeps.InferProviderFromRepoRemotes == nil {
		return "", fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set or pass --provider github|gitlab")
	}
	return remoteDeps.InferProviderFromRepoRemotes(repo)
}
