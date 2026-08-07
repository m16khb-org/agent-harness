package projectbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
)

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

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func containsProjectCommand(commands []projectdocs.EvidenceCommand, command string) bool {
	for _, c := range commands {
		if c.Command == command {
			return true
		}
	}
	return false
}

func projectPlanContainsRel(files []projectdoc.ProjectDocsPlannedFile, rel string) bool {
	for _, file := range files {
		if file.RelPath == rel {
			return true
		}
	}
	return false
}

func routeContains(result projectdocs.ProjectDocsRouteResult, rel string) bool {
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
