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
