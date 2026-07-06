package nextaction

import (
	"strings"
	"testing"
)

func TestNumberedNextActionsDecisionBlocksMissingChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision("작업했습니다.", true, "stop")
	if got.Decision != "block" || !strings.Contains(got.Reason, "numbered next actions") {
		t.Fatalf("expected missing numbered next actions to block, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionAllowsChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
1. 진행: 다음 검증을 실행합니다. (추천)
2. 축소 진행: 작은 범위만 확인합니다.
3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected numbered choices to allow, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionBlocksMissingRecommendation(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
1. 진행: 다음 검증을 실행합니다.
2. 축소 진행: 작은 범위만 확인합니다.
3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "block" || !strings.Contains(got.Reason, "exactly three numbered options") {
		t.Fatalf("expected missing recommendation to block, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionBlocksMultipleRecommendations(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
1. 진행: 다음 검증을 실행합니다. (추천)
2. 축소 진행: 작은 범위만 확인합니다. (추천)
3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "block" || !strings.Contains(got.Reason, "exactly three numbered options") {
		t.Fatalf("expected multiple recommendations to block, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionAllowsMarkdownListChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
- 1. 진행: 다음 검증을 실행합니다. (추천)
* 2. 축소 진행: 작은 범위만 확인합니다.
+ 3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected markdown list numbered choices to allow, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionNoopsWhenDisabled(t *testing.T) {
	got := BuildNumberedNextActionsDecision("작업했습니다.", false, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected disabled guard to allow, got %+v", got)
	}
}

func TestBuildNextActionJudgementTriggerReportsRecommendedChoiceFacts(t *testing.T) {
	message := strings.Join([]string{
		"구현을 마쳤습니다.",
		"선택지:",
		"1. 진행: 다음 단계 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 먼저 검증합니다.",
		"3. 보류: 현재 상태로 멈추고 사용자 확인을 기다립니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if !result.ShouldReenterAgent {
		t.Fatalf("expected next-action facts to re-enter the main agent, got %+v", result)
	}
	if !result.ChoicesFound || result.ChoiceCount != 3 {
		t.Fatalf("expected three observed choices, got %+v", result)
	}
	if result.RecommendedCount != 1 || result.RecommendedIndex != 1 || result.RecommendedText == "" {
		t.Fatalf("expected exactly one recommended choice fact, got %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.Score != 0 || candidate.Destructive {
			t.Fatalf("trigger must not score choices or emit destructive verdicts, got %+v", result)
		}
	}
}

func TestBuildNextActionJudgementTriggerReportsDestructiveTextAsFactOnly(t *testing.T) {
	message := strings.Join([]string{
		"리뷰가 통과했습니다.",
		"선택지:",
		"1. 진행: PR을 머지하고 이슈를 닫습니다. (추천)",
		"2. 보류: 추가 확인을 기다립니다.",
		"3. 축소: 일부만 merge 합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent {
		t.Fatalf("expected hook to relay facts to the main agent, got %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.Destructive || candidate.Score != 0 {
			t.Fatalf("hook trigger must not judge destructive text or score choices, got %+v", result)
		}
	}
}

func TestBuildNextActionJudgementTriggerReportsMissingRecommendationAsFact(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 해석 A로 구현합니다.",
		"2. 해석 B로 구현합니다.",
		"3. 해석 C로 구현합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent || result.RecommendedCount != 0 {
		t.Fatalf("expected missing recommendation to be relayed as facts, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerReportsMultipleRecommendationsAsFact(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 진행: 구현합니다. (추천)",
		"2. 검증: 테스트합니다. (추천)",
		"3. 보류: 멈춥니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent || result.RecommendedCount != 2 {
		t.Fatalf("expected multiple recommendations to be relayed as facts, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerDoesNotParseExplanatoryNumberedText(t *testing.T) {
	message := strings.Join([]string{
		"설명입니다.",
		"1. Stop hook이 먼저 정적 휴리스틱으로 자동진행 후보를 판정합니다.",
		"2. 메인 에이전트는 실행 여부만 판단합니다.",
		"3. `agent-harness`가 추천 선택지를 분석해서 자동진행 후보라고 판단합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ShouldReenterAgent || result.ChoicesFound {
		t.Fatalf("numbered explanation without 선택지 header must not trigger, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerNoChoicesDoesNotTrigger(t *testing.T) {
	result := BuildNextActionJudgementTrigger("작업을 완료했습니다.")
	if result.ShouldReenterAgent || result.ChoicesFound {
		t.Fatalf("message without choices must not trigger, got %+v", result)
	}
}

func TestBuildNextActionJudgementRelayReasonRequiresOneDecision(t *testing.T) {
	trigger := BuildNextActionJudgementTrigger(strings.Join([]string{
		"선택지:",
		"1. 진행: 다음 foldering slice를 계속합니다. (추천)",
		"2. 리뷰: 현재 diff를 확인합니다.",
		"3. 보류: 멈춥니다.",
	}, "\n"))
	reason := BuildJudgementRelayReason(trigger)
	for _, want := range []string{
		"훅은 안전성, 가역성, 사용자 의도 정합성, 진행 여부를 판단하지 않습니다",
		"한 번에 하나의 판단만 하세요",
		"둘을 같은 답변에서 섞지 마세요",
		"no-auto-proceed 판단을 남겼다면 같은 작업을 자동 goal continuation으로 재개하지 마세요",
		"자동진행 결과 보고에는 `선택지:` 3개와 정확히 하나의 `(추천)`",
		"자동진행하지 않음 판단에는 선택지 블록을 다시 붙이지 마세요",
	} {
		if !strings.Contains(reason, want) {
			t.Fatalf("judgement relay reason missing %q:\n%s", want, reason)
		}
	}
}

func TestNumberedNextActionsDecisionIgnoresBareRecommendWordInOptionText(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
1. 추천 로직 리뷰를 진행합니다. (추천)
2. 추천 마커 파서만 손봅니다.
3. 보류: 멈춥니다.`, true, "stop")
	if got.Decision != "allow" {
		t.Fatalf("bare 추천 word in option text must not count as a marker, got %+v", got)
	}
}

func TestNextActionIsRecommendedRequiresExplicitMarker(t *testing.T) {
	for text, want := range map[string]bool{
		"진행합니다. (추천)":             true,
		"proceed (Recommended)":   true,
		"추천 로직을 검토합니다":            false,
		"이 방식은 추천하지 않습니다":         false,
		"this is not recommended": false,
	} {
		if got := nextActionIsRecommended(text); got != want {
			t.Fatalf("nextActionIsRecommended(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestParseNextActionSectionClosesAtProseAfterCandidates(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 진행: 구현합니다. (추천)",
		"2. 검증: 테스트만 돌립니다.",
		"3. 보류: 멈춥니다.",
		"",
		"참고한 문서:",
		"1. ADR.md",
		"2. CAUTIONS.md",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ChoiceCount != 3 {
		t.Fatalf("numbered list after the choices block must not pollute candidates, got %+v", result)
	}
	if got := BuildNumberedNextActionsDecision(message, true, "stop"); got.Decision != "allow" {
		t.Fatalf("well-formed choices followed by another numbered list must allow, got %+v", got)
	}
}

func TestParseNextActionIgnoresDecimalNumbers(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 진행: 구현합니다. (추천)",
		"2. 검증: 1.5배 커버리지 목표로 테스트합니다.",
		"3. 보류: 멈춥니다.",
		"",
		"성능은 1.8배 개선되었습니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ChoiceCount != 3 {
		t.Fatalf("decimal numbers must not parse as candidates, got %+v", result)
	}
}

func TestParseNextActionLastSectionWins(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 옛 선택지 A (추천)",
		"2. 옛 선택지 B",
		"3. 옛 선택지 C",
		"",
		"정정합니다.",
		"",
		"선택지:",
		"1. 새 선택지 A",
		"2. 새 선택지 B (추천)",
		"3. 새 선택지 C",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ChoiceCount != 3 || result.RecommendedIndex != 2 {
		t.Fatalf("the last choices section must win, got %+v", result)
	}
}

func TestParseNextActionRecognizesMarkdownHeadingSectionHeader(t *testing.T) {
	for _, header := range []string{"## 선택지:", "**선택지:**", "### Options:"} {
		message := strings.Join([]string{
			header,
			"1. 진행: 구현합니다. (추천)",
			"2. 검증: 테스트합니다.",
			"3. 보류: 멈춥니다.",
		}, "\n")
		if got := BuildNumberedNextActionsDecision(message, true, "stop"); got.Decision != "allow" {
			t.Fatalf("header %q must be recognized, got %+v", header, got)
		}
	}
}

func TestNumberedNextActionsDecisionBlocksMoreThanThreeChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`선택지:
1. 진행합니다. (추천)
2. 검증합니다.
3. 보류합니다.
4. 문서화합니다.`, true, "stop")
	if got.Decision != "block" {
		t.Fatalf("four options must block (exactly three required), got %+v", got)
	}
}

func TestParseNextActionKeepsFirstDuplicateIndex(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 첫 번째 (추천)",
		"1. 중복 번호",
		"2. 두 번째",
		"3. 세 번째",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ChoiceCount != 3 {
		t.Fatalf("duplicate index must keep the first occurrence only, got %+v", result)
	}
	if result.Candidates[0].Text != "첫 번째 (추천)" {
		t.Fatalf("first occurrence must win, got %+v", result.Candidates)
	}
}

func TestIsNoAutoProceedJudgementRequiresLineStartMarker(t *testing.T) {
	for message, want := range map[string]bool{
		"자동진행하지 않음 판단입니다.\n사용자 결정이 필요합니다.":            true,
		"자동진행하지 않겠습니다.\n\n판단 근거: 파괴적 작업입니다.":          true,
		"**자동진행하지 않음** - 사용자 결정 필요":                   true,
		"no-auto-proceed: this needs a user decision": true,
		"이 훅은 자동진행하지 않음 판단을 요구한다는 룰을 설명하는 리뷰입니다.":     false,
		"릴레이 문구에는 no-auto-proceed 라는 표현이 들어 있습니다.":    false,
		"": false,
	} {
		if got := IsNoAutoProceedJudgement(message); got != want {
			t.Fatalf("IsNoAutoProceedJudgement(%q) = %v, want %v", message, got, want)
		}
	}
}

func TestBuildJudgementRelayReasonInstructsLineStartMarker(t *testing.T) {
	trigger := BuildNextActionJudgementTrigger(strings.Join([]string{
		"선택지:",
		"1. 진행합니다. (추천)",
		"2. 검증합니다.",
		"3. 보류합니다.",
	}, "\n"))
	reason := BuildJudgementRelayReason(trigger)
	if !strings.Contains(reason, "`자동진행하지 않") {
		t.Fatalf("relay reason must instruct the line-start marker, got:\n%s", reason)
	}
}
