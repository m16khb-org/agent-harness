package projectbootstrap

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/projectdocs"
	projectdocscontract "agent-harness/internal/contract/projectdocs"
	projectdoc "agent-harness/internal/domain/projectdoc"
)

func TestAppendProjectDocsEntryWritesCautionsAndADR(t *testing.T) {
	root := t.TempDir()
	caution, err := projectdocs.AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{
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
	cautions := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "CAUTIONS.md"))
	for _, want := range []string{"MCP route over-read fixed", "Resolution:", "go test ./internal/core"} {
		if !strings.Contains(cautions, want) {
			t.Fatalf("CAUTIONS.md missing %q:\n%s", want, cautions)
		}
	}

	adr, err := projectdocs.AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{
		RepoRoot:     root,
		Kind:         "adr",
		Title:        "Use task-routed project docs",
		Summary:      "Agents should retrieve only the documents relevant to the current task.",
		Decision:     "Expose project_docs_route and project_docs_append through MCP.",
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
	adrDoc := mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "ADR.md"))
	for _, want := range []string{"Use task-routed project docs", "Decision:", "Always inject every document"} {
		if !strings.Contains(adrDoc, want) {
			t.Fatalf("ADR.md missing %q:\n%s", want, adrDoc)
		}
	}
}

func TestReadAndReviseProjectDocRequireSHAConsensus(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	read, err := projectdocs.ReadProjectDoc(root, ".agent-harness/TESTING.md")
	if err != nil {
		t.Fatal(err)
	}
	if !read.OK || !read.Exists || read.SHA256 == "" || !strings.Contains(read.Content, "# Testing") {
		t.Fatalf("unexpected read result: %+v", read)
	}
	content := read.Content + "\n## Repo-specific evidence\n\n- Evidence: test updated through project_docs_revise.\n"
	if _, err := projectdocs.ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{
		RepoRoot: root,
		RelPath:  "TESTING.md",
		Content:  content,
		Summary:  "record repo-specific testing evidence",
		Confirm:  true,
	}); err == nil || !strings.Contains(err.Error(), "expected_sha256 is required") {
		t.Fatalf("expected missing sha error, got %v", err)
	}
	dry, err := projectdocs.ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{
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
	if strings.Contains(mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md")), "Repo-specific evidence") {
		t.Fatalf("dry-run wrote the document")
	}
	written, err := projectdocs.ReviseProjectDoc(projectdocscontract.ProjectDocsReviseRequest{
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
	if !strings.Contains(mustRead(t, filepath.Join(root, projectdoc.ProjectDocsDir, "TESTING.md")), "Repo-specific evidence") {
		t.Fatalf("confirmed update did not write")
	}
}
