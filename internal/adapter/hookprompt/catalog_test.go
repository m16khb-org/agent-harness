package hookprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	"agent-harness/internal/core/projectdoc"
)

func linkedWorktreeCycleForHookPromptTest(t *testing.T, repo, branch string) string {
	t.Helper()
	record, err := issueops.StartIssueOps(issueops.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/" + strings.SplitN(branch, "-", 2)[0]
	if _, err := issueops.LinkIssueOpsIssue(issueops.IssueOpsStateRoot(), record.ID, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.PrepareIssueOpsBranch(issueops.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", branch)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.LinkIssueOpsWorktree(issueops.IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	record, err = issueops.ReadIssueOps(issueops.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhaseImplement
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	return worktree
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

func TestBuildProjectDocCatalogContextIncludesLinkedWorktreeReminder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "agent-harness")
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 핵심 경계\n")
	worktree := linkedWorktreeCycleForHookPromptTest(t, repo, "2519-test-quality-comprehensive")

	cat := BuildProjectDocCatalogContext(repo)
	for _, text := range []string{cat.Compact, cat.UserView} {
		if !strings.Contains(text, "worktree: "+worktree) || !strings.Contains(text, "편집 전 cwd/절대경로 확인") {
			t.Fatalf("catalog context missing worktree reminder:\n%s", text)
		}
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

func TestBuildUserPromptMCPHintsIncludesLinkedWorktreeReminder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := linkedWorktreeCycleForHookPromptTest(t, repo, "2519-test-quality-comprehensive")

	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "계획 계속 진행", Repo: repo})
	if !strings.Contains(got.AdditionalContext, "worktree: "+worktree) || !strings.Contains(got.AdditionalContext, "편집 전 cwd/절대경로 확인") {
		t.Fatalf("user prompt context missing worktree reminder:\n%s", got.AdditionalContext)
	}
	emptyRepo := filepath.Join(t.TempDir(), "agent-harness")
	if noCycle := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "계획 계속 진행", Repo: emptyRepo}); strings.Contains(noCycle.AdditionalContext, "편집 전 cwd/절대경로 확인") {
		t.Fatalf("user prompt context should not include worktree reminder without linked cycles:\n%s", noCycle.AdditionalContext)
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
	events := []lifecyclecontract.DocUpkeepEvent{
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
