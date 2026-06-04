package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildUserPromptMCPHintsInjectsNextActionPolicy(t *testing.T) {
	// The next-action / auto-proceed policy replaces the external-LLM gate: it is
	// injected into every non-empty user prompt so the main agent frames its own
	// turn-ending choices for the cheap heuristic Stop gate to act on.
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "아무 작업이나 진행해줘"})
	if !got.OK || !got.ShouldInject {
		t.Fatalf("expected next-action policy to inject for a non-empty prompt: %+v", got)
	}
	for _, want := range []string{"next-action:", "선택지:", "(추천)", "main agent", "context", "destructive", "user confirmation"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("next-action policy missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsForAPIWork(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
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
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "문제 파악부터 GitLab 이슈, 구현 계획, TDD, 피드백 루프, MR까지 진행해줘"})
	for _, want := range []string{"issueops", "issue-driven workflow", "hooks must not create issues or PRs"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("issueops hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsForBugRecordsCaution(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 회귀 버그 고쳐줘"})
	if !strings.Contains(got.AdditionalContext, "record reusable caution") {
		t.Fatalf("expected caution record hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsUsesCompactBanner(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
	for _, want := range []string{"[agent-harness]", "routing hint", "docs:"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("compact banner missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	for _, verbose := range []string{"\n", "╭", "Before acting", "Required project docs:", "Writable tools must preserve"} {
		if strings.Contains(got.AdditionalContext, verbose) {
			t.Fatalf("compact hook context should not include verbose prose %q:\n%s", verbose, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsDropsKeywordDocPrescription(t *testing.T) {
	// Architecture keywords used to prescribe required:/consider: docs. That
	// verdict is removed; doc choice is left to the agent via the catalog.
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 대안을 설계해줘"})
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
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 symbol의 call graph와 impact를 codegraph로 확인해줘"})
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
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "지난번에 이미 해결한 memory 찾아줘"})
	if !strings.Contains(got.AdditionalContext, "memory: use claude-mem only for previous-session/repeated-work recall") {
		t.Fatalf("expected claude-mem secondary hint:\n%s", got.AdditionalContext)
	}
	if strings.Contains(got.AdditionalContext, "agentmemory") {
		t.Fatalf("memory routing must not mention agentmemory:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsRoutesLLMReviewToAgyWhenEnabled(t *testing.T) {
	disabled := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 계획을 검토하고 개선점을 분석해줘"})
	if strings.Contains(disabled.AdditionalContext, "agy -p") {
		t.Fatalf("agy hint should be opt-in:\n%s", disabled.AdditionalContext)
	}
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 계획을 검토하고 개선점을 분석해줘", EnableAgyHints: true})
	if !strings.Contains(got.AdditionalContext, "agy -p for LLM second-pass review") {
		t.Fatalf("expected agy secondary hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsEmptyPromptDoesNotInject(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "   "})
	if got.ShouldInject || got.AdditionalContext != "" || len(got.Hints) != 0 {
		t.Fatalf("expected no injection for empty prompt: %+v", got)
	}
}

func TestBuildUserPromptMCPHintsAmbiguousPromptEmphasizesRoute(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘"})
	if !strings.Contains(got.AdditionalContext, "docs:") || strings.Contains(got.AdditionalContext, "required:") {
		t.Fatalf("ambiguous prompt should emphasize route without required docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsDoesNotTreatPRSubstringAsCommit(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "print output formatting을 확인해줘"})
	if strings.Contains(got.AdditionalContext, "COMMIT_POLICY.md") {
		t.Fatalf("substring pr should not trigger commit policy:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsIncludesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := InitProjectLifecycleState(root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(root, DocUpkeepEvent{
		Kind:       "operation_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "Hook behavior changed.",
		Source:     "test",
	}); err != nil {
		t.Fatal(err)
	}
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "계속", Repo: root})
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
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘", Repo: root})
	for _, want := range []string{"profile:", "gitlab/self-hosted@gitlab.example.internal", "JavaScript/TypeScript", "backend", "frontend", "fullstack"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("profile hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}
