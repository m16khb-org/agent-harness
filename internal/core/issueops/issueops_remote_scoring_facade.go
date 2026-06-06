package issueops

import "agent-harness/internal/core/issueops/remote"

type IssueOpsRemoteArtifact = remote.IssueOpsRemoteArtifact
type IssueOpsRemoteIssueCandidate = remote.IssueOpsRemoteIssueCandidate
type IssueOpsRemoteLabelCandidate = remote.IssueOpsRemoteLabelCandidate
type IssueOpsRemoteScoringRequest = remote.IssueOpsRemoteScoringRequest
type IssueOpsRemoteScoredItem = remote.IssueOpsRemoteScoredItem
type IssueOpsRemoteScoringResult = remote.IssueOpsRemoteScoringResult
type IssueOpsRemoteAgyJudgeRequest = remote.IssueOpsRemoteAgyJudgeRequest

func DecodeIssueOpsRemoteScoringRequest(data []byte) (IssueOpsRemoteScoringRequest, error) {
	return remote.DecodeIssueOpsRemoteScoringRequest(data)
}

func ScoreIssueOpsRemoteCandidates(req IssueOpsRemoteScoringRequest) (IssueOpsRemoteScoringResult, error) {
	return remote.ScoreIssueOpsRemoteCandidates(req)
}

func RunIssueOpsRemoteAgyJudge(req IssueOpsRemoteAgyJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	return remote.RunIssueOpsRemoteAgyJudge(req)
}
