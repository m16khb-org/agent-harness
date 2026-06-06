package core

import (
	"strings"
	"testing"
)

func TestEvaluateNextActionAutoProceedCoversJudgementAndGuardBranches(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. (추천) 다음 slice를 계속 구현하고 테스트",
		"2. 보류",
		"3. 삭제 후 reset --hard",
	}, "\n")
	result := EvaluateNextActionAutoProceed(message, 0)
	if !result.OK || !result.AgentJudgementRequired || result.AutoProceed {
		t.Fatalf("expected judgement-required result: %+v", result)
	}
	if result.SelectedIndex != 1 || result.TopScore < result.Threshold {
		t.Fatalf("unexpected selected action: %+v", result)
	}

	destructive := EvaluateNextActionAutoProceed(strings.Join([]string{
		"선택지:",
		"1. (추천) reset --hard 후 force push",
		"2. 계속 점검",
		"3. 보류",
	}, "\n"), 0.5)
	if destructive.BlockedByGuard != "destructive_action" || destructive.AgentJudgementRequired {
		t.Fatalf("expected destructive guard: %+v", destructive)
	}
}

func TestEvaluateNextActionAutoProceedCoversUserDecisionBranches(t *testing.T) {
	none := EvaluateNextActionAutoProceed("선택지:\n1. 계속\n2. 보류\n3. 점검", 0.8)
	if none.Reason != "no explicitly recommended next action; user decision required" {
		t.Fatalf("no recommendation result = %+v", none)
	}
	low := EvaluateNextActionAutoProceed("선택지:\n1. (추천) 아마도 검토 필요\n2. 계속\n3. 보류", 0.8)
	if low.AgentJudgementRequired || !strings.Contains(low.Reason, "below threshold") {
		t.Fatalf("low score result = %+v", low)
	}
	tooFew := EvaluateNextActionAutoProceed("no choices here", 0)
	if tooFew.Reason != "no numbered next-action choices to evaluate" {
		t.Fatalf("too few result = %+v", tooFew)
	}
}
