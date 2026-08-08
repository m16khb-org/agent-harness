package draftwikicli

import (
	draftwiki "agent-harness/internal/contract/draftwiki"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunProjectDraftWikiSuggest_printsPrompt_whenInputIsPositional(t *testing.T) {
	// Given
	root, input := draftWikiSuggestCLIFixture(t)

	// When
	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{
			"suggest",
			"--repo", root,
			input,
		})
	})

	// Then
	if !strings.Contains(out, "Host-Agent Judgement Response Schema") || !strings.Contains(out, "Hook policy should stay bookkeeping-only") {
		t.Fatalf("unexpected suggest prompt:\n%s", out)
	}
}

func TestRunProjectDraftWikiSuggest_printsPromptJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	root, input := draftWikiSuggestCLIFixture(t)

	// When
	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWikiSuggest([]string{
			"--repo", root,
			"--input", input,
			"--target-wiki", "agent-harness",
			"--target-type", "notes",
			"--json",
		})
	})

	// Then
	var result draftwiki.DraftWikiSuggestResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode suggest dry-run json: %v\n%s", err, out)
	}
	if !result.OK || result.Executed || result.Prompt == "" {
		t.Fatalf("unexpected suggest result: %+v", result)
	}
	if result.PromptBytes <= 0 || result.Draft != nil {
		t.Fatalf("suggest should build prompt without writing a draft: %+v", result)
	}
}

func TestRunProjectDraftWikiSuggest_returnsInputPathError_whenInputMissing(t *testing.T) {
	// Given
	root := t.TempDir()

	// When
	err := runProjectDraftWikiSuggest([]string{"--repo", root})

	// Then
	if err == nil || !strings.Contains(err.Error(), "input path is required") {
		t.Fatalf("expected input path error, got %v", err)
	}
}

func draftWikiSuggestCLIFixture(t *testing.T) (root, input string) {
	t.Helper()
	root = t.TempDir()
	input = filepath.Join(root, "memory.md")
	if err := os.WriteFile(input, []byte("Hook policy should stay bookkeeping-only.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, input
}
