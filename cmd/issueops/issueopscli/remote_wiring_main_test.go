package issueopscli

import (
	"context"

	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscore "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. 원격 CLI 계약 테스트는 실제 연산 경로를
// 검증하므로 같은 배선을 재현한다.
func wireRemoteForTests() {

	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		BeginIssueCreateIntent:    issueopscore.BeginIssueCreateIntent,
		CloseIssueOpsRemoteIssue:  issueopscore.CloseIssueOpsRemoteIssue,
		CompleteIssueCreateIntent: issueopscore.CompleteIssueCreateIntent,
		CreateRemoteChild:         issueopscore.CreateRemoteChild,
		CreateRemoteIssue:         issueopscore.CreateRemoteIssue,
		CreateRemoteIssueContext:  issueopscore.CreateRemoteIssueContext,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:              issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest:         issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                          issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:                 issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		RecordIssueCreateOutcome:                   issueopscore.RecordIssueCreateOutcome,
		ReflectDevilsAdvocateFindingsWithActor:     issueopscore.ReflectDevilsAdvocateFindingsWithActor,
		ReflectIssueCompletion:                     issueopscore.ReflectIssueCompletion,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:                      issueopscore.ResolveRecordProvider,
		ResolveProviderProjectAuthority:            issueopscore.ResolveProviderProjectAuthority,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
}
