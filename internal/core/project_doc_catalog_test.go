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

func TestBuildProjectDocCatalogContext(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	cat := BuildProjectDocCatalogContext(repo)
	if !cat.ShouldInject || len(cat.ProjectDocs) != 1 {
		t.Fatalf("expected catalog context with one doc: %+v", cat)
	}
	canonical, _ := DocMetaDescription("ARCHITECTURE.md")
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

func TestBuildUserPromptMCPHintsHasNoCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘", Repo: repo})
	// The catalog now ships via SessionStart/PostCompact, not per turn.
	if strings.Contains(got.AdditionalContext, "project docs (read what's relevant):") {
		t.Fatalf("user-prompt must not embed the catalog: %s", got.AdditionalContext)
	}
	if len(got.ProjectDocs) != 0 {
		t.Fatalf("user-prompt result must not carry catalog docs: %+v", got.ProjectDocs)
	}
}

func TestRenderUserPromptUserView(t *testing.T) {
	result := HookUserPromptResult{
		ProjectDocs: []ProjectDocCatalogEntry{
			{RelPath: ".agent-harness/ADR.md", Title: "구현 계획", Description: "Structural decisions, rationale, and rejected alternatives."},
			{RelPath: ".agent-harness/X.md", Title: "엑스", Description: ""},
		},
	}
	view := RenderUserPromptUserView(result)
	for _, want := range []string{"📚 agent-harness", "• ADR.md — Structural decisions, rationale, and rejected alternatives.", "• X.md — 엑스"} {
		if !strings.Contains(view, want) {
			t.Fatalf("user view missing %q:\n%s", want, view)
		}
	}
	// Pretty view is multi-line (one bullet per doc).
	if strings.Count(view, "\n") < 2 {
		t.Fatalf("expected multi-line user view:\n%s", view)
	}
	if got := RenderUserPromptUserView(HookUserPromptResult{}); got != "" {
		t.Fatalf("expected empty user view without docs, got %q", got)
	}
}

func TestRenderUserPromptCodexContextPreservesFullCatalogForAgent(t *testing.T) {
	result := HookUserPromptResult{
		AdditionalContext: "[agent-harness] 프로젝트 지침 확인 중... | project docs (read what's relevant): ADR.md=Structural decisions, rationale, and rejected alternatives. | route: choose project docs if ambiguous | rule: use docs/tools only when material",
		ProjectDocs: []ProjectDocCatalogEntry{
			{RelPath: ".agent-harness/ADR.md", Title: "구현 계획", Description: "Structural decisions, rationale, and rejected alternatives."},
		},
	}
	view := RenderUserPromptCodexContext(result)
	for _, want := range []string{"📚 agent-harness", "\n• ADR.md — Structural decisions, rationale, and rejected alternatives."} {
		if !strings.Contains(view, want) {
			t.Fatalf("Codex context missing %q:\n%s", want, view)
		}
	}
	for _, blocked := range []string{"[agent-harness]", "route:", "actions:", "profile:", "pending upkeep:", "rule:", "project docs (read what's relevant):"} {
		if strings.Contains(view, blocked) {
			t.Fatalf("Codex context should only contain the readable full catalog; found %q:\n%s", blocked, view)
		}
	}
}

func TestAppendCompactPendingUpkeepDeduplicatesEvents(t *testing.T) {
	parts := []string{}
	events := []DocUpkeepEvent{
		{TargetDocs: []string{"ARCHITECTURE.md", "OPERATIONS.md"}, Summary: "Bash touched harness lifecycle-relevant files; shared project docs may need review."},
		{TargetDocs: []string{"ARCHITECTURE.md", "OPERATIONS.md"}, Summary: "Bash touched harness lifecycle-relevant files; shared project docs may need review."},
	}
	appendCompactPendingUpkeep(&parts, events)
	got := strings.Join(parts, "\n")
	if strings.Count(got, "Bash touched") != 1 {
		t.Fatalf("expected duplicate pending upkeep entries to be collapsed, got:\n%s", got)
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
