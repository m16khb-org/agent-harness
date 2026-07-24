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

// DecodeIssueOpsBenchmarkJudgeJSON는 judge score 객체 하나만 엄격하게 디코딩한다
// (LLM judge가 반환하는 것과 같은 형태다). fixture-ID -> score 맵을 가진 호출자는
// 바깥 맵을 직접 디코딩한 뒤 각 값을 이 함수에 넣어야 한다. strict decoder는 알 수
// 없는 필드를 거부하므로 맵 전체를 여기 넘기면 의도적으로 실패한다.
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
