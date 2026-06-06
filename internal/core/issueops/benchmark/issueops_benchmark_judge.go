package benchmark

import (
	"agent-harness/internal/core/externalllm"
	"fmt"
	"strings"
	"time"
)

type IssueOpsAgyJudgeRequest struct {
	RepoRoot   string
	AgyCommand string
	Timeout    time.Duration
	Attempts   int
	Fixture    IssueOpsBenchmarkFixture
	Artifact   IssueOpsBenchmarkArtifact
}

func RunIssueOpsAgyJudge(req IssueOpsAgyJudgeRequest) (IssueOpsBenchmarkScore, error) {
	command := strings.TrimSpace(req.AgyCommand)
	if command == "" {
		command = "agy"
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	attempts := req.Attempts
	if attempts <= 0 {
		attempts = 3
	}

	prompt, err := buildIssueOpsAgyJudgePrompt(req.Fixture, req.Artifact)
	if err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		llm, err := externalllm.RunExternalLLMPrint(externalllm.ExternalLLMPrintRequest{Command: command, WorkDir: req.RepoRoot, Prompt: prompt, Timeout: timeout})
		if err != nil {
			lastErr = fmt.Errorf("agy judge failed: %s: %w", boundedIssueOpsText(string(llm.Output)), err)
			continue
		}
		score, err := decodeStrictIssueOpsBenchmarkScore(llm.Output)
		if err == nil {
			return score, nil
		}
		lastErr = err
	}
	return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge failed after %d strict-output attempts: %w", attempts, lastErr)
}

func decodeStrictIssueOpsBenchmarkScore(out []byte) (IssueOpsBenchmarkScore, error) {
	var score IssueOpsBenchmarkScore
	if err := externalllm.DecodeExternalLLMStructuredJSONObject("agy judge", out, &score); err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	if len(score.DimensionScores) == 0 {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge output missing dimension_scores")
	}
	return score, nil
}

func boundedIssueOpsText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "...[truncated]"
	}
	return s
}
