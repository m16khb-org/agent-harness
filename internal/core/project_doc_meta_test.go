package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDocFrontmatter(t *testing.T) {
	name, desc, body, ok := parseDocFrontmatter("---\nname: ADR.md\ndescription: 결정과 근거.\n---\n\n# 제목\n본문\n")
	if !ok || name != "ADR.md" || desc != "결정과 근거." {
		t.Fatalf("parse mismatch: ok=%v name=%q desc=%q", ok, name, desc)
	}
	if body != "# 제목\n본문\n" {
		t.Fatalf("body mismatch: %q", body)
	}
	if _, _, _, has := parseDocFrontmatter("# No frontmatter\n"); has {
		t.Fatalf("expected no frontmatter detected")
	}
}

func TestEnsureDocMetaFrontmatterPrependsAndIsIdempotent(t *testing.T) {
	canonical, _ := DocMetaDescription("ADR.md")
	body := "# 구현 계획\n\n결정들\n"

	once := ensureDocMetaFrontmatter("ADR.md", body)
	if !strings.HasPrefix(once, "---\nname: ADR.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("frontmatter not prepended:\n%s", once)
	}
	if !strings.Contains(once, "# 구현 계획") || !strings.Contains(once, "결정들") {
		t.Fatalf("body not preserved:\n%s", once)
	}
	if twice := ensureDocMetaFrontmatter("ADR.md", once); twice != once {
		t.Fatalf("ensure not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestEnsureDocMetaFrontmatterReplacesStaleBlockPreservingBody(t *testing.T) {
	canonical, _ := DocMetaDescription("ADR.md")
	stale := "---\nname: ADR.md\ndescription: 옛 설명.\n---\n\n# 구현 계획\n본문 유지\n"
	got := ensureDocMetaFrontmatter("ADR.md", stale)
	if strings.Contains(got, "옛 설명") {
		t.Fatalf("stale description should be replaced:\n%s", got)
	}
	if !strings.Contains(got, "description: "+canonical) || !strings.Contains(got, "본문 유지") {
		t.Fatalf("canonical desc or body missing:\n%s", got)
	}
}

func TestEnsureDocMetaFrontmatterLeavesUnknownDocsUnchanged(t *testing.T) {
	body := "# Custom\n내용\n"
	if got := ensureDocMetaFrontmatter("NOT_A_STANDARD_DOC.md", body); got != body {
		t.Fatalf("unknown doc must be unchanged, got:\n%s", got)
	}
}

func TestBootstrapWritesMetaFrontmatterIntoCreatedDocs(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	arch := mustRead(t, filepath.Join(root, ProjectDocsDir, "ARCHITECTURE.md"))
	canonical, _ := DocMetaDescription("ARCHITECTURE.md")
	if !strings.HasPrefix(arch, "---\nname: ARCHITECTURE.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("created doc missing canonical frontmatter:\n%s", firstLines(arch, 4))
	}
}

func TestBootstrapAddsFrontmatterToExistingDocPreservingBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, ProjectDocsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Hand-written existing doc with no frontmatter.
	if err := os.WriteFile(filepath.Join(dir, "CONVENTIONS.md"), []byte("# 손수 쓴 컨벤션\n\n보존되어야 할 본문\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-sync bootstrap must NOT overwrite the body but must add frontmatter.
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, filepath.Join(dir, "CONVENTIONS.md"))
	canonical, _ := DocMetaDescription("CONVENTIONS.md")
	if !strings.HasPrefix(got, "---\nname: CONVENTIONS.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("frontmatter not added to existing doc:\n%s", firstLines(got, 4))
	}
	if !strings.Contains(got, "손수 쓴 컨벤션") || !strings.Contains(got, "보존되어야 할 본문") {
		t.Fatalf("existing body must be preserved:\n%s", got)
	}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
