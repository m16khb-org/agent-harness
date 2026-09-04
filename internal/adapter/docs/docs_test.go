package docs

import (
	docscontract "issueops/internal/contract/docs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// In a real git repo, an UNTRACKED .md (e.g. a session research artifact) must
// be excluded from the hermetic docs index while TRACKED docs — including ones
// under .issueops/research — stay. t.TempDir() sits under macOS
// /var -> /private/var, so this also guards the symlink-divergence case where
// git's resolved root differs from WalkDir's literal root.
func TestListDocsExcludesUntrackedInGitRepo(t *testing.T) {
	root := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable for hermetic test: %v\n%s", err, out)
		}
	}
	gitRun("init")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Agents\n")
	mustWrite(t, filepath.Join(root, ".issueops", "CONVENTIONS.md"), "# Conventions\n")
	mustWrite(t, filepath.Join(root, ".issueops", "research", "tracked-note.md"), "# Tracked research\n")
	gitRun("add", "AGENTS.md", ".issueops/CONVENTIONS.md", ".issueops/research/tracked-note.md")
	gitRun("commit", "-m", "seed")
	mustWrite(t, filepath.Join(root, ".issueops", "research", "untracked-research.md"), "# Untracked\n")

	index := DocsIndex(root, "test")
	for _, want := range []string{"AGENTS.md", ".issueops/CONVENTIONS.md", ".issueops/research/tracked-note.md"} {
		if !docIndexContains(index.Docs, want) {
			t.Fatalf("tracked doc %s must stay in the hermetic index: %+v", want, index.Docs)
		}
	}
	if docIndexContains(index.Docs, ".issueops/research/untracked-research.md") {
		t.Fatalf("untracked research artifact must be excluded from the hermetic index: %+v", index.Docs)
	}
}

func TestListDocsIncludesUntrackedCanonicalProjectDocs(t *testing.T) {
	root := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable for hermetic test: %v\n%s", err, out)
		}
	}
	gitRun("init")
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Agents\n")
	gitRun("add", "AGENTS.md")
	gitRun("commit", "-m", "seed")

	mustWrite(t, filepath.Join(root, ".issueops", "TESTING.md"), "# Testing\n")
	mustWrite(t, filepath.Join(root, ".issueops", "testing", "unit.md"), "# Unit\n")
	mustWrite(t, filepath.Join(root, ".issueops", "research", "scratch.md"), "# Scratch\n")
	mustWrite(
		t,
		filepath.Join(root, ".issueops", "documentation", "manifest.json"),
		`{"schema_version":1,"families":[{"root":".issueops/TESTING.md","module_dir":".issueops/testing","responsibility":"testing"}]}`,
	)

	index := DocsIndex(root, "test")
	for _, want := range []string{
		".issueops/TESTING.md",
		".issueops/testing/unit.md",
	} {
		if !docIndexContains(index.Docs, want) {
			t.Fatalf("canonical untracked doc %s must be discoverable: %+v", want, index.Docs)
		}
	}
	if docIndexContains(index.Docs, ".issueops/research/scratch.md") {
		t.Fatalf("untracked research artifact must stay excluded: %+v", index.Docs)
	}
}

func TestDocsIndexIncludesAgentDocs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	index := DocsIndex(root, "test")
	if !index.OK {
		t.Fatalf("DocsIndex ok=false: %+v", index)
	}
	for _, want := range []string{"AGENTS.md", "CLAUDE.md", ".issueops/COMMIT_POLICY.md", ".issueops/OPERATIONS.md"} {
		if !docIndexContains(index.Docs, want) {
			t.Fatalf("DocsIndex missing %s: %+v", want, index.Docs)
		}
	}
	for _, doc := range index.Docs {
		if doc.Title == "" {
			t.Fatalf("doc %s has empty title", doc.RelPath)
		}
	}
}

func TestDocsIndexExcludesDraftWiki(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Rules\n")
	mustWrite(t, filepath.Join(root, ".issueops", "CAUTIONS.md"), "# Cautions\n")
	mustWrite(t, filepath.Join(root, ".issueops", "draft-wiki", "draft", "candidate.md"), "# Draft candidate\n")

	index := DocsIndex(root, "test")
	if !docIndexContains(index.Docs, "AGENTS.md") {
		t.Fatalf("DocsIndex missing AGENTS.md: %+v", index.Docs)
	}
	if !docIndexContains(index.Docs, ".issueops/CAUTIONS.md") {
		t.Fatalf("DocsIndex missing CAUTIONS.md: %+v", index.Docs)
	}
	if docIndexContains(index.Docs, ".issueops/draft-wiki/draft/candidate.md") {
		t.Fatalf("DocsIndex included draft-wiki candidate: %+v", index.Docs)
	}
}

func TestDocsIndexExcludesEvidence(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "# Rules\n")
	mustWrite(t, filepath.Join(root, ".issueops", "CAUTIONS.md"), "# Cautions\n")
	// .issueops/evidence is gitignored, working-tree-dependent runtime data.
	// It must never enter the docs index, or the response-contract golden becomes non-hermetic.
	mustWrite(t, filepath.Join(root, ".issueops", "evidence", "pioneer-skills-quality", "baseline.md"), "# Baseline\n")

	index := DocsIndex(root, "test")
	if !docIndexContains(index.Docs, "AGENTS.md") {
		t.Fatalf("DocsIndex missing AGENTS.md: %+v", index.Docs)
	}
	if !docIndexContains(index.Docs, ".issueops/CAUTIONS.md") {
		t.Fatalf("DocsIndex missing CAUTIONS.md: %+v", index.Docs)
	}
	if docIndexContains(index.Docs, ".issueops/evidence/pioneer-skills-quality/baseline.md") {
		t.Fatalf("DocsIndex included gitignored evidence doc: %+v", index.Docs)
	}
}

func docIndexContains(docs []docscontract.DocIndexInfo, relPath string) bool {
	for _, doc := range docs {
		if doc.RelPath == relPath {
			return true
		}
	}
	return false
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
