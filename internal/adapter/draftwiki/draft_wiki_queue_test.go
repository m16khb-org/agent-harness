package draftwiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftWikiQueueAppendCapsTailAfterSlack(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()

	for i := 0; i < maxDraftWikiQueueEvents*2+1; i++ {
		if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: fmt.Sprintf("queue material %03d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	_, path, err := draftWikiQueuePath(root, false)
	if err != nil {
		t.Fatal(err)
	}
	events, warnings, err := readDraftWikiQueueEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", warnings)
	}
	if len(events) != maxDraftWikiQueueEvents {
		t.Fatalf("queue length=%d want %d", len(events), maxDraftWikiQueueEvents)
	}
	if got, want := events[0].SourceMaterial, "queue material 201"; got != want {
		t.Fatalf("oldest retained material=%q want %q", got, want)
	}
	if got, want := events[len(events)-1].SourceMaterial, "queue material 400"; got != want {
		t.Fatalf("newest retained material=%q want %q", got, want)
	}
}

func TestPruneDraftWikiQueueKeepZeroAndTail(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()

	for i := 0; i < 5; i++ {
		if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
			RepoRoot:       root,
			SourceMaterial: fmt.Sprintf("prune material %d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	tail, err := PruneDraftWikiQueue(root, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !tail.OK || tail.Before != 5 || tail.After != 2 || tail.Pruned != 3 {
		t.Fatalf("unexpected tail prune result: %+v", tail)
	}
	events, warnings, err := readDraftWikiQueueEvents(tail.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(events) != 2 {
		t.Fatalf("unexpected queue after tail prune: events=%+v warnings=%+v", events, warnings)
	}
	if events[0].SourceMaterial != "prune material 3" || events[1].SourceMaterial != "prune material 4" {
		t.Fatalf("prune did not retain newest events: %+v", events)
	}

	empty, err := PruneDraftWikiQueue(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Before != 2 || empty.After != 0 || empty.Pruned != 2 {
		t.Fatalf("unexpected empty prune result: %+v", empty)
	}
	stat, err := os.Stat(empty.Path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size() != 0 || stat.Mode().Perm() != 0o600 {
		t.Fatalf("expected empty 0600 queue file, size=%d mode=%o", stat.Size(), stat.Mode().Perm())
	}
}

func TestPruneAllDraftWikiQueues(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	rootA := t.TempDir()
	rootB := t.TempDir()

	for _, root := range []string{rootA, rootB} {
		for i := 0; i < 3; i++ {
			if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
				RepoRoot:       root,
				SourceMaterial: fmt.Sprintf("%s material %d", filepath.Base(root), i),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	result, err := PruneAllDraftWikiQueues(stateDir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || len(result.Queues) != 2 || result.Before != 6 || result.After != 2 || result.Pruned != 4 {
		t.Fatalf("unexpected prune-all result: %+v", result)
	}
	for _, queue := range result.Queues {
		events, warnings, err := readDraftWikiQueueEvents(queue.Path)
		if err != nil {
			t.Fatal(err)
		}
		if len(warnings) != 0 || len(events) != 1 {
			t.Fatalf("unexpected queue after prune-all: queue=%+v events=%+v warnings=%+v", queue, events, warnings)
		}
		if !strings.HasSuffix(events[0].SourceMaterial, " material 2") {
			t.Fatalf("prune-all did not retain newest event: %+v", events[0])
		}
	}
}

func TestDraftWikiQueueRewritePreservesRedactionAndMaterialCap(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	longSecret := "token=secret-value\n" + strings.Repeat("x", 13000)
	queued, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		SourceMaterial: "old material",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDraftWikiQueueEvent(DraftWikiQueueAppendRequest{
		RepoRoot:       root,
		SourceMaterial: longSecret,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := PruneDraftWikiQueue(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != queued.Path || result.After != 1 {
		t.Fatalf("unexpected prune result: %+v", result)
	}
	events, warnings, err := readDraftWikiQueueEvents(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(events) != 1 {
		t.Fatalf("unexpected queue after prune: events=%+v warnings=%+v", events, warnings)
	}
	material := events[0].SourceMaterial
	if strings.Contains(material, "secret-value") {
		t.Fatalf("secret leaked after rewrite: %q", material)
	}
	if !strings.Contains(material, "<redacted>") {
		t.Fatalf("redaction marker missing after rewrite: %q", material)
	}
	if len([]byte(material)) > 12020 || !strings.Contains(material, "[truncated]") {
		t.Fatalf("material cap not preserved after rewrite, bytes=%d material=%q", len([]byte(material)), material)
	}
	stat, err := os.Stat(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("queue rewrite mode=%o want 600", stat.Mode().Perm())
	}
}
