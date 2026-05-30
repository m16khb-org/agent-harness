package core

import (
	"strings"
	"testing"
)

func TestBuildUserPromptMCPHintsForAPIWork(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
	if !got.OK || !got.ShouldInject {
		t.Fatalf("expected injected hint: %+v", got)
	}
	for _, want := range []string{"choose project docs if ambiguous", "check OpenAPI gaps", "review API error contract", "OPEN_API_SPEC"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("hint missing %q:\n%s", want, got.AdditionalContext)
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
	for _, want := range []string{"[agent-harness]", "프로젝트 지침 확인 중", "required:", "route:"} {
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

func TestBuildUserPromptMCPHintsRoutesArchitectureDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 대안을 설계해줘"})
	for _, want := range []string{"[agent-harness]", "ARCHITECTURE.md", "ADR.md", "choose project docs if ambiguous"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("architecture doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesOperationsDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "install hook와 daemon 운영 경로를 고쳐줘"})
	for _, want := range []string{"OPERATIONS.md", "CONVENTIONS.md", "TECH_STACK.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("operations doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesTestingDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "golden test와 verification을 추가해줘"})
	for _, want := range []string{"TESTING.md", "AGENT_WORKFLOW.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("testing doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsCompanionToolsStaySecondary(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 symbol의 call graph와 impact를 codegraph로 확인해줘"})
	projectIndex := strings.Index(got.AdditionalContext, "[agent-harness]")
	companionIndex := strings.Index(got.AdditionalContext, "secondary")
	if projectIndex < 0 || companionIndex < 0 || projectIndex > companionIndex {
		t.Fatalf("expected project-doc routing before companion hints:\n%s", got.AdditionalContext)
	}
	if !strings.Contains(got.AdditionalContext, "CodeGraph for symbol/call-impact lookup") {
		t.Fatalf("expected CodeGraph secondary hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsEmptyPromptDoesNotInject(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "   "})
	if got.ShouldInject || got.AdditionalContext != "" || len(got.Hints) != 0 {
		t.Fatalf("expected no injection for empty prompt: %+v", got)
	}
}

func TestBuildUserPromptMCPHintsUsesPrioritySections(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 테스트 검증을 설계해줘"})
	for _, want := range []string{"required:", "consider:", "route:", "ARCHITECTURE.md", "OPERATIONS.md", "TESTING.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("priority section missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	if strings.Index(got.AdditionalContext, "required:") > strings.Index(got.AdditionalContext, "consider:") {
		t.Fatalf("required docs should render before consider docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsAmbiguousPromptEmphasizesRoute(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘"})
	if !strings.Contains(got.AdditionalContext, "route:") || strings.Contains(got.AdditionalContext, "required:") {
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

func TestBuildUserPromptMCPHintsFallsBackWhenLifecycleStateMissing(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 운영을 고쳐줘", Repo: t.TempDir()})
	for _, want := range []string{"OPERATIONS.md", "CONVENTIONS.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("fallback hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}
