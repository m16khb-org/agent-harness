package draftwiki

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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

func draftWikiLLMJSONForTest(t *testing.T, bodyMarkdown string) string {
	t.Helper()
	b, err := json.Marshal(DraftWikiSuggestLLMResponse{BodyMarkdown: bodyMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	return "```json\n" + string(b) + "\n```"
}
