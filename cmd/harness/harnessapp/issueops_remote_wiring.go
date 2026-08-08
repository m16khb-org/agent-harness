package harnessapp

import (
	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"context"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// remote CLI는 원격 연산 구현을 알지 않는다. 어댑터를 아는 곳은 composition
// root 하나뿐이다.
func configureIssueOpsRemote() {
	remotecmd.ConfigureRemote(remotecmd.RemoteDeps{
		CloseIssueOpsRemoteIssue: issueopscore.CloseIssueOpsRemoteIssue,
		CreateRemoteChild:        issueopscore.CreateRemoteChild,
		CreateRemoteIssue:        issueopscore.CreateRemoteIssue,
		CreateRemotePullRequestWithHandler: func(ctx context.Context, stateRoot string, req issueopscontract.RemotePullRequestRequest, handler func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)) (port.IssueProviderCreatePullRequestResult, error) {
			return issueopscore.CreateRemotePullRequestWithHandler(ctx, stateRoot, req, handler)
		},
		DecodeIssueOpsRemoteJudgeJSON:              issueopscore.DecodeIssueOpsRemoteJudgeJSON,
		DecodeIssueOpsRemoteScoringRequest:         issueopscore.DecodeIssueOpsRemoteScoringRequest,
		IssueOpsStateRoot:                          issueopscore.IssueOpsStateRoot,
		LinkIssueOpsChildWithActor:                 issueopscore.LinkIssueOpsChildWithActor,
		ObserveNativeProcessAncestry:               issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                               issueopscore.ReadIssueOps,
		ReflectDevilsAdvocateFindingsWithActor:     issueopscore.ReflectDevilsAdvocateFindingsWithActor,
		ReflectIssueCompletion:                     issueopscore.ReflectIssueCompletion,
		RenderIssueOpsRemoteJudgePrompt:            issueopscore.RenderIssueOpsRemoteJudgePrompt,
		ResolveRecordProvider:                      issueopscore.ResolveRecordProvider,
		ScoreIssueOpsRemoteCandidates:              issueopscore.ScoreIssueOpsRemoteCandidates,
		SyncRemoteIssueGraph:                       issueopscore.SyncRemoteIssueGraph,
		UmbrellaBranchGateReason:                   issueopscore.UmbrellaBranchGateReason,
		ValidateIssueOpsMutationActor:              issueopscore.ValidateIssueOpsMutationActor,
		ValidateIssueOpsRemoteArtifactVerification: issueopscore.ValidateIssueOpsRemoteArtifactVerification,
		VerifyIssueOpsRemoteArtifactWithActor:      issueopscore.VerifyIssueOpsRemoteArtifactWithActor,
	})
}
