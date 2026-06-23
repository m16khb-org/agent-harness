package remote

import (
	"agent-harness/internal/core/externalllm"
	"fmt"
	"strings"
	"time"
)

func RunIssueOpsRemoteLLMJudge(req IssueOpsRemoteLLMJudgeRequest) (IssueOpsRemoteScoringResult, error) {
	model := strings.TrimSpace(req.Model)
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	attempts := req.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	prompt, err := buildIssueOpsRemoteLLMJudgePrompt(req.Request)
	if err != nil {
		return IssueOpsRemoteScoringResult{}, err
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		llm, err := externalllm.RunExternalLLMPrint(externalllm.ExternalLLMPrintRequest{Model: model, WorkDir: req.RepoRoot, Prompt: prompt, Timeout: timeout})
		if err != nil {
			lastErr = fmt.Errorf("issueops remote LLM judge failed: %s: %w", boundedIssueOpsText(string(llm.Output)), err)
			continue
		}
		result, err := decodeStrictIssueOpsRemoteScoringResult(llm.Output)
		if err == nil {
			return normalizeIssueOpsRemoteScoringResult(result), nil
		}
		lastErr = err
	}
	return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote LLM judge failed after %d strict-output attempts: %w", attempts, lastErr)
}

func decodeStrictIssueOpsRemoteScoringResult(out []byte) (IssueOpsRemoteScoringResult, error) {
	var result IssueOpsRemoteScoringResult
	if err := externalllm.DecodeExternalLLMStructuredJSONObject("issueops remote LLM scoring", out, &result); err != nil {
		return IssueOpsRemoteScoringResult{}, err
	}
	if !result.OK {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote LLM scoring output not ok")
	}
	if result.ExecutionClass != "background_join" {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote LLM scoring execution_class must be background_join")
	}
	if !result.ReadOnly {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote LLM scoring read_only must be true")
	}
	if result.JoinBefore != "remote_artifact_write" {
		return IssueOpsRemoteScoringResult{}, fmt.Errorf("issueops remote LLM scoring join_before must be remote_artifact_write")
	}
	return result, nil
}
