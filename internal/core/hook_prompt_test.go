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
	for _, want := range []string{"project_docs_route", "api_doc_static_check", "api_doc_review", "OPEN_API_SPEC"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsForBugRecordsCaution(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 회귀 버그 고쳐줘"})
	if !strings.Contains(got.AdditionalContext, "project_docs_record") || !strings.Contains(got.AdditionalContext, "kind=caution") {
		t.Fatalf("expected caution record hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsOutputIsEnglish(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "새 endpoint와 DTO를 추가해줘"})
	for _, r := range got.AdditionalContext {
		if r >= 0xAC00 && r <= 0xD7A3 {
			t.Fatalf("expected injected prompt context to stay English, got Korean rune %q in:\n%s", r, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesArchitectureDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 대안을 설계해줘"})
	for _, want := range []string{"agent_harness project-doc routing hint", "ARCHITECTURE.md", "ADR.md", "project_docs_route"} {
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
	projectIndex := strings.Index(got.AdditionalContext, "agent_harness project-doc routing hint")
	companionIndex := strings.Index(got.AdditionalContext, "Secondary companion-tool hints")
	if projectIndex < 0 || companionIndex < 0 || projectIndex > companionIndex {
		t.Fatalf("expected project-doc routing before companion hints:\n%s", got.AdditionalContext)
	}
	if !strings.Contains(got.AdditionalContext, "CodeGraph") {
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
	for _, want := range []string{"Required project docs:", "Consider project docs:", "Route if ambiguous:", "ARCHITECTURE.md", "OPERATIONS.md", "TESTING.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("priority section missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	if strings.Index(got.AdditionalContext, "Required project docs:") > strings.Index(got.AdditionalContext, "Consider project docs:") {
		t.Fatalf("required docs should render before consider docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsAmbiguousPromptEmphasizesRoute(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘"})
	if !strings.Contains(got.AdditionalContext, "Route if ambiguous:") || strings.Contains(got.AdditionalContext, "Required project docs:") {
		t.Fatalf("ambiguous prompt should emphasize route without required docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsDoesNotTreatPRSubstringAsCommit(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "print output formatting을 확인해줘"})
	if strings.Contains(got.AdditionalContext, "COMMIT_POLICY.md") {
		t.Fatalf("substring pr should not trigger commit policy:\n%s", got.AdditionalContext)
	}
}
