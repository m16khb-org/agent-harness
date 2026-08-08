package remote

import (
	"fmt"

	"agent-harness/internal/domain/judgement"
)

func RunIssueOpsRemoteLLMJudge(req IssueOpsRemoteLLMJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	if _, err := buildIssueOpsRemoteLLMJudgePrompt(req.Request); err != nil {
		return IssueOpsRemoteScoringResult{}, err
	}
	return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote score no longer calls external LLM services; render the prompt with BuildIssueOpsRemoteLLMJudgePrompt and pass the host-agent result through --judge file --judge-file")
}

func RenderIssueOpsRemoteLLMJudgePrompt(req IssueOpsRemoteLLMJudgeRequest) (string, error) {
	prompt, err := buildIssueOpsRemoteLLMJudgePrompt(req.Request)
	if err != nil {
		return "", err
	}
	return prompt, nil
}

func RenderIssueOpsRemoteJudgePrompt(req IssueOpsRemoteLLMJudgeRequest) (IssueOpsRemoteJudgePromptResult, error) {
	prompt, err := RenderIssueOpsRemoteLLMJudgePrompt(req)
	if err != nil {
		return IssueOpsRemoteJudgePromptResult{}, err
	}
	return IssueOpsRemoteJudgePromptResult{
		OK:             true,
		ExecutionClass: "background_join",
		ReadOnly:       true,
		JoinBefore:     "remote_artifact_write",
		Prompt:         prompt,
	}, nil
}

func DecodeIssueOpsRemoteJudgeJSON(out []byte) (IssueOpsRemoteScoringResult, error) {
	result, err := decodeStrictIssueOpsRemoteScoringResult(out)
	if err != nil {
		return IssueOpsRemoteScoringResult{}, err
	}
	return normalizeIssueOpsRemoteScoringResult(result), nil
}

func decodeStrictIssueOpsRemoteScoringResult(out []byte) (IssueOpsRemoteScoringResult, error) {
	var result IssueOpsRemoteScoringResult
	if err := judgement.DecodeStructuredJSONObject("issueops remote host-agent scoring", out, &result); err != nil {
		return IssueOpsRemoteScoringResult{}, err
	}
	if !result.OK {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote host-agent scoring output not ok")
	}
	if result.ExecutionClass != "background_join" {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote host-agent scoring execution_class must be background_join")
	}
	if !result.ReadOnly {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote host-agent scoring read_only must be true")
	}
	if result.JoinBefore != "remote_artifact_write" {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote host-agent scoring join_before must be remote_artifact_write")
	}
	return result, nil
}
