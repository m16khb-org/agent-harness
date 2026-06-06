package docs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocsIndexIncludesAgentDocs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	index := DocsIndex(root, "test")
	if !index.OK {
		t.Fatalf("DocsIndex ok=false: %+v", index)
	}
	for _, want := range []string{"AGENTS.md", "CLAUDE.md", ".agent-harness/COMMIT_POLICY.md", ".agent-harness/OPERATIONS.md"} {
		if !docIndexContains(index.Docs, want) {
			t.Fatalf("DocsIndex missing %s: %+v", want, index.Docs)
		}
	}
	for _, doc := range index.Docs {
		if doc.Title == "" {
			t.Fatalf("doc %s has empty title", doc.RelPath)
		}
	}
}

func TestDocsIndexExcludesDraftWiki(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Rules\n")
	mustWrite(t, filepath.Join(root, ".agent-harness", "CAUTIONS.md"), "# Cautions\n")
	mustWrite(t, filepath.Join(root, ".agent-harness", "draft-wiki", "draft", "candidate.md"), "# Draft candidate\n")

	index := DocsIndex(root, "test")
	if !docIndexContains(index.Docs, "AGENTS.md") {
		t.Fatalf("DocsIndex missing AGENTS.md: %+v", index.Docs)
	}
	if !docIndexContains(index.Docs, ".agent-harness/CAUTIONS.md") {
		t.Fatalf("DocsIndex missing CAUTIONS.md: %+v", index.Docs)
	}
	if docIndexContains(index.Docs, ".agent-harness/draft-wiki/draft/candidate.md") {
		t.Fatalf("DocsIndex included draft-wiki candidate: %+v", index.Docs)
	}
}

func docIndexContains(docs []DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
