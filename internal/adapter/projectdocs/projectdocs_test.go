package projectdocs

import (
	projectdocscontract "agent-harness/internal/contract/projectdocs"
	projectdoc "agent-harness/internal/domain/projectdoc"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeRenderRouteAndProfile(t *testing.T) {
	root := projectDocsFixture(t)
	signals := AnalyzeProjectSignals(root)
	for _, want := range []string{"Go", "JavaScript/TypeScript"} {
		if !containsProjectDocString(signals.Languages, want) {
			t.Fatalf("expected language %q in %#v", want, signals.Languages)
		}
	}
	if !containsProjectDocString(signals.PackageManagers, "pnpm") {
		t.Fatalf("expected pnpm package manager in %#v", signals.PackageManagers)
	}
	if signals.Profile.VCS.Provider != "github" || signals.Profile.VCS.RemoteHost != "github.com" {
		t.Fatalf("unexpected VCS profile: %#v", signals.Profile.VCS)
	}
	if len(signals.GitHubWorkflows) != 1 || len(signals.TestCommands) == 0 || len(signals.BuildCommands) == 0 || len(signals.LintCommands) == 0 {
		t.Fatalf("unexpected signals: %#v", signals)
	}
	if remoteHost("git@gitlab.example.com:team/repo.git") != "gitlab.example.com" || remoteHost("") != "" {
		t.Fatal("unexpected remoteHost parsing")
	}
	docs := RenderProjectDocs(root, signals)
	// 11 root docs + 6 family module starters.
	if len(docs) != 17 {
		t.Fatalf("expected 17 rendered docs, got %d", len(docs))
	}
	for rel, content := range docs {
		if !strings.Contains(content, "#") || !strings.Contains(content, "name:") || !strings.Contains(content, "description:") {
			t.Fatalf("rendered doc %s missing expected content/frontmatter:\n%s", rel, content)
		}
	}
	conventionsOverview := docs[filepath.ToSlash(filepath.Join(ProjectDocsDir, "conventions", "overview.md"))]
	if !strings.Contains(conventionsOverview, "Engineering standards checklist") || !strings.Contains(conventionsOverview, "DDD") || !strings.Contains(conventionsOverview, "engineering-standards.md") {
		t.Fatalf("conventions overview missing engineering standards checklist:\n%s", conventionsOverview)
	}
	architectureOverview := docs[filepath.ToSlash(filepath.Join(ProjectDocsDir, "architecture", "overview.md"))]
	if !strings.Contains(architectureOverview, "hexagonal/ports-and-adapters") {
		t.Fatalf("architecture overview missing style naming guidance:\n%s", architectureOverview)
	}
	route, err := RouteProjectDocs(root, "OpenAPI controller DTO test")
	if err != nil {
		t.Fatalf("RouteProjectDocs returned error: %v", err)
	}
	if route.Task != "openapi controller dto test" || len(route.Docs) == 0 || len(route.Warnings) != 0 {
		t.Fatalf("unexpected route result: %#v", route)
	}
	if !routeContains(route.Docs, filepath.ToSlash(filepath.Join(ProjectDocsDir, "OPEN_API_SPEC.md"))) {
		t.Fatalf("route missing OPEN_API_SPEC: %#v", route.Docs)
	}
	if len(routeDocsForTask("dependency upgrade")) < 2 || len(routeDocsForTask("")) < 2 {
		t.Fatal("expected routed docs for dependency and default tasks")
	}
}

func TestReadUpdateRecordAndAgentsBlock(t *testing.T) {
	root := t.TempDir()
	missing, err := ReadProjectDoc(root, filepath.ToSlash(filepath.Join(ProjectDocsDir, "TESTING.md")))
	if err != nil {
		t.Fatalf("ReadProjectDoc missing returned error: %v", err)
	}
	if missing.Exists || len(missing.Warnings) == 0 {
		t.Fatalf("expected missing warning, got %#v", missing)
	}
	create, err := ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{
		RepoRoot: root,
		RelPath:  filepath.ToSlash(filepath.Join(ProjectDocsDir, "TESTING.md")),
		Content:  "# Testing\n",
		Summary:  "seed testing",
		Evidence: []string{" ", "test"},
		Confirm:  true,
	})
	if err != nil {
		t.Fatalf("ReviseProjectDoc create returned error: %v", err)
	}
	if create.Action != "create" || create.DryRun || !create.Confirmed || len(create.Evidence) != 1 {
		t.Fatalf("unexpected create result: %#v", create)
	}
	read, err := ReadProjectDoc(root, create.RelPath)
	if err != nil || !read.Exists || read.SHA256 == "" || !strings.Contains(read.Content, "# Testing") {
		t.Fatalf("unexpected read result: %#v err=%v", read, err)
	}
	if _, err := ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{RepoRoot: root, RelPath: create.RelPath, Content: "# Changed", Summary: "change"}); err == nil {
		t.Fatal("expected existing update without SHA to fail")
	}
	dry, err := ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{RepoRoot: root, RelPath: create.RelPath, Content: "# Changed", Summary: "change", ExpectedSHA256: read.SHA256})
	if err != nil {
		t.Fatalf("dry update returned error: %v", err)
	}
	if !dry.DryRun || dry.Action != "update" || len(dry.Warnings) == 0 {
		t.Fatalf("unexpected dry update: %#v", dry)
	}
	for _, req := range []projectdocscontract.ProjectDocsAppendRequest{
		{RepoRoot: root, Kind: "failure", Title: "Caution", Summary: "Summary", Context: "ctx", Resolution: "fixed", Evidence: []string{"go test"}, Source: "test"},
		{RepoRoot: root, Kind: "decision", Title: "ADR", Summary: "Summary", Decision: "do it", Alternatives: []string{"skip"}, Consequences: "tradeoff"},
	} {
		result, err := AppendProjectDocsEntry(req)
		if err != nil {
			t.Fatalf("AppendProjectDocsEntry returned error: %v", err)
		}
		if !result.OK || result.BytesAppended == 0 || result.SHA256 == "" {
			t.Fatalf("unexpected record result: %#v", result)
		}
	}
	if _, err := AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{RepoRoot: root, Kind: "unknown", Title: "x", Summary: "y"}); err == nil {
		t.Fatal("expected unsupported record kind")
	}
	rendered := RenderAgentsWithBlock(root, "")
	if !strings.Contains(rendered, agentsStartMarker) || !strings.Contains(rendered, ProjectDocsDir+"/TESTING.md") {
		t.Fatalf("unexpected AGENTS block:\n%s", rendered)
	}
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("Custom\n\n"+agentsStartMarker+"\nold\n"+agentsEndMarker+"\nTail\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replaced := RenderAgentsWithBlock(root, "")
	if strings.Contains(replaced, "\nold\n") || !strings.Contains(replaced, "Tail") {
		t.Fatalf("expected existing block replacement, got:\n%s", replaced)
	}
	if !strings.Contains(ensureBehavioralGuidelinesAtTop("Custom"), "Custom") {
		t.Fatal("behavioral guidelines wrapper should preserve custom content")
	}
}

func TestOptionalVCSProjectDocCanBeCreatedAndReadOnDemand(t *testing.T) {
	root := t.TempDir()
	created, err := ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{
		RepoRoot: root,
		RelPath:  ".agent-harness/VCS.md",
		Content:  "# VCS\n\n## GitHub\n",
		Summary:  "record verified provider recipe",
		Confirm:  true,
	})
	if err != nil || created.Action != "create" {
		t.Fatalf("create optional VCS.md: result=%#v err=%v", created, err)
	}
	read, err := ReadProjectDoc(root, ".agent-harness/VCS.md")
	if err != nil || !read.Exists || !strings.Contains(read.Content, "## GitHub") {
		t.Fatalf("read optional VCS.md: result=%#v err=%v", read, err)
	}
}

func TestRouteProjectDocsIncludesOptionalVCSForRemoteWork(t *testing.T) {
	root := t.TempDir()
	route, err := RouteProjectDocs(root, "GitLab MR push")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		".agent-harness/VCS.md",
		".agent-harness/COMMIT_POLICY.md",
		".agent-harness/TESTING.md",
		".agent-harness/CAUTIONS.md",
	} {
		if !routeContains(route.Docs, want) {
			t.Fatalf("combined VCS/commit route missing %s: %+v", want, route.Docs)
		}
	}
}

func TestProjectDocsHelpers(t *testing.T) {
	if rel, err := normalizeProjectDocRelPath(filepath.ToSlash(filepath.Join(ProjectDocsDir, "ADR.md"))); err != nil || rel == "" {
		t.Fatalf("normalize rel = %q, %v", rel, err)
	}
	if got := nonEmptyStrings([]string{"", " a ", "b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("nonEmptyStrings = %#v", got)
	}
	if got := appendUnique([]string{"a"}, "a"); len(got) != 1 {
		t.Fatalf("appendUnique duplicate = %#v", got)
	}
	if got := appendUnique([]string{"a"}, "b"); len(got) != 2 {
		t.Fatalf("appendUnique new = %#v", got)
	}
	tmp := t.TempDir()
	path := filepath.Join(tmp, "doc.md")
	if plannedFileAction(path, "x") != "create" {
		t.Fatal("missing file should be create")
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if plannedFileAction(path, "x") != "unchanged" || plannedFileAction(path, "y") != "update" {
		t.Fatal("unexpected planned file action")
	}
	if sha256Hex("x") == "" || !strings.Contains(ensureDocMetaFrontmatter("ADR.md", "# ADR"), "# ADR") {
		t.Fatal("unexpected primitive helpers")
	}
	if !isProjectSignalFile("go.mod") || isProjectSignalFile("random.txt") {
		t.Fatal("unexpected signal file classification")
	}
	if got := bulletListWithFallback(nil, "fallback"); got != "- fallback\n" {
		t.Fatalf("bullet fallback = %q", got)
	}
	if got := commandList([]projectdoc.EvidenceCommand{{Command: "go test", Evidence: []string{"go.mod"}, Confidence: "high"}}); !strings.Contains(got, "`go test`") {
		t.Fatalf("commandList = %q", got)
	}
}

func projectDocsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeProjectDocFixtureFile(t, root, "go.mod", "module example.com/repo\n")
	writeProjectDocFixtureFile(t, root, "package.json", `{"scripts":{"test":"vitest"}}`)
	writeProjectDocFixtureFile(t, root, "pnpm-lock.yaml", "lock")
	writeProjectDocFixtureFile(t, root, "Makefile", "test:\n\tgo test ./...\n")
	writeProjectDocFixtureFile(t, root, "AGENTS.md", "# AGENTS\n")
	writeProjectDocFixtureFile(t, root, ".github/workflows/ci.yml", "name: ci\n")
	writeProjectDocFixtureFile(t, root, "cmd/app/main_test.go", "package app\n")
	writeProjectDocFixtureFile(t, root, ".git/config", "[remote \"origin\"]\nurl = https://github.com/acme/repo.git\n")
	if err := os.MkdirAll(filepath.Join(root, ProjectDocsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeProjectDocFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func containsProjectDocString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func routeContains(entries []projectdocscontract.ProjectDocRouteEntry, rel string) bool {
	for _, entry := range entries {
		if entry.RelPath == rel {
			return true
		}
	}
	return false
}

func TestRenderAgentsWithBlockRespectsCuratedHeader(t *testing.T) {
	root := t.TempDir()
	curated := "# nextcandle-api\n\n## Core behavior\n\n- 프로젝트 자체 규칙이 우선한다.\n\n" + agentsStartMarker + "\nold\n" + agentsEndMarker + "\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(curated), 0o600); err != nil {
		t.Fatal(err)
	}
	got := RenderAgentsWithBlock(root, "")
	if !strings.HasPrefix(got, "# nextcandle-api\n") {
		t.Fatalf("curated AGENTS.md header must stay authoritative:\n%s", firstLinesGot(got))
	}
	if strings.Contains(got, "Behavioral guidelines to reduce common LLM coding mistakes") {
		t.Fatalf("generic template must not be stacked over repo-authored rules:\n%s", firstLinesGot(got))
	}
	if !strings.Contains(got, "프로젝트 자체 규칙이 우선한다.") {
		t.Fatalf("repo-authored rules must survive marker refresh:\n%s", firstLinesGot(got))
	}
	if !strings.Contains(got, agentsStartMarker) || strings.Contains(got, "\nold\n") {
		t.Fatalf("marker block must be refreshed in place:\n%s", firstLinesGot(got))
	}
}

func firstLinesGot(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 8 {
		lines = lines[:8]
	}
	return strings.Join(lines, "\n")
}
