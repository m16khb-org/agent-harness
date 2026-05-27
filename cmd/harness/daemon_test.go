package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeMCPStreamListsLLMWikiTools(t *testing.T) {
	root := t.TempDir()
	writeMCPWikiFixture(t, root)
	t.Setenv("LLM_WIKI_ROOT", root)
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://llm-wiki/session-context"}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	var diag bytes.Buffer
	if err := serveMCPStream(strings.NewReader(input), &out, &diag); err != nil {
		t.Fatal(err)
	}
	lines := splitLines(out.String())
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("invalid json %q: %v", line, err)
		}
		if _, ok := obj["result"]; !ok {
			t.Fatalf("missing result: %s", line)
		}
	}
	if !strings.Contains(out.String(), "llm_wiki_session_context") || !strings.Contains(out.String(), "LLM Wiki Session Context") {
		t.Fatalf("missing llm wiki tools/context in output:\n%s", out.String())
	}
}

func TestDaemonPathsUseOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_DAEMON_DIR", dir)
	paths, err := currentDaemonPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.Dir != dir || filepath.Base(paths.Socket) != "agent-harness.sock" || filepath.Base(paths.PID) != "agent-harness.pid" {
		t.Fatalf("unexpected daemon paths: %+v", paths)
	}
}

func writeMCPWikiFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"00-meta/AGENTS.md":                    "# Schema\n",
		"00-meta/index.md":                     "# Wiki Index\n\n- [[llm-wiki-pattern]]\n",
		"00-meta/log.md":                       "# Log\n",
		"20-wiki/concepts/llm-wiki-pattern.md": "---\ntitle: LLM Wiki Pattern\ntype: concept\nstatus: active\ntags: [llm-wiki]\n---\n\nDurable wiki memory.\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
