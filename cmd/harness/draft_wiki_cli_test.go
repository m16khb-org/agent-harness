package main

import (
	"encoding/json"
	"strings"
	"testing"

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
