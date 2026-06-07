package docupkeep

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestAppendWritesJSONL(t *testing.T) {
	plan := model.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        t.TempDir(),
		RepoID:          "repo-1",
		ProjectStateDir: t.TempDir(),
		QueuePath:       filepath.Join(t.TempDir(), model.DocUpkeepQueueFile),
		Exists:          true,
		NamespaceValid:  true,
	}
	store := Store{
		Validate: func(string) (model.ProjectLifecycleStatePlan, error) { return plan, nil },
		Init:     func(string, bool) (model.ProjectLifecycleStatePlan, error) { return plan, nil },
	}

	result, err := Append(store, plan.RepoRoot, model.DocUpkeepEvent{
		Kind:       "operation_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "Hook behavior changed.",
		Evidence:   []string{"internal/core/hook_prompt.go"},
		Source:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Path == "" || result.Event.ID == "" {
		t.Fatalf("unexpected append result: %+v", result)
	}

	file, err := os.Open(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one jsonl record")
	}
	var got model.DocUpkeepEvent
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "operation_change" || got.TargetDocs[0] != "OPERATIONS.md" || got.Status != "pending" {
		t.Fatalf("unexpected event: %+v", got)
	}
	if scanner.Scan() {
		t.Fatalf("expected one event, got extra line: %s", scanner.Text())
	}
}
