package llmpromote

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawHelpers(t *testing.T) {
	if !isLLMWikiRawType("notes") || isLLMWikiRawType("bad") {
		t.Fatal("unexpected raw type classification")
	}
	if got := RawFileName("2026-06-13", "My Draft!.md"); got != "2026-06-13-my-draft.md" {
		t.Fatalf("RawFileName = %q", got)
	}
	if got := RawFileName("2026-06-13", "2026-01-01-existing.md"); got != "2026-01-01-existing.md" {
		t.Fatalf("dated RawFileName = %q", got)
	}
	if slugifyDraftWiki(" !!! ") != "draft" {
		t.Fatal("empty slug should fall back to draft")
	}
	content := RawNoteContent(Draft{Title: "Title", RelPath: "drafts/a.md"}, "notes", "2026-06-13", "---\nx: y\n---\n# Body")
	for _, want := range []string{`title: "Title"`, "type: notes", "ingested: 2026-06-13", "original_draft: \"drafts/a.md\"", "# Body"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected raw content to contain %q, got:\n%s", want, content)
		}
	}
	if !strings.Contains(RawNoteContent(Draft{Title: "Empty", RelPath: "d.md"}, "notes", "2026-06-13", " "), "# Empty") {
		t.Fatal("empty draft should get title body")
	}
	if stripDraftWikiFrontmatter("---\nunclosed") != "---\nunclosed" {
		t.Fatal("unterminated frontmatter should be preserved")
	}
}

func TestResolveLLMWikiRootAndPromote(t *testing.T) {
	tmp := t.TempDir()
	hub := filepath.Join(tmp, "hub")
	wiki := filepath.Join(hub, "topics", "main")
	if err := os.MkdirAll(wiki, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(config, []byte(`{"hub_path":`+quoteJSON(hub)+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	registry := `{"default":"main","wikis":{"main":{"path":"topics/main"},"hub":{"path":"<HUB>"}}}`
	if err := os.WriteFile(filepath.Join(hub, "wikis.json"), []byte(registry), 0o600); err != nil {
		t.Fatal(err)
	}
	draftPath := filepath.Join(tmp, "Draft Note.md")
	if err := os.WriteFile(draftPath, []byte("# Draft\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := resolveLLMWikiRoot(config, "")
	if err != nil || root != wiki {
		t.Fatalf("resolve root = %q, %v", root, err)
	}
	if root, err := resolveLLMWikiRoot(config, "fallback"); err != nil || root != filepath.Join(hub, "topics", "fallback") {
		if mkdirErr := os.MkdirAll(filepath.Join(hub, "topics", "fallback"), 0o755); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
		root, err = resolveLLMWikiRoot(config, "fallback")
		if err != nil || root != filepath.Join(hub, "topics", "fallback") {
			t.Fatalf("fallback root = %q, %v", root, err)
		}
	}
	result, err := Promote(Request{
		Draft:             Draft{Title: "Draft Note", RelPath: "drafts/Draft Note.md", Path: draftPath, Summary: "summary"},
		TargetType:        "notes",
		LLMWikiConfigPath: config,
	})
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	if result.WikiRoot != wiki || result.RawRel == "" || result.RawPath == "" || result.LogPath == "" {
		t.Fatalf("unexpected promote result: %#v", result)
	}
	raw, err := os.ReadFile(result.RawPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# Draft") || !strings.Contains(string(raw), `summary: "summary"`) {
		t.Fatalf("unexpected raw file:\n%s", string(raw))
	}
	log, err := os.ReadFile(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "Draft Note") || !strings.Contains(string(log), result.RawRel) {
		t.Fatalf("unexpected log:\n%s", string(log))
	}
	if _, err := Promote(Request{Draft: Draft{Title: "Draft Note", RelPath: "drafts/Draft Note.md", Path: draftPath}, LLMWikiConfigPath: config}); err == nil {
		t.Fatal("duplicate promote should fail")
	}
	if _, err := Promote(Request{Draft: Draft{Path: draftPath}, TargetType: "bad", LLMWikiConfigPath: config}); err == nil {
		t.Fatal("unsupported raw type should fail")
	}
}

func TestResolveLLMWikiRootErrors(t *testing.T) {
	tmp := t.TempDir()
	config := filepath.Join(tmp, "config.json")
	if _, err := resolveLLMWikiHub(config); err == nil {
		t.Fatal("missing config should fail")
	}
	if err := os.WriteFile(config, []byte(`{bad`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLLMWikiHub(config); err == nil {
		t.Fatal("bad config JSON should fail")
	}
	if err := os.WriteFile(config, []byte(`{"hub_path":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveLLMWikiHub(config); err == nil {
		t.Fatal("empty hub should fail")
	}
}

func quoteJSON(value string) string {
	return `"` + strings.ReplaceAll(value, `\`, `\\`) + `"`
}
