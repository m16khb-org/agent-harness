package nextaction

import (
	"strings"
	"testing"
)

func safeRecommendedMessage() string {
	return strings.Join([]string{
		"선택지:",
		"1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 멈춥니다.",
	}, "\n")
}

func TestEvaluateNextActionAutoProceedLLMReturnsRemovedGateError(t *testing.T) {
	_, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message: safeRecommendedMessage(),
	}, 0)
	if err == nil || !strings.Contains(err.Error(), "external LLM gate was removed") {
		t.Fatalf("expected removed-gate error, got %v", err)
	}
}

func TestEvaluateNextActionAutoProceedLLMHardVetoesDestructiveWithoutCallingModel(t *testing.T) {
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
		t.Fatalf("destructive hard-veto must not invoke external judgement or error, got %v", err)
	}
	if result.AutoProceed {
		t.Fatalf("destructive recommendation must never auto-proceed, got %+v", result)
	}
	if result.BlockedByGuard != "destructive_action" {
		t.Fatalf("expected destructive guard, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedLLMNoRecommendationDoesNotCallModel(t *testing.T) {
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
