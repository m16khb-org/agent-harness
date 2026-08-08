package remotecmd

import (
	"context"
	"errors"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsremote "agent-harness/internal/domain/issueopsremote"
	"agent-harness/internal/port"
)

var errRemoteNotConfigured = errors.New("issueops remote is not configured")

// 원격 이슈·PR 연산은 외부 제공자와 상태 저장소를 다루는 I/O다. CLI는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var remoteDeps = neutralRemoteDeps()

// RemoteDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type RemoteDeps struct {
	CloseIssueOpsRemoteIssue                   func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error)
	CreateRemoteChild                          func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error)
	CreateRemoteIssue                          func(req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error)
	CreateRemotePullRequestWithHandler         func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error)
	DecodeIssueOpsRemoteJudgeJSON              func(out []byte) (issueopsremote.IssueOpsRemoteScoringResult, error)
	DecodeIssueOpsRemoteScoringRequest         func(data []byte) (issueopsremote.IssueOpsRemoteScoringRequest, error)
	IssueOpsStateRoot                          func() string
	LinkIssueOpsChildWithActor                 func(stateRoot, id, childURL, title string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
	ObserveNativeProcessAncestry               func(pid int) ([]issueopscontract.NativeProcessReceipt, error)
	ReadIssueOps                               func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	ReflectDevilsAdvocateFindingsWithActor     func(stateRoot, id string, confirm bool, prov port.IssueProvider, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	ReflectIssueCompletion                     func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, error)
	RenderIssueOpsRemoteJudgePrompt            func(req issueopsremote.IssueOpsRemoteLLMJudgeRequest) (issueopsremote.IssueOpsRemoteJudgePromptResult, error)
	ResolveRecordProvider                      func(record issueopscontract.IssueOpsRecord) string
	ScoreIssueOpsRemoteCandidates              func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error)
	SyncRemoteIssueGraph                       func(record issueopscontract.IssueOpsRecord) (map[string]any, error)
	UmbrellaBranchGateReason                   func(record issueopscontract.IssueOpsRecord) string
	ValidateIssueOpsMutationActor              func(stateRoot, id string, actor issueopscontract.IssueOpsActor) error
	ValidateIssueOpsRemoteArtifactVerification func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) (issueopscontract.IssueOpsRecord, error)
	VerifyIssueOpsRemoteArtifactWithActor      func(stateRoot, id string, req issueopscontract.IssueOpsRemoteArtifactVerificationRequest, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error)
}

// ConfigureRemote는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureRemote(deps RemoteDeps) {
	if deps.CloseIssueOpsRemoteIssue != nil {
		remoteDeps.CloseIssueOpsRemoteIssue = deps.CloseIssueOpsRemoteIssue
	}
	if deps.CreateRemoteChild != nil {
		remoteDeps.CreateRemoteChild = deps.CreateRemoteChild
	}
	if deps.CreateRemoteIssue != nil {
		remoteDeps.CreateRemoteIssue = deps.CreateRemoteIssue
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
	if deps.ScoreIssueOpsRemoteCandidates != nil {
		remoteDeps.ScoreIssueOpsRemoteCandidates = deps.ScoreIssueOpsRemoteCandidates
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
		CloseIssueOpsRemoteIssue: func(stateRoot, id string, merged, confirm bool, prov port.IssueProvider) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			return issueopscontract.IssueOpsRecord{}, port.IssueProviderCloseIssueResult{}, errRemoteNotConfigured
		},
		CreateRemoteChild: func(req port.IssueProviderCreateChildRequest, prov port.IssueProvider) (port.IssueProviderCreateChildResult, error) {
			return port.IssueProviderCreateChildResult{}, errRemoteNotConfigured
		},
		CreateRemoteIssue: func(req port.IssueProviderCreateIssueRequest, prov port.IssueProvider) (port.IssueProviderCreateIssueResult, error) {
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
		ScoreIssueOpsRemoteCandidates: func(req issueopsremote.IssueOpsRemoteScoringRequest) (issueopsremote.IssueOpsRemoteScoringResult, error) {
			return issueopsremote.IssueOpsRemoteScoringResult{}, errRemoteNotConfigured
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
