package projectbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/projectdocs"
	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdoc "issueops/internal/domain/projectdoc"
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
	if !caution.OK || caution.RecordKind != "caution" || !strings.HasPrefix(caution.RelPath, ".issueops/cautions/2") || !strings.HasSuffix(caution.RelPath, "-mcp-route-over-read-fixed.md") || caution.BytesAppended == 0 {
		t.Fatalf("unexpected caution result: %+v", caution)
	}
	cautionRecord := mustRead(t, filepath.Join(root, filepath.FromSlash(caution.RelPath)))
	for _, want := range []string{"MCP route over-read fixed", "Resolution:", "go test ./internal/core"} {
		if !strings.Contains(cautionRecord, want) {
			t.Fatalf("caution record missing %q:\n%s", want, cautionRecord)
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
	if !adr.OK || adr.RecordKind != "adr" || !strings.HasPrefix(adr.RelPath, ".issueops/adr/2") || !strings.HasSuffix(adr.RelPath, "-use-task-routed-project-docs.md") || adr.BytesAppended == 0 {
		t.Fatalf("unexpected adr result: %+v", adr)
	}
	adrRecord := mustRead(t, filepath.Join(root, filepath.FromSlash(adr.RelPath)))
	for _, want := range []string{"Use task-routed project docs", "Decision:", "Always inject every document"} {
		if !strings.Contains(adrRecord, want) {
			t.Fatalf("adr record missing %q:\n%s", want, adrRecord)
		}
	}
	// Single-path contract: even in a repo with no manifest and no bootstrap,
	// append never falls back to writing the family root files.
	for _, absent := range []string{"CAUTIONS.md", "ADR.md"} {
		if _, err := os.Stat(filepath.Join(root, projectdoc.ProjectDocsDir, absent)); !os.IsNotExist(err) {
			t.Fatalf("append must not create or write the family root %s: %v", absent, err)
		}
	}
}

func TestReadAndReviseProjectDocRequireSHAConsensus(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	read, err := projectdocs.ReadProjectDoc(root, ".issueops/TESTING.md")
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
		RelPath:        ".issueops/TESTING.md",
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

func TestAppendWritesRecordFileAndPreservesRootIndex(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/modular\n")
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	adrRoot := filepath.Join(root, projectdoc.ProjectDocsDir, "ADR.md")
	rootSHA := projectdoc.SHA256Hex(mustRead(t, adrRoot))

	res, err := projectdocs.AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{
		RepoRoot: root,
		Kind:     "adr",
		Title:    "Folder-first project docs",
		Summary:  "Bootstrap creates root indexes and module starters together.",
		Decision: "Families are created modular from the first bootstrap write.",
		Source:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.RelPath, ".issueops/adr/2") || !strings.HasSuffix(res.RelPath, "-folder-first-project-docs.md") {
		t.Fatalf("modular append did not route to a dated record file: %+v", res)
	}
	record := mustRead(t, filepath.Join(root, filepath.FromSlash(res.RelPath)))
	for _, want := range []string{"name: ", "description: Accepted decision record", "# Folder-first project docs", "- Kind: `adr`", "- Decision:"} {
		if !strings.Contains(record, want) {
			t.Fatalf("record file missing %q:\n%s", want, record)
		}
	}
	// The root index must remain byte-identical: append never churns root SHA.
	if got := projectdoc.SHA256Hex(mustRead(t, adrRoot)); got != rootSHA {
		t.Fatalf("modular append modified the family root index")
	}
	// Same-title collision must produce a distinct file, not overwrite.
	second, err := projectdocs.AppendProjectDocsEntry(projectdocscontract.ProjectDocsAppendRequest{
		RepoRoot: root,
		Kind:     "adr",
		Title:    "Folder-first project docs",
		Summary:  "Collision probe.",
		Source:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.RelPath == res.RelPath || second.SHA256 == res.SHA256 {
		t.Fatalf("record collision overwrote the first file: %+v vs %+v", res, second)
	}
}

func TestRouteAttachesFamilyOverviewInModularRepo(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/routed\n")
	if _, err := BootstrapProjectDocs(ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}); err != nil {
		t.Fatal(err)
	}
	route, err := projectdocs.RouteProjectDocs(root, "test")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, doc := range route.Docs {
		found[doc.RelPath] = true
	}
	if !found[".issueops/TESTING.md"] || !found[".issueops/testing/overview.md"] {
		t.Fatalf("route did not attach the family overview module: %#v", route.Docs)
	}
}
