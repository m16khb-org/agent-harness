package draftwikicli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/cmd/harness/workercli"
	"agent-harness/internal/core"
)

func TestRunProjectDraftWikiPruneJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, err := core.AppendDraftWikiQueueEvent(core.DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: "cli prune material",
		}); err != nil {
			t.Fatal(err)
		}
	}

	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"prune", "--repo", root, "--keep", "1", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("prune returned invalid JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["kind"] != "draft_wiki_queue_prune" {
		t.Fatalf("unexpected prune result: %#v", result)
	}
	if result["before"] != float64(3) || result["after"] != float64(1) || result["pruned"] != float64(2) {
		t.Fatalf("unexpected prune counts: %#v", result)
	}
	if path, _ := result["path"].(string); !strings.HasSuffix(path, "draft-wiki-queue.jsonl") {
		t.Fatalf("missing queue path in result: %#v", result)
	}
}

func TestRunProjectDraftWikiQueueAndWorkerWritesDraft(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	materialPath := filepath.Join(root, "material.md")
	if err := os.WriteFile(materialPath, []byte("The main agent judged this hook policy reusable enough for draft wiki."), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "agy-settings.json")
	if err := os.WriteFile(configPath, []byte(`{"model":"Gemini 3.5 Flash (High)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeAgy := filepath.Join(root, "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo "missing agy flags" >&2
  exit 2
fi
cat <<'EOF'
{"body_markdown":"---\ntitle: \"Explicit queued draft\"\nsource: \"main-agent\"\ntarget_wiki: \"agent-harness\"\ntarget_type: \"notes\"\nsummary: \"The main agent explicitly queues reusable draft-wiki material.\"\n---\n\n# Explicit queued draft\n\nThe main agent, not a heuristic hook, decides whether material is worth queueing."}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}

	queuedOut := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"queue", "--repo", root, "--input", materialPath, "--target-wiki", "agent-harness", "--json"})
	})
	var queued map[string]any
	if err := json.Unmarshal([]byte(queuedOut), &queued); err != nil {
		t.Fatalf("queue returned invalid JSON: %v\n%s", err, queuedOut)
	}
	if queued["ok"] != true {
		t.Fatalf("unexpected queue result: %#v", queued)
	}

	out := captureStdoutForContract(t, func() error {
		return workercli.Run([]string{"draft-wiki", "--repo", root, "--agy-command", fakeAgy, "--agy-settings", configPath, "--json"})
	})
	var processed map[string]any
	if err := json.Unmarshal([]byte(out), &processed); err != nil {
		t.Fatalf("worker output is not JSON: %q: %v", out, err)
	}
	if processed["processed"] != float64(1) || processed["succeeded"] != float64(1) {
		t.Fatalf("worker did not process explicitly queued draft-wiki item: %+v", processed)
	}
	wantDraftName := time.Now().Format(time.DateOnly) + "-explicit-queued-draft.md"
	if _, err := os.Stat(filepath.Join(root, ".agent-harness", "draft-wiki", "draft", wantDraftName)); err != nil {
		t.Fatalf("draft-wiki draft file missing after explicit queue+worker: %v", err)
	}
}

func TestRunProjectDraftWikiQueueReadsStdin(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
	if _, err := w.WriteString("Heredoc material judged reusable by the main agent."); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runProjectDraftWiki([]string{"queue", "--repo", root, "--stdin", "--target-wiki", "agent-harness", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("queue returned invalid JSON: %v\n%s", err, out)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected queue result: %#v", result)
	}
	path, ok := result["path"].(string)
	if !ok || path == "" {
		t.Fatalf("missing queue path in result: %#v", result)
	}
	rawQueue, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(rawQueue, &event); err != nil {
		t.Fatalf("queue event is not valid JSON: %v\n%s", err, rawQueue)
	}
	if event["source"] != "main-agent" || event["source_material"] != "Heredoc material judged reusable by the main agent." {
		t.Fatalf("stdin queue did not preserve main-agent source/material: %+v", event)
	}
}
