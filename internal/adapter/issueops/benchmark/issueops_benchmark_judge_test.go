package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
	"testing"
)

const validBenchmarkJudgeJSON = `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`

func TestIssueOpsJudgeFileParsesStrictJSON(t *testing.T) {
	result, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte(validBenchmarkJudgeJSON))
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("unexpected judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsJudgeFileParsesFencedJSON(t *testing.T) {
	result, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte("```json\n" + validBenchmarkJudgeJSON + "\n```"))
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("expected fenced JSON judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsLLMJudgeReturnsRemovedServiceError(t *testing.T) {
	_, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		Fixture: issueopscontract.IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil || !strings.Contains(err.Error(), "no longer calls external LLM services") {
		t.Fatalf("expected removed service error, got %v", err)
	}
}

func TestIssueOpsJudgeFileRejectsNoisyOutput(t *testing.T) {
	_, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte(`I will judge now. {"ok":true}`))
	if err == nil {
		t.Fatal("expected strict JSON error")
	}
}

func TestIssueOpsJudgeFileRejectsDimensionScoreObjectWithOutputEvidence(t *testing.T) {
	output := `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":{"intent_understanding":{"score":100,"evidence":"object is invalid"}},"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`
	_, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte(output))
	if err == nil {
		t.Fatal("expected object-shaped dimension_scores to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dimension_scores") || !strings.Contains(msg, "object is invalid") {
		t.Fatalf("expected decode error to include bounded output evidence, got: %v", err)
	}
}

func TestIssueOpsJudgeFileRejectsFencedUnknownField(t *testing.T) {
	_, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte("```json\n" + `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true,"unexpected":true}` + "\n```"))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestIssueOpsLLMJudgePromptRequiresDimensionScoresArray(t *testing.T) {
	prompt, err := buildIssueOpsLLMJudgePrompt(
		issueopscontract.IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "context", CriticalFailures: []string{"failure"}},
		issueopscontract.IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dimension_scores must be a JSON array of objects",
		"Never encode dimension_scores as an object",
		`"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"short evidence"}]`,
		"Every rubric dimension appears exactly once in dimension_scores as an array item",
		"Host-Agent Judgement Response Schema",
		"ok: boolean",
		"dimension_scores: array of objects",
		"dimension_scores[].score: number",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
