package draftwikicli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDraftWikiCLIDraft(t *testing.T, root, status, name, title, targetWiki string) string {
	t.Helper()
	path := filepath.Join(root, ".agent-harness", "draft-wiki", status, name)
	body := "---\n" +
		"title: \"" + title + "\"\n" +
		"target_wiki: \"" + targetWiki + "\"\n" +
		"target_type: \"notes\"\n" +
		"---\n\n# " + title + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
