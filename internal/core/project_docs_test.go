package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapProjectDocsDryRunAndWrite(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Existing Rules\n\nKeep this.\n")

	dry, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || !dry.DryRun || dry.Write {
		t.Fatalf("unexpected dry-run flags: %+v", dry)
	}
	if _, err := os.Stat(filepath.Join(root, ProjectDocsDir)); !os.IsNotExist(err) {
		t.Fatalf("dry-run created docs dir or unexpected stat error: %v", err)
	}
	wantBootstrapFiles := 1 + len(ProjectDocNames()) + len(draftWikiSeedFiles())
	if len(dry.Files) != wantBootstrapFiles {
		t.Fatalf("planned files=%d want %d", len(dry.Files), wantBootstrapFiles)
	}
	for rel := range draftWikiSeedFiles() {
		if !projectPlanContainsRel(dry.Files, rel) {
			t.Fatalf("dry-run plan missing draft-wiki seed %s: %+v", rel, dry.Files)
		}
	}
	if dry.LifecycleState.ProjectStateDir == "" || dry.LifecycleState.ProjectJSONPath == "" {
		t.Fatalf("dry-run missing lifecycle namespace plan: %+v", dry.LifecycleState)
	}
	if _, err := os.Stat(dry.LifecycleState.ProjectJSONPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote lifecycle project.json or unexpected stat error: %v", err)
	}
	if !containsProjectCommand(dry.Signals.TestCommands, "go test ./...") {
		t.Fatalf("go test command not inferred: %+v", dry.Signals.TestCommands)
	}
	if dry.Signals.Profile.VCS.Provider != "none" || !containsProjectString(dry.Signals.Profile.ProjectTypes, "backend") {
		t.Fatalf("dry-run profile not inferred from repo evidence: %+v", dry.Signals.Profile)
	}

	written, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if !written.OK || written.DryRun || !written.Write {
		t.Fatalf("unexpected write flags: %+v", written)
	}
	if !written.LifecycleState.NamespaceValid || !written.LifecycleState.Exists {
		t.Fatalf("write did not initialize lifecycle namespace: %+v", written.LifecycleState)
	}
	if _, err := os.Stat(written.LifecycleState.ProjectJSONPath); err != nil {
		t.Fatalf("write did not create lifecycle project.json: %v", err)
	}
	if written.LifecycleState.Profile == nil || written.LifecycleState.Profile.Metadata == nil {
		t.Fatalf("write did not persist repo metadata: %+v", written.LifecycleState.Profile)
	}
	if !containsProjectString(written.LifecycleState.Profile.Metadata.Languages, "Go") || !containsProjectString(written.LifecycleState.Profile.Metadata.ProjectTypes, "backend") {
		t.Fatalf("persisted metadata missing project profile: %+v", written.LifecycleState.Profile.Metadata)
	}
	agents := mustRead(t, filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(agents, "# Existing Rules") || !strings.Contains(agents, agentsStartMarker) || !strings.Contains(agents, ".agent-harness/TESTING.md") || !strings.Contains(agents, ".agent-harness/OPERATIONS.md") {
		t.Fatalf("AGENTS.md marker block not merged correctly:\n%s", agents)
	}
	for _, name := range ProjectDocNames() {
		path := filepath.Join(root, ProjectDocsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	for rel := range draftWikiSeedFiles() {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected draft-wiki seed %s: %v", rel, err)
		}
	}
	testingDoc := mustRead(t, filepath.Join(root, ProjectDocsDir, "TESTING.md"))
	if !strings.Contains(testingDoc, "go test ./...") || !strings.Contains(testingDoc, "Confidence: high") {
		t.Fatalf("TESTING.md lacks inferred command evidence:\n%s", testingDoc)
	}
}

func TestRouteProjectDocsForPreciseTasks(t *testing.T) {
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		task string
		want []string
	}{
		{"commit and push", []string{"AGENTS.md", ".agent-harness/COMMIT_POLICY.md", ".agent-harness/TESTING.md"}},
		{"deploy with env", []string{"AGENTS.md", ".agent-harness/OPERATIONS.md", ".agent-harness/TECH_STACK.md"}},
		{"conflicting instructions", []string{"AGENTS.md", ".agent-harness/CONSTITUTION.md"}},
		{"finish work", []string{"AGENTS.md", ".agent-harness/AGENT_WORKFLOW.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.task, func(t *testing.T) {
			got, err := RouteProjectDocs(root, tc.task)
			if err != nil {
				t.Fatal(err)
			}
			for _, rel := range tc.want {
				if !routeContains(got, rel) {
					t.Fatalf("route for %q missing %s: %+v", tc.task, rel, got.Docs)
				}
			}
		})
	}
}

func TestProjectBootstrapPreservesExistingDocsUnlessSync(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ProjectDocsDir, "TESTING.md"), "# Custom Testing\n\nKeep local detail.\n")
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(root, ProjectDocsDir, "TESTING.md")); !strings.Contains(got, "Keep local detail.") {
		t.Fatalf("bootstrap without sync replaced existing doc:\n%s", got)
	}
	synced, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true, Sync: true})
	if err != nil {
		t.Fatal(err)
	}
	if !synced.Sync {
		t.Fatalf("sync flag not reflected in result: %+v", synced)
	}
	if got := mustRead(t, filepath.Join(root, ProjectDocsDir, "TESTING.md")); strings.Contains(got, "Keep local detail.") {
		t.Fatalf("bootstrap --sync did not refresh existing doc:\n%s", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func containsProjectCommand(commands []EvidenceCommand, command string) bool {
	for _, c := range commands {
		if c.Command == command {
			return true
		}
	}
	return false
}

func projectPlanContainsRel(files []ProjectDocsPlannedFile, rel string) bool {
	for _, file := range files {
		if file.RelPath == rel {
			return true
		}
	}
	return false
}

func routeContains(result ProjectDocsRouteResult, rel string) bool {
	for _, doc := range result.Docs {
		if doc.RelPath == rel {
			return true
		}
	}
	return false
}

func containsProjectString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestAppendProjectDocsRecordWritesCautionsAndADR(t *testing.T) {
	root := t.TempDir()
	caution, err := AppendProjectDocsRecord(ProjectDocsRecordRequest{
		RepoRoot:   root,
		Kind:       "caution",
		Title:      "MCP route over-read fixed",
		Summary:    "A routing rule was too broad and made agents read unrelated docs.",
		Context:    "project_docs_route returned generic docs for specific tasks",
		Resolution: "Narrowed task keywords and added route tests.",
		Evidence:   []string{"go test ./internal/core -run TestRouteProjectDocsForPreciseTasks"},
		Source:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !caution.OK || caution.RecordKind != "caution" || caution.RelPath != ".agent-harness/CAUTIONS.md" || caution.BytesAppended == 0 {
		t.Fatalf("unexpected caution result: %+v", caution)
	}
	cautions := mustRead(t, filepath.Join(root, ProjectDocsDir, "CAUTIONS.md"))
	for _, want := range []string{"MCP route over-read fixed", "Resolution:", "go test ./internal/core"} {
		if !strings.Contains(cautions, want) {
			t.Fatalf("CAUTIONS.md missing %q:\n%s", want, cautions)
		}
	}

	adr, err := AppendProjectDocsRecord(ProjectDocsRecordRequest{
		RepoRoot:     root,
		Kind:         "adr",
		Title:        "Use task-routed project docs",
		Summary:      "Agents should retrieve only the documents relevant to the current task.",
		Decision:     "Expose project_docs_route and project_docs_record through MCP.",
		Alternatives: []string{"Always inject every document at SessionStart"},
		Consequences: "Tool descriptions and routing rules must stay precise.",
		Evidence:     []string{"MCP client best-practices recommend dynamic server/tool loading"},
		Source:       "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !adr.OK || adr.RecordKind != "adr" || adr.RelPath != ".agent-harness/ADR.md" || adr.BytesAppended == 0 {
		t.Fatalf("unexpected adr result: %+v", adr)
	}
	adrDoc := mustRead(t, filepath.Join(root, ProjectDocsDir, "ADR.md"))
	for _, want := range []string{"Use task-routed project docs", "Decision:", "Always inject every document"} {
		if !strings.Contains(adrDoc, want) {
			t.Fatalf("ADR.md missing %q:\n%s", want, adrDoc)
		}
	}
}

func TestReadAndUpdateProjectDocRequireSHAConsensus(t *testing.T) {
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	read, err := ReadProjectDoc(root, ".agent-harness/TESTING.md")
	if err != nil {
		t.Fatal(err)
	}
	if !read.OK || !read.Exists || read.SHA256 == "" || !strings.Contains(read.Content, "# Testing") {
		t.Fatalf("unexpected read result: %+v", read)
	}
	content := read.Content + "\n## Repo-specific evidence\n\n- Evidence: test updated through project_docs_update.\n"
	if _, err := UpdateProjectDoc(ProjectDocsUpdateRequest{
		RepoRoot: root,
		RelPath:  "TESTING.md",
		Content:  content,
		Summary:  "record repo-specific testing evidence",
		Confirm:  true,
	}); err == nil || !strings.Contains(err.Error(), "expected_sha256 is required") {
		t.Fatalf("expected missing sha error, got %v", err)
	}
	dry, err := UpdateProjectDoc(ProjectDocsUpdateRequest{
		RepoRoot:       root,
		RelPath:        "TESTING.md",
		Content:        content,
		ExpectedSHA256: read.SHA256,
		Summary:        "record repo-specific testing evidence",
		Evidence:       []string{"internal/core/project_docs_test.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || !dry.DryRun || dry.Action != "update" || dry.NextSHA256 == "" {
		t.Fatalf("unexpected dry-run update: %+v", dry)
	}
	if strings.Contains(mustRead(t, filepath.Join(root, ProjectDocsDir, "TESTING.md")), "Repo-specific evidence") {
		t.Fatalf("dry-run wrote the document")
	}
	written, err := UpdateProjectDoc(ProjectDocsUpdateRequest{
		RepoRoot:       root,
		RelPath:        ".agent-harness/TESTING.md",
		Content:        content,
		ExpectedSHA256: read.SHA256,
		Summary:        "record repo-specific testing evidence",
		Evidence:       []string{"internal/core/project_docs_test.go"},
		Confirm:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !written.OK || written.DryRun || written.Action != "update" {
		t.Fatalf("unexpected written update: %+v", written)
	}
	if !strings.Contains(mustRead(t, filepath.Join(root, ProjectDocsDir, "TESTING.md")), "Repo-specific evidence") {
		t.Fatalf("confirmed update did not write")
	}
}
