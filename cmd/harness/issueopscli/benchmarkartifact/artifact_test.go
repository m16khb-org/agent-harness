package benchmarkartifact

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"strings"
	"testing"
)

func TestDefaultsForNoExtraRequirementsFixture(t *testing.T) {
	artifact := FromFixture(issueopscontract.IssueOpsBenchmarkFixture{
		ID:          "empty-requirements",
		Title:       "Fallback title",
		UserPrompt:  "   ",
		RepoContext: "  minimal repo context  ",
	})

	for _, want := range []string{
		"요청 요약: Fallback title",
		"저장소 맥락: minimal repo context",
		"- 해당 fixture의 추가 요구사항 없음",
		"- Worker Fixture owns verification that this fixture has no additional task requirements.",
		"Fixture: `empty-requirements` - Fallback title",
	} {
		if !strings.Contains(strings.Join([]string{
			artifact.ProblemSummary,
			artifact.IssueDraft,
			artifact.Plan,
			artifact.TDDPlan,
			artifact.TaskBreakdown,
			artifact.SubagentPrompts,
			artifact.PRDraft,
		}, "\n"), want) {
			t.Fatalf("artifact missing default text %q", want)
		}
	}
}

// FromFixture must forward the recorded routing trace so the deterministic
// benchmark scores skill_routing_fidelity; without this the dim would be N/A
// even for a fixture that declares ExpectedRouting (A5).
func TestFromFixtureForwardsRoutingTrace(t *testing.T) {
	withRouting := FromFixture(issueopscontract.IssueOpsBenchmarkFixture{
		ID:              "routing",
		ExpectedRouting: []issueopscontract.SkillRouting{{Phase: "plan", Skill: "codd"}},
	})
	if len(withRouting.RoutingTrace) != 1 || withRouting.RoutingTrace[0] != (issueopscontract.SkillRouting{Phase: "plan", Skill: "codd"}) {
		t.Fatalf("FromFixture must synthesize RoutingTrace from ExpectedRouting, got %+v", withRouting.RoutingTrace)
	}
	if got := FromFixture(issueopscontract.IssueOpsBenchmarkFixture{ID: "no-routing"}); len(got.RoutingTrace) != 0 {
		t.Fatalf("fixture without expected_routing must get empty RoutingTrace, got %+v", got.RoutingTrace)
	}
}

func TestListHelpersSkipBlankItems(t *testing.T) {
	if got := Bullets([]string{" ", "\t"}); got != "- 해당 fixture의 추가 요구사항 없음" {
		t.Fatalf("blank bullet fallback = %q", got)
	}
	if got := Bullets([]string{" first ", "", "second"}); got != "- first\n- second" {
		t.Fatalf("trimmed bullets = %q", got)
	}
	if got := OwnedTasks([]string{" ", ""}); got != "- Worker Fixture owns verification that this fixture has no additional task requirements." {
		t.Fatalf("blank task fallback = %q", got)
	}
	want := "- Worker Fixture-1 owns schema validation and reports test evidence for that task.\n- Worker Fixture-3 owns cli output and reports test evidence for that task."
	if got := OwnedTasks([]string{"schema validation", "", " cli output "}); got != want {
		t.Fatalf("trimmed owned tasks = %q", got)
	}
}
