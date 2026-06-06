package projectdoc

import (
	"strings"
	"testing"
)

func TestParseDocFrontmatter(t *testing.T) {
	name, desc, body, ok := ParseFrontmatter("---\nname: ADR.md\ndescription: 결정과 근거.\n---\n\n# 제목\n본문\n")
	if !ok || name != "ADR.md" || desc != "결정과 근거." {
		t.Fatalf("parse mismatch: ok=%v name=%q desc=%q", ok, name, desc)
	}
	if body != "# 제목\n본문\n" {
		t.Fatalf("body mismatch: %q", body)
	}
	if _, _, _, has := ParseFrontmatter("# No frontmatter\n"); has {
		t.Fatalf("expected no frontmatter detected")
	}
}

func TestEnsureDocMetaFrontmatterPrependsAndIsIdempotent(t *testing.T) {
	canonical, _ := DocMetaDescription("ADR.md")
	body := "# 구현 계획\n\n결정들\n"

	once := EnsureMetaFrontmatter("ADR.md", body)
	if !strings.HasPrefix(once, "---\nname: ADR.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("frontmatter not prepended:\n%s", once)
	}
	if !strings.Contains(once, "# 구현 계획") || !strings.Contains(once, "결정들") {
		t.Fatalf("body not preserved:\n%s", once)
	}
	if twice := EnsureMetaFrontmatter("ADR.md", once); twice != once {
		t.Fatalf("ensure not idempotent:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestEnsureDocMetaFrontmatterReplacesStaleBlockPreservingBody(t *testing.T) {
	canonical, _ := DocMetaDescription("ADR.md")
	stale := "---\nname: ADR.md\ndescription: 옛 설명.\n---\n\n# 구현 계획\n본문 유지\n"
	got := EnsureMetaFrontmatter("ADR.md", stale)
	if strings.Contains(got, "옛 설명") {
		t.Fatalf("stale description should be replaced:\n%s", got)
	}
	if !strings.Contains(got, "description: "+canonical) || !strings.Contains(got, "본문 유지") {
		t.Fatalf("canonical desc or body missing:\n%s", got)
	}
}

func TestEnsureDocMetaFrontmatterLeavesUnknownDocsUnchanged(t *testing.T) {
	body := "# Custom\n내용\n"
	if got := EnsureMetaFrontmatter("NOT_A_STANDARD_DOC.md", body); got != body {
		t.Fatalf("unknown doc must be unchanged, got:\n%s", got)
	}
}
