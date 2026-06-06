package core

import (
	"path/filepath"
	"testing"
)

func TestKubectlFlagTakesValueRecognizesValueFlags(t *testing.T) {
	for _, flag := range []string{"-n", "--namespace", "--context=prod", "--kubeconfig", "-o", "-l"} {
		if !kubectlFlagTakesValue(flag) {
			t.Fatalf("kubectlFlagTakesValue(%q) = false, want true", flag)
		}
	}

	for _, flag := range []string{"--watch", "--dry-run", "--force"} {
		if kubectlFlagTakesValue(flag) {
			t.Fatalf("kubectlFlagTakesValue(%q) = true, want false", flag)
		}
	}
}

func TestUniqueDocUpkeepEventsDeduplicatesByDocsAndSummary(t *testing.T) {
	events := []DocUpkeepEvent{
		{ID: "a", TargetDocs: []string{"OPERATIONS.md", "CONVENTIONS.md"}, Summary: "sync install docs", Source: "first"},
		{ID: "b", TargetDocs: []string{"CONVENTIONS.md", "OPERATIONS.md"}, Summary: " sync install docs ", Source: "duplicate"},
		{ID: "c", TargetDocs: []string{"TESTING.md"}, Summary: "sync install docs", Source: "distinct docs"},
		{ID: "d", TargetDocs: []string{"OPERATIONS.md"}, Summary: "record daemon behavior", Source: "distinct summary"},
	}

	got := uniqueDocUpkeepEvents(events)
	if len(got) != 3 {
		t.Fatalf("uniqueDocUpkeepEvents length = %d, want 3: %#v", len(got), got)
	}
	if got[0].ID != "a" || got[1].ID != "c" || got[2].ID != "d" {
		t.Fatalf("uniqueDocUpkeepEvents preserved IDs = %q, %q, %q; want a, c, d", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestWorktreeGuardTargetsIncludesRepoAndResolvedPaths(t *testing.T) {
	repo := t.TempDir()
	absPath := filepath.Join(repo, "absolute.go")

	got := worktreeGuardTargets(HookToolUseLifecycleRequest{
		Repo:  repo,
		Paths: []string{"relative.go", absPath, "   "},
	})

	want := []string{
		filepath.Clean(repo),
		filepath.Join(repo, "relative.go"),
		absPath,
	}
	if len(got) != len(want) {
		t.Fatalf("worktreeGuardTargets length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("worktreeGuardTargets[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
