package prompt_test

import (
	"agent-harness/internal/adapter/draftwiki"
	"agent-harness/internal/adapter/issueops"
	"agent-harness/internal/core/commitsuggest"
	"agent-harness/internal/core/lintdiagnose"
	"agent-harness/internal/domain/prompt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreHostJudgementPromptsUseStructuredContract(t *testing.T) {
	prompts := map[string]string{
		"commit": commitsuggest.BuildPrompt("diff --git a/file b/file\n"),
		"draft_wiki": draftwiki.BuildDraftWikiSuggestPrompt(draftwiki.DraftWikiSuggestRequest{
			Title:      "Prompt contract",
			TargetWiki: "dev-fundamentals",
		}, "source material", "notes"),
		"issueops_judge": mustIssueOpsJudgePromptForTest(t),
		"lint_diagnose":  lintdiagnose.BuildPrompt(1, "failure output"),
	}
	for name, promptText := range prompts {
		for _, heading := range prompt.StructuredPromptSectionHeadings {
			if !strings.Contains(promptText, heading) {
				t.Fatalf("%s prompt missing %q:\n%s", name, heading, promptText)
			}
		}
		for _, want := range []string{"## Host-Agent Judgement Response Schema", "Example:", "Return exactly one JSON object"} {
			if !strings.Contains(promptText, want) {
				t.Fatalf("%s prompt missing schema contract %q:\n%s", name, want, promptText)
			}
		}
	}
}

func TestProjectBootstrapPromptUsesStructuredContract(t *testing.T) {
	promptPath := filepath.Join("..", "..", "..", "skills", "project-bootstrap", "PROMPT.md")
	b, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	promptText := string(b)
	for _, heading := range prompt.StructuredPromptSectionHeadings {
		if !strings.Contains(promptText, heading) {
			t.Fatalf("project bootstrap prompt missing %q:\n%s", heading, promptText)
		}
	}
	for _, want := range []string{"project_docs_route", "project_docs_read", "project_docs_update", "project_docs_record", "Do not invent", "Completion criteria"} {
		if !strings.Contains(promptText, want) {
			t.Fatalf("project bootstrap prompt missing %q:\n%s", want, promptText)
		}
	}
}

func mustIssueOpsJudgePromptForTest(t *testing.T) string {
	t.Helper()
	promptText, err := issueops.BuildIssueOpsLLMJudgePrompt(
		issueops.IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "context", CriticalFailures: []string{"critical"}},
		issueops.IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return promptText
}
