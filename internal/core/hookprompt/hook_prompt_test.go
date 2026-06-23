package hookprompt_test

import (
	core "agent-harness/internal/core"
	"agent-harness/internal/core/hookprompt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildUserPromptMCPHintsInjectsConciseNextActionReminder(t *testing.T) {
	// UserPromptSubmit must stay compact because Codex renders additionalContext
	// inline in the hook transcript. The full policy belongs to Stop relay.
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "아무 작업이나 진행해줘"})
	if !got.OK || !got.ShouldInject {
		t.Fatalf("expected next-action reminder to inject for a non-empty prompt: %+v", got)
	}
	for _, want := range []string{"next-action:", "3 choices/1 recommendation", "Stop hook relays full decision details"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("next-action reminder missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	for _, gone := range []string{"The Stop hook may re-enter", "Auto-proceed result reports must still end with choices", "No-auto-proceed judgements must stop without adding another choices block", "make exactly one decision", "never both in the same answer"} {
		if strings.Contains(got.AdditionalContext, gone) {
			t.Fatalf("UserPromptSubmit must not embed verbose next-action policy %q:\n%s", gone, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsLeavesJudgementDetailsToStopRelay(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "작업을 마무리해줘"})
	for _, gone := range []string{"State the auto-proceed or no-auto-proceed rationale", "no-auto-proceed judgement is sticky", "recommended option is continued implementation"} {
		if strings.Contains(got.AdditionalContext, gone) {
			t.Fatalf("UserPromptSubmit must not carry Stop relay judgement detail %q:\n%s", gone, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsKeepsAutoContinuationContextShort(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "foldering slice가 모두 완료될 때까지 자동진행"})
	if !strings.Contains(got.AdditionalContext, "next-action:") {
		t.Fatalf("expected short next-action reminder:\n%s", got.AdditionalContext)
	}
	if len(got.AdditionalContext) > 500 {
		t.Fatalf("UserPromptSubmit context is too long for Codex transcript (%d chars):\n%s", len(got.AdditionalContext), got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsDoesNotInjectStickyNoAutoProceedPolicy(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "계속 진행"})
	for _, gone := range []string{"no-auto-proceed", "sticky", "goal continuation", "explicit user choice"} {
		if strings.Contains(got.AdditionalContext, gone) {
			t.Fatalf("sticky no-auto-proceed policy belongs to Stop relay, found %q:\n%s", gone, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsInjectsConciseDraftWikiReminder(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "훅 정책을 개선해줘"})
	for _, want := range []string{"draft-wiki:", "main-agent judgement only"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("draft-wiki reminder missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	for _, gone := range []string{"agent-harness project draft-wiki queue", "<<'EOF'", "heuristics must not queue"} {
		if strings.Contains(got.AdditionalContext, gone) {
			t.Fatalf("UserPromptSubmit must not embed verbose draft-wiki policy %q:\n%s", gone, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsForAPIWork(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
	if !got.OK || !got.ShouldInject {
		t.Fatalf("expected injected hint: %+v", got)
	}
	// API keywords now route only to actions, never to a required/consider doc verdict.
	for _, want := range []string{"use project docs only when repo-specific context matters", "check OpenAPI gaps", "review API error contract"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesProblemToPR(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "문제 파악부터 GitLab 이슈, 구현 계획, TDD, 피드백 루프, MR까지 진행해줘"})
	for _, want := range []string{"issueops", "issue-driven workflow", "hooks must not create issues or PRs"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("issueops hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsForBugRecordsCaution(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이 회귀 버그 고쳐줘"})
	if !strings.Contains(got.AdditionalContext, "record reusable caution") {
		t.Fatalf("expected caution record hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsUsesCompactBanner(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
	for _, want := range []string{"[agent-harness]\n", "- docs:", "- actions:", "- next-action:", "- draft-wiki:", "- rule:"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("compact multiline context missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	for _, verbose := range []string{" | ", "╭", "Before acting", "Required project docs:", "Writable tools must preserve"} {
		if strings.Contains(got.AdditionalContext, verbose) {
			t.Fatalf("compact hook context should not include verbose prose %q:\n%s", verbose, got.AdditionalContext)
		}
	}
}

func TestRenderHookMCPHintContextNormalizesFallbackLabels(t *testing.T) {
	got := hookprompt.RenderHookMCPHintContext([]hookprompt.HookUserPromptHint{
		{Tool: "project_docs_route"},
		{Tool: "project_docs_record", Reason: "kind=adr"},
		{Tool: "project_docs_record", Reason: "kind=caution"},
		{Tool: "api_doc_static_check"},
		{Tool: "CodeGraph"},
		{Tool: "CodeGraph"},
	}, nil, nil, "")
	for _, want := range []string{
		"- docs: use project docs only when repo-specific context matters",
		"- actions: record ADR decision, record reusable caution, check OpenAPI gaps",
		"- secondary: CodeGraph for structural lookup; rg for exact strings",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("compact render missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "CodeGraph for structural lookup") != 1 {
		t.Fatalf("expected duplicate fallback labels to collapse:\n%s", got)
	}
}

func TestBuildUserPromptMCPHintsDropsKeywordDocPrescription(t *testing.T) {
	// Architecture keywords used to prescribe required:/consider: docs. That
	// verdict is removed; doc choice is left to the agent via the catalog.
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "hook 구조와 대안을 설계해줘"})
	for _, gone := range []string{"required:", "consider:", "ARCHITECTURE.md", "ADR.md"} {
		if strings.Contains(got.AdditionalContext, gone) {
			t.Fatalf("keyword doc prescription should be gone, found %q:\n%s", gone, got.AdditionalContext)
		}
	}
	// Tool/action routing for the same keywords still works.
	if !strings.Contains(got.AdditionalContext, "record ADR decision") {
		t.Fatalf("expected ADR record action to survive:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsCompanionToolsStaySecondary(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이 symbol의 call graph와 impact를 codegraph로 확인해줘"})
	projectIndex := strings.Index(got.AdditionalContext, "[agent-harness]")
	companionIndex := strings.Index(got.AdditionalContext, "secondary")
	if projectIndex < 0 || companionIndex < 0 || projectIndex > companionIndex {
		t.Fatalf("expected project banner before companion hints:\n%s", got.AdditionalContext)
	}
	if !strings.Contains(got.AdditionalContext, "CodeGraph for structural lookup; rg for exact strings") {
		t.Fatalf("expected CodeGraph secondary hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsRoutesMemoryToClaudeMem(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "지난번에 이미 해결한 memory 찾아줘"})
	if !strings.Contains(got.AdditionalContext, "memory: use claude-mem only for previous-session/repeated-work recall") {
		t.Fatalf("expected claude-mem secondary hint:\n%s", got.AdditionalContext)
	}
	if strings.Contains(got.AdditionalContext, "agentmemory") {
		t.Fatalf("memory routing must not mention agentmemory:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsRoutesLLMReviewWhenEnabled(t *testing.T) {
	disabled := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이 계획을 검토하고 개선점을 분석해줘"})
	if strings.Contains(disabled.AdditionalContext, "Z.AI glm-5-turbo") {
		t.Fatalf("LLM hint should be opt-in:\n%s", disabled.AdditionalContext)
	}
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이 계획을 검토하고 개선점을 분석해줘", EnableLLMHints: true})
	if !strings.Contains(got.AdditionalContext, "Z.AI glm-5-turbo for LLM second-pass review") {
		t.Fatalf("expected LLM secondary hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsEmptyPromptDoesNotInject(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "   "})
	if got.ShouldInject || got.AdditionalContext != "" || len(got.Hints) != 0 {
		t.Fatalf("expected no injection for empty prompt: %+v", got)
	}
}

func TestBuildUserPromptMCPHintsAmbiguousPromptEmphasizesRoute(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이거 좀 개선해줘"})
	if !strings.Contains(got.AdditionalContext, "docs:") || strings.Contains(got.AdditionalContext, "required:") {
		t.Fatalf("ambiguous prompt should emphasize route without required docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsDoesNotTreatPRSubstringAsCommit(t *testing.T) {
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "print output formatting을 확인해줘"})
	if strings.Contains(got.AdditionalContext, "COMMIT_POLICY.md") {
		t.Fatalf("substring pr should not trigger commit policy:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsIncludesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := core.InitProjectLifecycleState(root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := core.AppendDocUpkeepEvent(root, core.DocUpkeepEvent{
		Kind:       "operation_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "Hook behavior changed.",
		Source:     "test",
	}); err != nil {
		t.Fatal(err)
	}
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "계속", Repo: root})
	for _, want := range []string{"pending upkeep:", "OPERATIONS.md", "Hook behavior changed.", "refresh project docs only if evidence changed"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("pending upkeep hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsIncludesProjectProfile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[remote \"origin\"]\n\turl = git@gitlab.example.internal:group/app.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"react":"latest","express":"latest"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	got := hookprompt.BuildUserPromptMCPHints(hookprompt.HookUserPromptRequest{Prompt: "이거 좀 개선해줘", Repo: root})
	for _, want := range []string{"profile:", "gitlab/self-hosted@gitlab.example.internal", "JavaScript/TypeScript", "backend", "frontend", "fullstack"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("profile hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}
