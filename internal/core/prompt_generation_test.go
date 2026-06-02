package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreLLMPromptsUseStructuredContract(t *testing.T) {
	prompts := map[string]string{
		"commit": buildCommitSuggestPrompt("diff --git a/file b/file\n"),
		"draft_wiki": buildDraftWikiSuggestPrompt(DraftWikiSuggestRequest{
			Title:      "Prompt contract",
			TargetWiki: "dev-fundamentals",
		}, "source material", "gemini-3.5-flash", "notes"),
		"issueops_judge": mustIssueOpsJudgePromptForTest(t),
		"lint_diagnose":  buildLintDiagnosePrompt(1, "failure output"),
	}
	for name, prompt := range prompts {
		for _, heading := range StructuredPromptSectionHeadings {
			if !strings.Contains(prompt, heading) {
				t.Fatalf("%s prompt missing %q:\n%s", name, heading, prompt)
			}
		}
	}
}

func TestProjectBootstrapPromptUsesStructuredContract(t *testing.T) {
	promptPath := filepath.Join("..", "..", "skills", "project-bootstrap", "PROMPT.md")
	b, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(b)
	for _, heading := range StructuredPromptSectionHeadings {
		if !strings.Contains(prompt, heading) {
			t.Fatalf("project bootstrap prompt missing %q:\n%s", heading, prompt)
		}
	}
	for _, want := range []string{"project_docs_route", "project_docs_read", "project_docs_update", "project_docs_record", "Do not invent", "Completion criteria"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("project bootstrap prompt missing %q:\n%s", want, prompt)
		}
	}
}

func mustIssueOpsJudgePromptForTest(t *testing.T) string {
	t.Helper()
	prompt, err := buildIssueOpsAgyJudgePrompt(
		IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "context", CriticalFailures: []string{"critical"}},
		IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return prompt
}
