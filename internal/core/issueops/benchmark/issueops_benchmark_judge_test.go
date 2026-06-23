package benchmark

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func TestIssueOpsLLMJudgeParsesStrictJSON(t *testing.T) {
	withFakeIssueOpsZAI(t, fakeZAIResponse{Content: `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`})
	result, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact: IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("unexpected judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsLLMJudgeParsesFencedJSON(t *testing.T) {
	withFakeIssueOpsZAI(t, fakeZAIResponse{Content: "```json\n" + `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}` + "\n```"})
	result, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact: IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("expected fenced JSON judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsLLMJudgeRejectsNoisyOutput(t *testing.T) {
	withFakeIssueOpsZAI(t, fakeZAIResponse{Content: `I will judge now. {"ok":true}`})
	_, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil {
		t.Fatal("expected strict JSON error")
	}
}

func TestIssueOpsLLMJudgeRetriesEmptyStrictOutput(t *testing.T) {
	withFakeIssueOpsZAI(t,
		fakeZAIResponse{Content: ""},
		fakeZAIResponse{Content: `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"retry ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`},
	)
	result, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Attempts: 2,
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover empty output: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsLLMJudgeRetriesExternalLLMFailure(t *testing.T) {
	withFakeIssueOpsZAI(t,
		fakeZAIResponse{Status: http.StatusInternalServerError, Body: `{"error":{"message":"transient failure"}}`},
		fakeZAIResponse{Content: `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"retry ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`},
	)
	result, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Attempts: 2,
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover external LLM failure: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsLLMJudgeRejectsDimensionScoreObjectWithOutputEvidence(t *testing.T) {
	output := `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":{"intent_understanding":{"score":100,"evidence":"object is invalid"}},"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`
	withFakeIssueOpsZAI(t, fakeZAIResponse{Content: output})
	_, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil {
		t.Fatal("expected object-shaped dimension_scores to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dimension_scores") || !strings.Contains(msg, "object is invalid") {
		t.Fatalf("expected decode error to include bounded output evidence, got: %v", err)
	}
}

func TestIssueOpsLLMJudgeRejectsFencedUnknownField(t *testing.T) {
	withFakeIssueOpsZAI(t, fakeZAIResponse{Content: "```json\n" + `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true,"unexpected":true}` + "\n```"})
	_, err := RunIssueOpsLLMJudge(IssueOpsLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Fixture:  IssueOpsBenchmarkFixture{ID: "fixture"},
		Attempts: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestIssueOpsLLMJudgePromptRequiresDimensionScoresArray(t *testing.T) {
	prompt, err := buildIssueOpsLLMJudgePrompt(
		IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "context", CriticalFailures: []string{"failure"}},
		IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dimension_scores must be a JSON array of objects",
		"Never encode dimension_scores as an object",
		`"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"short evidence"}]`,
		"Every rubric dimension appears exactly once in dimension_scores as an array item",
		"```json",
		"Response Schema",
		"Field Types",
		"Return a raw JSON object",
		"ok: boolean",
		"dimension_scores: array of objects",
		"dimension_scores[].score: number",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

type fakeZAIResponse struct {
	Status  int
	Body    string
	Content string
}

func withFakeIssueOpsZAI(t *testing.T, responses ...fakeZAIResponse) {
	t.Helper()
	if len(responses) == 0 {
		t.Fatal("missing fake Z.AI responses")
	}
	t.Setenv("Z_AI_API_KEY", "test-key")
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		response := responses[index]
		if index < len(responses)-1 {
			index++
		}
		if response.Status != 0 {
			w.WriteHeader(response.Status)
		}
		if response.Body != "" {
			_, _ = w.Write([]byte(response.Body))
			return
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, response.Content)
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
}
