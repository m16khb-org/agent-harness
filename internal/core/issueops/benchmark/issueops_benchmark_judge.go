package benchmark

import (
	"fmt"

	"agent-harness/internal/core/judgement"
)

type IssueOpsLLMJudgeRequest struct {
	Fixture  IssueOpsBenchmarkFixture
	Artifact IssueOpsBenchmarkArtifact
}

func RunIssueOpsLLMJudge(req IssueOpsLLMJudgeRequest) (IssueOpsBenchmarkScore, error) {
	if _, err := buildIssueOpsLLMJudgePrompt(req.Fixture, req.Artifact); err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	return IssueOpsBenchmarkScore{}, fmt.Errorf("issueops benchmark judge no longer calls external LLM services; render the prompt with BuildIssueOpsLLMJudgePrompt and pass the host-agent result through --judge file --judge-file")
}

func RenderIssueOpsLLMJudgePrompt(req IssueOpsLLMJudgeRequest) (string, error) {
	prompt, err := buildIssueOpsLLMJudgePrompt(req.Fixture, req.Artifact)
	if err != nil {
		return "", err
	}
	return prompt, nil
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
	if err := judgement.DecodeStructuredJSONObject("issueops benchmark host-agent judge", out, &score); err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	if len(score.DimensionScores) == 0 {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("issueops benchmark host-agent judge output missing dimension_scores")
	}
	return score, nil
}
