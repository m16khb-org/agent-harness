package docupkeep

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/adapter/lifecycle/model"
	corestate "agent-harness/internal/adapter/outbound/state"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func TestAppendWritesJSONL(t *testing.T) {
	plan := lifecyclecontract.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        t.TempDir(),
		RepoID:          "repo-1",
		ProjectStateDir: t.TempDir(),
		QueuePath:       filepath.Join(t.TempDir(), model.DocUpkeepQueueFile),
		Exists:          true,
		NamespaceValid:  true,
	}
	store := Store{
		Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil },
		Init:     func(string, bool) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil },
	}

	result, err := Append(store, plan.RepoRoot, lifecyclecontract.DocUpkeepEvent{
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
	var got lifecyclecontract.DocUpkeepEvent
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

func TestReadPendingFiltersLimitsAndSkipsMalformedLines(t *testing.T) {
	plan := docUpkeepPlanForTest(t)
	lines := []string{
		`{"id":"done","kind":"docs","summary":"done","status":"done"}`,
		`not-json`,
		`{"id":"pending-1","kind":"docs","summary":"pending one","status":"pending"}`,
		`{"id":"blank-status","kind":"docs","summary":"blank status"}`,
		`{"id":"pending-2","kind":"docs","summary":"pending two","status":"pending"}`,
	}
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.QueuePath, []byte(joinLines(lines...)), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil }}

	events, gotPlan, err := ReadPending(store, plan.RepoRoot, 2)
	if err != nil {
		t.Fatal(err)
	}
	if gotPlan.QueuePath != plan.QueuePath {
		t.Fatalf("plan QueuePath=%q, want %q", gotPlan.QueuePath, plan.QueuePath)
	}
	if len(events) != 2 || events[0].ID != "blank-status" || events[1].ID != "pending-2" {
		t.Fatalf("events=%+v, want last two pending/blank-status events", events)
	}
}

func TestReadPendingCompactsQueueToPendingEvents(t *testing.T) {
	plan := docUpkeepPlanForTest(t)
	lines := []string{
		`{"id":"done","kind":"docs","summary":"done","status":"done"}`,
		`not-json`,
		`{"id":"pending-1","kind":"docs","summary":"pending one","status":"pending"}`,
		`{"id":"resolved","kind":"docs","summary":"resolved","status":"resolved"}`,
		`{"id":"blank-status","kind":"docs","summary":"blank status"}`,
	}
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.QueuePath, []byte(joinLines(lines...)), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil }}

	events, _, err := ReadPending(store, plan.RepoRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events=%+v, want two pending events", events)
	}
	raw, err := os.ReadFile(plan.QueuePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "done") || strings.Contains(string(raw), "resolved") || strings.Contains(string(raw), "not-json") {
		t.Fatalf("queue should compact away resolved and malformed records:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"id":"pending-1"`) || !strings.Contains(string(raw), `"id":"blank-status"`) {
		t.Fatalf("queue should preserve pending records:\n%s", raw)
	}
}

func TestAppendWaitsForDocUpkeepLock(t *testing.T) {
	plan := docUpkeepPlanForTest(t)
	store := Store{
		Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil },
		Init:     func(string, bool) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil },
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	if err := corestate.WithKeyLock(context.Background(), plan.ProjectStateDir, "doc-upkeep", func(context.Context) error {
		go func() {
			close(started)
			_, err := Append(store, plan.RepoRoot, lifecyclecontract.DocUpkeepEvent{
				Kind:    "operation_change",
				Summary: "Hook behavior changed.",
				Source:  "test",
			})
			done <- err
		}()
		<-started
		select {
		case err := <-done:
			t.Fatalf("append should wait for doc-upkeep lock, returned early with %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("append after lock release: %v", err)
	}
}

func TestReadPendingHandlesPlanAndQueueBoundaries(t *testing.T) {
	plan := docUpkeepPlanForTest(t)
	store := Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return plan, nil }}
	events, _, err := ReadPending(store, plan.RepoRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("missing queue returned events=%+v, want empty", events)
	}

	noState := plan
	noState.Exists = false
	store = Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return noState, nil }}
	events, _, err = ReadPending(store, plan.RepoRoot, 0)
	if err != nil || len(events) != 0 {
		t.Fatalf("non-existent lifecycle returned events=%+v err=%v", events, err)
	}

	badNamespace := plan
	badNamespace.NamespaceValid = false
	store = Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) { return badNamespace, nil }}
	events, _, err = ReadPending(store, plan.RepoRoot, 0)
	if err != nil || len(events) != 0 {
		t.Fatalf("namespace mismatch returned events=%+v err=%v", events, err)
	}

	store = Store{Validate: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
		return plan, fmt.Errorf("validate failed")
	}}
	events, _, err = ReadPending(store, plan.RepoRoot, 0)
	if err == nil || err.Error() != "validate failed" || len(events) != 0 {
		t.Fatalf("validate failure events=%+v err=%v", events, err)
	}
}

func TestNormalizeTargetDocs(t *testing.T) {
	got := NormalizeTargetDocs([]string{
		" OPERATIONS.md ",
		".agent-harness/OPERATIONS.md",
		".agent-harness/CONVENTIONS.md",
		"notes.txt",
		"",
		"CONVENTIONS.md",
	})
	want := []string{"CONVENTIONS.md", "OPERATIONS.md"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTargetDocs=%v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTargetDocs=%v, want %v", got, want)
		}
	}
}

func docUpkeepPlanForTest(t *testing.T) lifecyclecontract.ProjectLifecycleStatePlan {
	t.Helper()
	stateDir := t.TempDir()
	return lifecyclecontract.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        t.TempDir(),
		RepoID:          "repo-1",
		ProjectStateDir: stateDir,
		QueuePath:       filepath.Join(stateDir, model.DocUpkeepQueueFile),
		Exists:          true,
		NamespaceValid:  true,
	}
}

func joinLines(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}
