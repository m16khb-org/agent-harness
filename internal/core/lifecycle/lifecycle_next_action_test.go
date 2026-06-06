package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestRecordStopNextActionRelaySuppressesWhenStateCannotPersist(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile ProjectLifecycleProfile
	b, err := os.ReadFile(written.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Fingerprint.RepoRoot = filepath.Join(t.TempDir(), "other")
	b, err = json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written.ProjectJSONPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	trigger := BuildNextActionJudgementTrigger(strings.Join([]string{
		"선택지:",
		"1. (추천) 사용자가 외부 담당자에게 문의문을 전달하고 답변을 공유한다.",
		"2. 임시 조치 변경안을 검토하라고 지시한다.",
		"3. 식별자 분리 설계안을 검토하라고 지시한다.",
	}, "\n"))
	got := RecordStopNextActionRelay(repo, trigger)
	if got.ShouldRelay || !containsString(got.Warnings, "project_lifecycle_namespace_mismatch") {
		t.Fatalf("expected unpersistable Stop relay to fail closed, got %+v", got)
	}
}
