package hookprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/projectdoc"
)

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

func TestBuildUserPromptMCPHintsHasNoCatalog(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘", Repo: repo})
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
	AppendCompactPendingUpkeep(&parts, events)
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
