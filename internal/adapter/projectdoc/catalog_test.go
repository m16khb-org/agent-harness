package projectdoc

import (
	projectdocdomain "agent-harness/internal/domain/projectdoc"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectDocsUsesFrontmatterThenCanonicalMeta(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "CUSTOM.md", "---\nname: CUSTOM.md\ndescription: 이 레포 전용 메모를 담는다.\n---\n\n# 커스텀\n")
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 경계\n")
	if err := os.MkdirAll(filepath.Join(repo, ".agent-harness", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "notes.txt", "ignored")

	catalog := DiscoverProjectDocs(repo)
	if len(catalog) != 2 {
		t.Fatalf("expected 2 project docs, got %d: %+v", len(catalog), catalog)
	}
	if catalog[0].RelPath != ".agent-harness/ARCHITECTURE.md" || catalog[1].RelPath != ".agent-harness/CUSTOM.md" {
		t.Fatalf("catalog not sorted by rel path: %+v", catalog)
	}
	canonical, _ := projectdocdomain.DocMetaDescription("ARCHITECTURE.md")
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

func TestDiscoverProjectDocsSkipsSymlinkAndNonRegularMarkdown(t *testing.T) {
	repo := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo, ".agent-harness")
	if err := os.MkdirAll(filepath.Join(dir, "directory.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "outside.md")); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "inside.md", "# Inside\n")

	catalog := DiscoverProjectDocs(repo)
	if len(catalog) != 1 || catalog[0].RelPath != ".agent-harness/inside.md" {
		t.Fatalf("catalog must exclude symlinked and non-regular markdown: %+v", catalog)
	}
}

func TestDiscoverProjectDocsRejectsSymlinkedAgentHarnessDirectory(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.md"), []byte("# Outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, ".agent-harness")); err != nil {
		t.Fatal(err)
	}

	if got := DiscoverProjectDocs(repo); got != nil {
		t.Fatalf("symlinked .agent-harness directory must be ignored: %+v", got)
	}
}

func TestDiscoverProjectDocsBoundsEntriesAndContent(t *testing.T) {
	repo := t.TempDir()
	for index := 0; index < projectDocCatalogMaxEntries+1; index++ {
		writeProjectDoc(t, repo, fmt.Sprintf("entry-%02d.md", index), "# Entry\n")
	}
	if got := DiscoverProjectDocs(repo); len(got) != projectDocCatalogMaxEntries {
		t.Fatalf("catalog entries = %d, want %d", len(got), projectDocCatalogMaxEntries)
	}

	perFileRepo := t.TempDir()
	writeProjectDoc(t, perFileRepo, "oversize.md", strings.Repeat("x", projectDocCatalogMaxFileBytes+1))
	if got := DiscoverProjectDocs(perFileRepo); len(got) != 0 {
		t.Fatalf("oversize document must be skipped: %+v", got)
	}

	totalRepo := t.TempDir()
	for index := 0; index < 8; index++ {
		writeProjectDoc(t, totalRepo, fmt.Sprintf("entry-%02d.md", index), strings.Repeat("x", 240*1024))
	}
	writeProjectDoc(t, totalRepo, "entry-99.md", strings.Repeat("x", 160*1024))
	if got := DiscoverProjectDocs(totalRepo); len(got) != 8 {
		t.Fatalf("catalog must stop before exceeding total content bound: %d entries", len(got))
	}
}

func TestReadProjectDocCatalogEntriesCapsRawDirectoryScan(t *testing.T) {
	dir := t.TempDir()
	for index := 0; index < projectDocCatalogMaxRawEntries+1; index++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("ignored-%03d.txt", index)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	file, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	entries, err := readProjectDocCatalogEntries(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != projectDocCatalogMaxRawEntries {
		t.Fatalf("raw entries = %d, want %d", len(entries), projectDocCatalogMaxRawEntries)
	}
}

func TestFormatProjectDocCatalogUsesDescription(t *testing.T) {
	catalog := []projectdocdomain.ProjectDocCatalogEntry{
		{RelPath: ".agent-harness/ADR.md", Title: "구현 계획", Description: "Structural decisions, rationale, and rejected alternatives."},
		{RelPath: ".agent-harness/X.md", Title: "엑스", Description: ""},
	}
	got := FormatProjectDocCatalog(catalog)
	if !strings.HasPrefix(got, "project docs (read what's relevant): ") {
		t.Fatalf("unexpected catalog prefix: %q", got)
	}
	if !strings.Contains(got, "ADR.md=Structural decisions, rationale, and rejected alternatives.") {
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
