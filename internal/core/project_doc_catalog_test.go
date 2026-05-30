package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProjectDoc(t *testing.T, repo, name, content string) {
	t.Helper()
	dir := filepath.Join(repo, ".agent-harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverProjectDocsUsesFrontmatterThenCanonicalMeta(t *testing.T) {
	repo := t.TempDir()
	// Custom doc with its own frontmatter description.
	writeProjectDoc(t, repo, "CUSTOM.md", "---\nname: CUSTOM.md\ndescription: 이 레포 전용 메모를 담는다.\n---\n\n# 커스텀\n")
	// Known standard doc WITHOUT frontmatter -> canonical metadata fallback.
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 경계\n")
	// Nested dirs and non-md files must be ignored.
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "notes.txt", "ignored")

	catalog := DiscoverProjectDocs(repo)
	if len(catalog) != 2 {
		t.Fatalf("expected 2 project docs, got %d: %+v", len(catalog), catalog)
	}
	// Sorted by rel path: ARCHITECTURE before CUSTOM.
	if catalog[0].RelPath != ".agent-harness/ARCHITECTURE.md" || catalog[1].RelPath != ".agent-harness/CUSTOM.md" {
		t.Fatalf("catalog not sorted by rel path: %+v", catalog)
	}
	canonical, _ := DocMetaDescription("ARCHITECTURE.md")
	if catalog[0].Description != canonical {
		t.Fatalf("expected canonical meta fallback, got %q", catalog[0].Description)
	}
	if catalog[1].Description != "이 레포 전용 메모를 담는다." {
		t.Fatalf("expected frontmatter description, got %q", catalog[1].Description)
	}
	if catalog[0].Title != "아키텍처" {
		t.Fatalf("expected H1 title, got %q", catalog[0].Title)
	}
}

func TestDiscoverProjectDocsEmptyWhenNoAgentHarness(t *testing.T) {
	if got := DiscoverProjectDocs(t.TempDir()); got != nil {
		t.Fatalf("expected nil catalog for repo without .agent-harness, got %+v", got)
	}
	if got := DiscoverProjectDocs(""); got != nil {
		t.Fatalf("expected nil catalog for empty repo root, got %+v", got)
	}
}

func TestFormatProjectDocCatalogUsesDescription(t *testing.T) {
	catalog := []ProjectDocCatalogEntry{
		{RelPath: ".agent-harness/ADR.md", Title: "구현 계획", Description: "프로젝트의 결정과 근거를 담는다."},
		{RelPath: ".agent-harness/X.md", Title: "엑스", Description: ""},
	}
	got := FormatProjectDocCatalog(catalog)
	if !strings.HasPrefix(got, "project docs (read what's relevant): ") {
		t.Fatalf("unexpected catalog prefix: %q", got)
	}
	if !strings.Contains(got, "ADR.md=프로젝트의 결정과 근거를 담는다.") {
		t.Fatalf("catalog should use description: %s", got)
	}
	if !strings.Contains(got, "X.md=엑스") {
		t.Fatalf("catalog should fall back to title when no description: %s", got)
	}
}

func TestFormatProjectDocCatalogEmpty(t *testing.T) {
	if got := FormatProjectDocCatalog(nil); got != "" {
		t.Fatalf("expected empty string for empty catalog, got %q", got)
	}
}

func TestBuildUserPromptMCPHintsInjectsProjectDocCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘", Repo: repo})
	if !got.ShouldInject {
		t.Fatalf("expected injection when repo has project docs: %+v", got)
	}
	canonical, _ := DocMetaDescription("ARCHITECTURE.md")
	if !strings.Contains(got.AdditionalContext, "project docs (read what's relevant):") || !strings.Contains(got.AdditionalContext, "ARCHITECTURE.md="+canonical) {
		t.Fatalf("expected project doc catalog with canonical meta in context:\n%s", got.AdditionalContext)
	}
	if len(got.ProjectDocs) != 1 {
		t.Fatalf("expected catalog entry in result, got %+v", got.ProjectDocs)
	}
}

func TestBuildUserPromptMCPHintsNoCatalogWithoutRepoDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "그냥 질문이야"})
	if strings.Contains(got.AdditionalContext, "project docs (read what's relevant):") {
		t.Fatalf("catalog must not appear without a working repo: %s", got.AdditionalContext)
	}
	if got.ProjectDocs != nil {
		t.Fatalf("expected no project docs without repo, got %+v", got.ProjectDocs)
	}
}
