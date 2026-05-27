package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLLMWikiInventorySearchReadCapture(t *testing.T) {
	root := t.TempDir()
	writeLLMWikiTestFile(t, root, "00-meta/AGENTS.md", "# Schema\n\nRead before writes.\n")
	writeLLMWikiTestFile(t, root, "00-meta/index.md", "# Wiki Index\n\n- [[llm-wiki-pattern]]\n")
	writeLLMWikiTestFile(t, root, "00-meta/log.md", "# Wiki Log\n\n")
	writeLLMWikiTestFile(t, root, "10-sources/karpathy-llm-wiki-gist.md", "---\ntitle: Karpathy LLM Wiki Gist\ntype: source\nstatus: active\ntags: [llm-wiki, source]\n---\n\nRaw source card about durable wiki memory.\n")
	writeLLMWikiTestFile(t, root, "20-wiki/concepts/llm-wiki-pattern.md", "---\ntitle: LLM Wiki Pattern\ntype: concept\nstatus: active\ntags: [llm-wiki, memory]\n---\n\nLLM Wiki compiles durable interlinked markdown knowledge.\n")
	writeLLMWikiTestFile(t, root, "20-wiki/entities/andrej-karpathy.md", "---\ntitle: Andrej Karpathy\ntype: entity\nstatus: active\ntags: [person]\n---\n\nEntity page.\n")
	writeLLMWikiTestFile(t, root, "20-wiki/summaries/2026/summary.md", "---\ntitle: Summary\ntype: summary\nstatus: active\ntags: [summary]\n---\n\nSummary.\n")
	writeLLMWikiTestFile(t, root, "30-sessions/2026/session.md", "---\ntitle: Session\ntype: session\nstatus: active\ntags: [session]\n---\n\nSession.\n")

	inv, err := LLMWikiInventoryFor(root, "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	if !inv.OK || inv.Status != "ready" || inv.Counts.Sources != 1 || inv.Counts.Concepts != 1 || inv.Counts.Entities != 1 || inv.Counts.Summaries != 1 || inv.Counts.Sessions != 1 {
		t.Fatalf("unexpected inventory: %+v", inv)
	}

	ctx, err := LLMWikiSessionContextFor(root, "/tmp/project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ctx.Text, "LLM Wiki Session Context") || !strings.Contains(ctx.Text, "llm_wiki_search") || !strings.Contains(ctx.Text, "Wiki Index") {
		t.Fatalf("session context missing expected guidance:\n%s", ctx.Text)
	}

	search, err := LLMWikiSearch(root, "durable wiki", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(search.Results) == 0 || search.Results[0].Path == "" {
		t.Fatalf("expected search results: %+v", search)
	}

	read, err := LLMWikiRead(root, "llm-wiki-pattern")
	if err != nil {
		t.Fatal(err)
	}
	if read.Path != "20-wiki/concepts/llm-wiki-pattern.md" || !strings.Contains(read.Content, "durable interlinked") {
		t.Fatalf("unexpected read result: %+v", read)
	}

	capture, err := LLMWikiCapture(LLMWikiCaptureRequest{
		Root:        root,
		Title:       "Reusable Finding",
		Content:     "This should be available to future sessions.",
		ProjectPath: "/tmp/project",
		Tags:        []string{"test", "capture"},
		Sources:     []string{"[[llm-wiki-pattern]]"},
		Now:         time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !capture.OK || capture.Path != "30-sessions/2026/2026-05-27-reusable-finding.md" || !capture.LogUpdated {
		t.Fatalf("unexpected capture: %+v", capture)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(capture.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "sources:\n  - \"[[llm-wiki-pattern]]\"") {
		t.Fatalf("capture did not include quoted wikilink source:\n%s", string(b))
	}
}

func writeLLMWikiTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
