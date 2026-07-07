package issueops

import "agent-harness/internal/core/issueops/remote"

type IssueOpsRemoteArtifact = remote.IssueOpsRemoteArtifact
type IssueOpsRemoteIssueCandidate = remote.IssueOpsRemoteIssueCandidate
type IssueOpsRemoteLabelCandidate = remote.IssueOpsRemoteLabelCandidate
type IssueOpsRemoteScoringRequest = remote.IssueOpsRemoteScoringRequest
type IssueOpsRemoteScoredItem = remote.IssueOpsRemoteScoredItem
type IssueOpsRemoteScoringResult = remote.IssueOpsRemoteScoringResult
type IssueOpsRemoteLLMJudgeRequest = remote.IssueOpsRemoteLLMJudgeRequest

func DecodeIssueOpsRemoteScoringRequest(data []byte) (IssueOpsRemoteScoringRequest, error) {
	return remote.DecodeIssueOpsRemoteScoringRequest(data)
}

func ScoreIssueOpsRemoteCandidates(req IssueOpsRemoteScoringRequest) (IssueOpsRemoteScoringResult, error) {
	return remote.ScoreIssueOpsRemoteCandidates(req)
}

func RunIssueOpsRemoteLLMJudge(req IssueOpsRemoteLLMJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	return remote.RunIssueOpsRemoteLLMJudge(req)
}

func RenderIssueOpsRemoteLLMJudgePrompt(req IssueOpsRemoteLLMJudgeRequest) (string, error) {
	return remote.RenderIssueOpsRemoteLLMJudgePrompt(req)
}

func DecodeIssueOpsRemoteJudgeJSON(out []byte) (IssueOpsRemoteScoringResult, error) {
	return remote.DecodeIssueOpsRemoteJudgeJSON(out)
}
