package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestRunProjectDraftWikiSuggest_printsDryRunText_whenInputIsPositional(t *testing.T) {
	// Given
	root, input, settings := draftWikiSuggestCLIFixture(t)

	// When
	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{
			"suggest",
			"--repo", root,
			"--agy-settings", settings,
			"--dry-run",
			input,
		})
	})

	// Then
	if !strings.Contains(out, "draft-wiki suggest dry-run:") || !strings.Contains(out, `model="Gemini 3.5 Flash (High)"`) {
		t.Fatalf("unexpected suggest dry-run text:\n%s", out)
	}
	if !strings.Contains(out, "prompt_bytes=") {
		t.Fatalf("dry-run text should include prompt size:\n%s", out)
	}
}

func TestRunProjectDraftWikiSuggest_printsDryRunJSON_whenJSONFlagIsSet(t *testing.T) {
	// Given
	root, input, settings := draftWikiSuggestCLIFixture(t)

	// When
	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWikiSuggest([]string{
			"--repo", root,
			"--input", input,
			"--target-wiki", "agent-harness",
			"--target-type", "notes",
			"--agy-settings", settings,
			"--agy-model", "Gemini 3.5 Flash (High)",
			"--dry-run",
			"--json",
		})
	})

	// Then
	var result core.DraftWikiSuggestResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode suggest dry-run json: %v\n%s", err, out)
	}
	if !result.OK || !result.DryRun || result.Write || result.Executed {
		t.Fatalf("unexpected suggest dry-run result: %+v", result)
	}
	if result.Kind != "draft_wiki_suggest" || result.AgyModel != "Gemini 3.5 Flash (High)" {
		t.Fatalf("unexpected suggest metadata: %+v", result)
	}
	if result.PromptBytes <= 0 || result.Draft != nil {
		t.Fatalf("dry-run should build prompt without writing a draft: %+v", result)
	}
}

func TestRunProjectDraftWikiSuggest_returnsInputPathError_whenInputMissing(t *testing.T) {
	// Given
	root := t.TempDir()

	// When
	err := runProjectDraftWikiSuggest([]string{"--repo", root, "--dry-run"})

	// Then
	if err == nil || !strings.Contains(err.Error(), "input path is required") {
		t.Fatalf("expected input path error, got %v", err)
	}
}

func TestRunProjectDraftWikiSuggest_returnsModelMismatch_whenAgyModelDiffers(t *testing.T) {
	// Given
	root, input, settings := draftWikiSuggestCLIFixture(t)

	// When
	err := runProjectDraftWikiSuggest([]string{
		"--repo", root,
		"--input", input,
		"--agy-settings", settings,
		"--agy-model", "Claude Opus 4.6 (Thinking)",
		"--dry-run",
	})

	// Then
	if err == nil || !strings.Contains(err.Error(), "agy model mismatch") {
		t.Fatalf("expected model mismatch error, got %v", err)
	}
}

func draftWikiSuggestCLIFixture(t *testing.T) (root, input, settings string) {
	t.Helper()
	root = t.TempDir()
	input = filepath.Join(root, "memory.md")
	if err := os.WriteFile(input, []byte("Hook policy should stay bookkeeping-only.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings = filepath.Join(root, "agy-settings.json")
	if err := os.WriteFile(settings, []byte(`{"model":"Gemini 3.5 Flash (High)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, input, settings
}
