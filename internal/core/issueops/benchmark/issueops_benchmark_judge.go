package benchmark

import (
	"agent-harness/internal/core/externalllm"
	"fmt"
	"strings"
	"time"
)

type IssueOpsLLMJudgeRequest struct {
	RepoRoot string
	Model    string
	Timeout  time.Duration
	Attempts int
	Fixture  IssueOpsBenchmarkFixture
	Artifact IssueOpsBenchmarkArtifact
}

func RunIssueOpsLLMJudge(req IssueOpsLLMJudgeRequest) (IssueOpsBenchmarkScore, error) {
	model := strings.TrimSpace(req.Model)
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	attempts := req.Attempts
	if attempts <= 0 {
		attempts = 3
	}

	prompt, err := buildIssueOpsLLMJudgePrompt(req.Fixture, req.Artifact)
	if err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		llm, err := externalllm.RunExternalLLMPrint(externalllm.ExternalLLMPrintRequest{Model: model, WorkDir: req.RepoRoot, Prompt: prompt, Timeout: timeout})
		if err != nil {
			lastErr = fmt.Errorf("issueops benchmark LLM judge failed: %s: %w", boundedIssueOpsText(string(llm.Output)), err)
			continue
		}
		score, err := decodeStrictIssueOpsBenchmarkScore(llm.Output)
		if err == nil {
			return score, nil
		}
		lastErr = err
	}
	return IssueOpsBenchmarkScore{}, fmt.Errorf("issueops benchmark LLM judge failed after %d strict-output attempts: %w", attempts, lastErr)
}

// DecodeIssueOpsBenchmarkJudgeJSON strictly decodes ONE judge score object
// (the same shape the LLM judge returns). Callers holding a map of
// fixture-ID -> score must decode the outer map themselves and feed each
// value through this function; the strict decoder rejects unknown fields, so
// passing the whole map here fails by design.
func DecodeIssueOpsBenchmarkJudgeJSON(out []byte) (IssueOpsBenchmarkScore, error) {
	return decodeStrictIssueOpsBenchmarkScore(out)
}

func decodeStrictIssueOpsBenchmarkScore(out []byte) (IssueOpsBenchmarkScore, error) {
	var score IssueOpsBenchmarkScore
	if err := externalllm.DecodeExternalLLMStructuredJSONObject("issueops benchmark LLM judge", out, &score); err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	if len(score.DimensionScores) == 0 {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("issueops benchmark LLM judge output missing dimension_scores")
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
