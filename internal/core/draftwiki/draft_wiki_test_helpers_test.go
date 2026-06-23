package draftwiki

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTestLLMWikiHub(t *testing.T) (configPath, hub string) {
	t.Helper()
	root := t.TempDir()
	hub = filepath.Join(root, "llm-wiki")
	topic := filepath.Join(hub, "topics", "agent-harness")
	mustWrite(t, filepath.Join(hub, "wikis.json"), `{
  "default": "agent-harness",
  "wikis": {
    "agent-harness": {
      "path": "topics/agent-harness",
      "description": "Agent harness memory",
      "status": "active"
    }
  }
}`)
	mustWrite(t, filepath.Join(topic, "config.md"), "# Agent Harness\n")
	mustWrite(t, filepath.Join(topic, "log.md"), "# Log\n")
	configPath = filepath.Join(root, "llm-wiki-config.json")
	mustWrite(t, configPath, `{"hub_path":"`+filepath.ToSlash(hub)+`"}`)
	return configPath, hub
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

func draftWikiLLMJSONForTest(t *testing.T, bodyMarkdown string) string {
	t.Helper()
	b, err := json.Marshal(DraftWikiSuggestLLMResponse{BodyMarkdown: bodyMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	return "```json\n" + string(b) + "\n```"
}
