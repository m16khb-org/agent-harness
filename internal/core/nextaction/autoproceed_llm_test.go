package nextaction

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func safeRecommendedMessage() string {
	return strings.Join([]string{
		"선택지:",
		"1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 멈춥니다.",
	}, "\n")
}

func TestEvaluateNextActionAutoProceedLLMAutoProceedsWhenLLMApproves(t *testing.T) {
	withFakeNextActionZAI(t, nextActionFakeZAIResponse{Content: `{"auto_proceed":true,"reason":"safe forward step"}`})
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: safeRecommendedMessage(),
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoProceed {
		t.Fatalf("LLM approval should auto-proceed, got %+v", result)
	}
	if result.TopScore != 1.0 {
		t.Fatalf("expected top score 1.0 on approval, got %.2f", result.TopScore)
	}
	if result.SelectedIndex != 1 {
		t.Fatalf("expected selected index 1, got %d", result.SelectedIndex)
	}
}

func TestEvaluateNextActionAutoProceedLLMDoesNotProceedWhenLLMDeclines(t *testing.T) {
	withFakeNextActionZAI(t, nextActionFakeZAIResponse{Content: `{"auto_proceed":false,"reason":"needs user confirmation"}`})
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: safeRecommendedMessage(),
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AutoProceed {
		t.Fatalf("LLM decline must not auto-proceed, got %+v", result)
	}
	if result.TopScore != 0.0 {
		t.Fatalf("expected top score 0.0 on decline, got %.2f", result.TopScore)
	}
}

func TestEvaluateNextActionAutoProceedLLMHardVetoesDestructiveWithoutCallingModel(t *testing.T) {
	requests := withFakeNextActionZAI(t, nextActionFakeZAIResponse{Content: `{"auto_proceed":true,"reason":"must not be called"}`})
	message := strings.Join([]string{
		"선택지:",
		"1. 정리 진행: merged worktree와 branch를 삭제합니다. (추천)",
		"2. 보류: 유지합니다.",
		"3. 확장 정리: 전체를 점검합니다.",
	}, "\n")
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: message,
	}, 0)
	if err != nil {
		t.Fatalf("destructive hard-veto must not invoke LLM or error, got %v", err)
	}
	if got := atomic.LoadInt32(requests); got != 0 {
		t.Fatalf("destructive hard-veto invoked LLM %d times", got)
	}
	if result.AutoProceed {
		t.Fatalf("destructive recommendation must never auto-proceed, got %+v", result)
	}
	if result.BlockedByGuard != "destructive_action" {
		t.Fatalf("expected destructive guard, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedLLMReturnsErrorWhenZAIRequestFails(t *testing.T) {
	withFakeNextActionZAI(t, nextActionFakeZAIResponse{Status: http.StatusInternalServerError, Body: `{"error":{"message":"model unavailable"}}`})
	_, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: safeRecommendedMessage(),
	}, 0)
	if err == nil {
		t.Fatal("expected error when Z.AI request fails so caller can fall back to heuristic")
	}
}

func TestEvaluateNextActionAutoProceedLLMNoRecommendationDoesNotCallModel(t *testing.T) {
	requests := withFakeNextActionZAI(t, nextActionFakeZAIResponse{Content: `{"auto_proceed":true,"reason":"must not be called"}`})
	message := strings.Join([]string{
		"선택지:",
		"1. 해석 A로 구현합니다.",
		"2. 해석 B로 구현합니다.",
	}, "\n")
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: message,
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AutoProceed {
		t.Fatalf("no explicit recommendation must not auto-proceed, got %+v", result)
	}
	if got := atomic.LoadInt32(requests); got != 0 {
		t.Fatalf("no-recommendation path invoked LLM %d times", got)
	}
}

func TestBuildNextActionAutoProceedLLMPromptRendersSchemaAndChoices(t *testing.T) {
	candidates := ParseCandidates(safeRecommendedMessage())
	recommended := SelectRecommendedCandidate(candidates)
	if recommended == nil {
		t.Fatal("expected recommended candidate fixture")
	}
	prompt := BuildLLMPrompt(*recommended, candidates)
	for _, want := range []string{
		"cautious release-safety gate",
		"auto_proceed",
		"Recommended Next Action",
		"1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천) (recommended)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 멈춥니다.",
		"reason concisely justifies the decision",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("LLM prompt missing %q:\n%s", want, prompt)
		}
	}
}

type nextActionFakeZAIResponse struct {
	Status  int
	Body    string
	Content string
}

func withFakeNextActionZAI(t *testing.T, responses ...nextActionFakeZAIResponse) *int32 {
	t.Helper()
	if len(responses) == 0 {
		t.Fatal("missing fake Z.AI responses")
	}
	t.Setenv("Z_AI_API_KEY", "test-key")
	var requests int32
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
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
	return &requests
}
