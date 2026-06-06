package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakeAgyScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agy.sh")
	script := "#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func safeRecommendedMessage() string {
	return strings.Join([]string{
		"선택지:",
		"1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 멈춥니다.",
	}, "\n")
}

func TestEvaluateNextActionAutoProceedLLMAutoProceedsWhenLLMApproves(t *testing.T) {
	agy := writeFakeAgyScript(t, `printf '%s' '{"auto_proceed":true,"reason":"safe forward step"}'`)
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message:    safeRecommendedMessage(),
		AgyCommand: agy,
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
	agy := writeFakeAgyScript(t, `printf '%s' '{"auto_proceed":false,"reason":"needs user confirmation"}'`)
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message:    safeRecommendedMessage(),
		AgyCommand: agy,
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

func TestEvaluateNextActionAutoProceedLLMHardVetoesDestructiveWithoutCallingAgy(t *testing.T) {
	// Point agy at a script that fails the test if invoked, proving the destructive
	// hard-veto short-circuits before any external call.
	agy := writeFakeAgyScript(t, `echo "agy must not be called for destructive recommendations" >&2; exit 3`)
	message := strings.Join([]string{
		"선택지:",
		"1. 정리 진행: merged worktree와 branch를 삭제합니다. (추천)",
		"2. 보류: 유지합니다.",
		"3. 확장 정리: 전체를 점검합니다.",
	}, "\n")
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message:    message,
		AgyCommand: agy,
	}, 0)
	if err != nil {
		t.Fatalf("destructive hard-veto must not invoke agy or error, got %v", err)
	}
	if result.AutoProceed {
		t.Fatalf("destructive recommendation must never auto-proceed, got %+v", result)
	}
	if result.BlockedByGuard != "destructive_action" {
		t.Fatalf("expected destructive guard, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedLLMReturnsErrorWhenAgyMissing(t *testing.T) {
	_, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message:    safeRecommendedMessage(),
		AgyCommand: filepath.Join(t.TempDir(), "nonexistent-agy"),
	}, 0)
	if err == nil {
		t.Fatal("expected error when agy command is missing so caller can fall back to heuristic")
	}
}

func TestEvaluateNextActionAutoProceedLLMNoRecommendationDoesNotCallAgy(t *testing.T) {
	agy := writeFakeAgyScript(t, `echo "agy must not be called without a recommendation" >&2; exit 3`)
	message := strings.Join([]string{
		"선택지:",
		"1. 해석 A로 구현합니다.",
		"2. 해석 B로 구현합니다.",
	}, "\n")
	result, err := EvaluateNextActionAutoProceedLLM(NextActionAutoProceedLLMRequest{
		Message:    message,
		AgyCommand: agy,
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AutoProceed {
		t.Fatalf("no explicit recommendation must not auto-proceed, got %+v", result)
	}
}

func TestBuildNextActionAutoProceedLLMPromptRendersSchemaAndChoices(t *testing.T) {
	candidates := parseNextActionCandidates(safeRecommendedMessage())
	recommended := selectRecommendedNextAction(candidates)
	if recommended == nil {
		t.Fatal("expected recommended candidate fixture")
	}
	prompt := buildNextActionAutoProceedLLMPrompt(*recommended, candidates)
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
