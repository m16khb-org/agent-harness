package projectbootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/draftwiki"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
)

func TestBootstrapProjectDocsDryRunAndWrite(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
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
	if _, err := os.Stat(filepath.Join(root, projectdoc.ProjectDocsDir)); !os.IsNotExist(err) {
		t.Fatalf("dry-run created docs dir or unexpected stat error: %v", err)
	}
	wantBootstrapFiles := 1 + len(projectdoc.ProjectDocNames()) + len(draftwiki.DraftWikiSeedFiles())
	if len(dry.Files) != wantBootstrapFiles {
		t.Fatalf("planned files=%d want %d", len(dry.Files), wantBootstrapFiles)
	}
	if projectPlanContainsRel(dry.Files, ".agent-harness/VCS.md") {
		t.Fatalf("optional VCS.md must not be created by bootstrap: %+v", dry.Files)
	}
	for rel := range draftwiki.DraftWikiSeedFiles() {
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
	if !strings.Contains(agents, "# Existing Rules") || !strings.Contains(agents, projectdoc.AgentsStartMarker) || !strings.Contains(agents, ".agent-harness/TESTING.md") || !strings.Contains(agents, ".agent-harness/OPERATIONS.md") {
		t.Fatalf("AGENTS.md marker block not merged correctly:\n%s", agents)
	}
	for _, name := range projectdoc.ProjectDocNames() {
		path := filepath.Join(root, projectdoc.ProjectDocsDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, projectdoc.ProjectDocsDir, "VCS.md")); !os.IsNotExist(err) {
		t.Fatalf("bootstrap unexpectedly created optional VCS.md: %v", err)
	}
	for rel := range draftwiki.DraftWikiSeedFiles() {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected draft-wiki seed %s: %v", rel, err)
		}
	}
	testingDoc := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md"))
	if !strings.Contains(testingDoc, "go test ./...") || !strings.Contains(testingDoc, "Confidence: high") {
		t.Fatalf("TESTING.md lacks inferred command evidence:\n%s", testingDoc)
	}
}

func TestRouteProjectDocsForPreciseTasks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
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
			got, err := projectdocs.RouteProjectDocs(root, tc.task)
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
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md"), "# Custom Testing\n\nKeep local detail.\n")
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md")); !strings.Contains(got, "Keep local detail.") {
		t.Fatalf("bootstrap without sync replaced existing doc:\n%s", got)
	}
	synced, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true, Sync: true})
	if err != nil {
		t.Fatal(err)
	}
	if !synced.Sync {
		t.Fatalf("sync flag not reflected in result: %+v", synced)
	}
	if got := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md")); strings.Contains(got, "Keep local detail.") {
		t.Fatalf("bootstrap --sync did not refresh existing doc:\n%s", got)
	}
}

func TestBootstrapWritesMetaFrontmatterIntoCreatedDocs(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	arch := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "ARCHITECTURE.md"))
	canonical, _ := projectdoc.DocMetaDescription("ARCHITECTURE.md")
	if !strings.HasPrefix(arch, "---\nname: ARCHITECTURE.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("created doc missing canonical frontmatter:\n%s", firstLines(arch, 4))
	}
}

func TestBootstrapAddsFrontmatterToExistingDocPreservingBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	dir := filepath.Join(root, projectdoc.ProjectDocsDir)
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
	canonical, _ := projectdoc.DocMetaDescription("CONVENTIONS.md")
	if !strings.HasPrefix(got, "---\nname: CONVENTIONS.md\ndescription: "+canonical+"\n---\n") {
		t.Fatalf("frontmatter not added to existing doc:\n%s", firstLines(got, 4))
	}
	if !strings.Contains(got, "손수 쓴 컨벤션") || !strings.Contains(got, "보존되어야 할 본문") {
		t.Fatalf("existing body must be preserved:\n%s", got)
	}
}

func TestProjectDocsBootstrapResultJSONContract(t *testing.T) {
	result := ProjectDocsBootstrapResult{
		OK:       true,
		Kind:     "project_docs_bootstrap",
		RepoRoot: "/repo",
		DocsDir:  "/repo/.agent-harness",
		Write:    true,
		Sync:     true,
		DryRun:   false,
		Signals: projectdocs.ProjectSignals{
			Languages:       []string{"go"},
			PackageManagers: []string{"go"},
			Profile: projectdocs.ProjectProfile{
				VCS: projectdocs.ProjectVCSProfile{
					Provider: "git",
					Hosting:  "github",
				},
				Languages: []string{"go"},
			},
		},
		Files: []projectdoc.ProjectDocsPlannedFile{
			{RelPath: ".agent-harness/ARCHITECTURE.md", Action: "write", SHA256: "abc"},
		},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal project docs bootstrap result: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"repo_root":"/repo"`,
		`"docs_dir":"/repo/.agent-harness"`,
		`"package_managers":["go"]`,
		`"lifecycle_state"`,
		`"rel_path":".agent-harness/ARCHITECTURE.md"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("JSON payload missing %s: %s", want, text)
		}
	}
}
