package draftwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPromoteDraftWikiDryRunDoesNotWriteLLMWiki(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "dry-run.md")
	mustWrite(t, approved, `---
title: "Dry Run"
target_wiki: "agent-harness"
target_type: "notes"
---

# Dry Run
`)

	result, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, LLMWikiConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.DryRun || result.Confirm || result.Executed {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if result.UpstreamTool != "nvk/llm-wiki" || !strings.Contains(result.HandoffCommand, "@wiki ingest") {
		t.Fatalf("dry-run should report upstream handoff only: %+v", result)
	}
	rawDir := filepath.Join(hub, "topics", "agent-harness", "raw")
	if _, err := os.Stat(rawDir); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote raw directory or unexpected stat error: %v", err)
	}
}

func TestPromoteDraftWikiRejectsCollisionWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "collision.md")
	mustWrite(t, approved, `---
title: "Collision"
target_wiki: "agent-harness"
target_type: "notes"
---

# Collision
`)
	rawPath := filepath.Join(hub, "topics", "agent-harness", "raw", "notes", time.Now().Format(time.DateOnly)+"-collision.md")
	mustWrite(t, rawPath, "existing raw note\n")
	_, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err == nil || !strings.Contains(err.Error(), "llm-wiki raw file already exists") {
		t.Fatalf("expected collision error, got %v", err)
	}
	got, readErr := os.ReadFile(rawPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "existing raw note\n" {
	}
}

func TestPromoteDraftWikiRejectsInvalidTargetType(t *testing.T) {
	root := t.TempDir()
	configPath, _ := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "invalid-type.md")
	mustWrite(t, approved, `---
title: "Invalid Type"
target_wiki: "agent-harness"
target_type: "badtype"
---

# Invalid Type
`)

	_, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err == nil || !strings.Contains(err.Error(), `unsupported llm-wiki raw type "badtype"`) {
		t.Fatalf("expected invalid target type error, got %v", err)
	}
}

func TestLLMWikiRawNoteContentStripsDraftFrontmatterAndDefaultsBody(t *testing.T) {
	draft := DraftWikiDraft{
		Title:   "Raw Note",
		RelPath: ".agent-harness/draft-wiki/approved/raw-note.md",
	}
	withBody := LLMWikiRawNoteContent(draft, "notes", "2026-06-06", `---
title: "Raw Note"
---

# Body
`)
	for _, want := range []string{
		`title: "Raw Note"`,
		`source: "agent-harness draft-wiki:.agent-harness/draft-wiki/approved/raw-note.md"`,
		"type: notes",
		"ingested: 2026-06-06",
		"Approved agent-harness draft wiki note promoted from repo-local review staging.",
		"# Body",
	} {
		if !strings.Contains(withBody, want) {
			t.Fatalf("raw note content missing %q:\n%s", want, withBody)
		}
	}
	if strings.Contains(withBody, `title: "Raw Note"`+"\n---\n\n---") {
		t.Fatalf("draft frontmatter should not be copied into raw body:\n%s", withBody)
	}

	emptyBody := LLMWikiRawNoteContent(draft, "notes", "2026-06-06", "---\n---\n")
	if !strings.Contains(emptyBody, "# Raw Note\n") {
		t.Fatalf("empty draft body should fall back to title heading:\n%s", emptyBody)
	}
}

func TestDraftWikiRawFileNameReusesDatedSlugAndDefaultsBlankSlug(t *testing.T) {
	if got := DraftWikiRawFileName("2026-06-06", filepath.Join("approved", "2026-06-01-existing.md")); got != "2026-06-01-existing.md" {
		t.Fatalf("dated draft filename should be reused, got %q", got)
	}
	if got := DraftWikiRawFileName("2026-06-06", filepath.Join("approved", "!!!.md")); got != "2026-06-06-draft.md" {
		t.Fatalf("blank slug should use stable fallback, got %q", got)
	}
}

func TestPromoteDraftWikiConfirmDoesNotCreateLLMWikiIndexArtifacts(t *testing.T) {
	root := t.TempDir()
	configPath, hub := writeTestLLMWikiHub(t)
	approved := filepath.Join(root, DraftWikiDir, "approved", "boundary.md")
	mustWrite(t, approved, `---
title: "Boundary"
target_wiki: "agent-harness"
target_type: "notes"
---

# Boundary
`)

	result, err := PromoteDraftWiki(DraftWikiPromoteRequest{RepoRoot: root, Path: approved, Confirm: true, LLMWikiConfigPath: configPath})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Executed || result.LLMWikiRawPath == "" || result.LLMWikiLogPath == "" {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
	for _, rel := range []string{"compiled", "index", "query", "embeddings"} {
		if _, err := os.Stat(filepath.Join(hub, "topics", "agent-harness", rel)); !os.IsNotExist(err) {
			t.Fatalf("promotion must not create llm-wiki %s artifact: %v", rel, err)
		}
	}
}
