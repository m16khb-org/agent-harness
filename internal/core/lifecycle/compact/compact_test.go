package compact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestPreAndPostCompactPreservePendingUpkeep(t *testing.T) {
	stateDir := t.TempDir()
	plan := compactPlanForTest(t, stateDir)
	store := compactStoreForTest(plan, []model.DocUpkeepEvent{{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md", "TESTING.md"},
		Summary:    "Hook and tests changed.",
		Source:     "test",
	}})

	pre := BuildPreCompactCapsule(store, plan.RepoRoot)
	if !pre.OK || !pre.Recorded || pre.PendingCount != 1 || pre.CompactPath == "" {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); err != nil {
		t.Fatalf("compact capsule missing: %v", err)
	}

	post := BuildPostCompactReminder(store, plan.RepoRoot)
	if !post.OK || !post.ShouldInject || post.PendingCount != 1 {
		t.Fatalf("unexpected post-compact result: %+v", post)
	}
	for _, want := range []string{"Restored agent-harness compaction capsule", "OPERATIONS.md", "TESTING.md", "UserPromptSubmit will keep surfacing the current details"} {
		if !strings.Contains(post.AdditionalContext, want) {
			t.Fatalf("post-compact context missing %q: %s", want, post.AdditionalContext)
		}
	}
	if strings.Contains(post.AdditionalContext, "Hook and tests changed.") {
		t.Fatalf("post-compact should not duplicate detailed pending-upkeep summaries:\n%s", post.AdditionalContext)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("post-compact should consume compact capsule, stat error: %v", err)
	}
}

func TestPreCompactNoPendingUpkeepDoesNotWriteCapsule(t *testing.T) {
	plan := compactPlanForTest(t, t.TempDir())
	store := compactStoreForTest(plan, nil)

	pre := BuildPreCompactCapsule(store, plan.RepoRoot)
	if !pre.OK || pre.Recorded || pre.PendingCount != 0 {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("pre-compact without pending upkeep should not write capsule, stat error: %v", err)
	}
}

func compactPlanForTest(t *testing.T, stateDir string) model.ProjectLifecycleStatePlan {
	t.Helper()
	repo := t.TempDir()
	projectStateDir := filepath.Join(stateDir, "projects", "repo-1")
	return model.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        repo,
		RepoID:          "repo-1",
		ProjectStateDir: projectStateDir,
		CompactPath:     filepath.Join(projectStateDir, model.CompactCapsuleFile),
		Exists:          true,
		NamespaceValid:  true,
	}
}

func compactStoreForTest(plan model.ProjectLifecycleStatePlan, events []model.DocUpkeepEvent) Store {
	return Store{
		ReadPending: func(string, int) ([]model.DocUpkeepEvent, model.ProjectLifecycleStatePlan, error) {
			return events, plan, nil
		},
		Validate: func(string) (model.ProjectLifecycleStatePlan, error) {
			return plan, nil
		},
		WriteJSON: writeJSONForTest,
	}
}

func writeJSONForTest(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), perm)
}
