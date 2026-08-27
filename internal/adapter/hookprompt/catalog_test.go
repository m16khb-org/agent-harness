package hookprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/domain/projectdoc"
)

func writeProjectDoc(t *testing.T, repo, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".agent-harness", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildProjectDocCatalogContext(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	cat := BuildProjectDocCatalogContext(repo)
	if !cat.ShouldInject || len(cat.ProjectDocs) != 1 {
		t.Fatalf("expected catalog context with one doc: %+v", cat)
	}
	canonical, _ := projectdoc.DocMetaDescription("ARCHITECTURE.md")
	if !strings.Contains(cat.Compact, "project docs (read what's relevant):") || !strings.Contains(cat.Compact, "ARCHITECTURE.md="+canonical) {
		t.Fatalf("compact catalog missing canonical meta: %q", cat.Compact)
	}
	if !strings.Contains(cat.UserView, "📚") || !strings.Contains(cat.UserView, "ARCHITECTURE.md") {
		t.Fatalf("user view missing catalog: %q", cat.UserView)
	}
	if got := BuildProjectDocCatalogContext(t.TempDir()); got.ShouldInject {
		t.Fatalf("expected no injection without docs: %+v", got)
	}
}

func TestRenderProjectDocCatalogUserViewFallsBackToTitle(t *testing.T) {
	view := renderProjectDocCatalogUserView([]ProjectDocCatalogEntry{
		{RelPath: ".agent-harness/ADR.md", Title: "Decisions", Description: "Structural decisions."},
		{RelPath: ".agent-harness/NOTES.md", Title: "Notes only"},
	})
	if !strings.Contains(view, "• ADR.md — Structural decisions.") || !strings.Contains(view, "• NOTES.md — Notes only") {
		t.Fatalf("user view = %q", view)
	}
	if renderProjectDocCatalogUserView(nil) != "" {
		t.Fatal("empty catalog must render nothing")
	}
}
