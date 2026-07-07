package workpool

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core/sqlstore"
)

func TestWorkPoolRoundTripAndOmitempty(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool, err := CreatePool(CreatePoolRequest{Repo: t.TempDir(), Name: "round-trip"})
	if err != nil {
		t.Fatal(err)
	}
	if pool.SchemaVersion != WorkPoolCurrentSchemaVersion || pool.ID == "" || pool.CreatedAt == "" || pool.UpdatedAt == "" {
		t.Fatalf("pool missing durable metadata: %#v", pool)
	}

	data, err := json.Marshal(pool)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "parent_cycle_id") {
		t.Fatalf("empty optional parent_cycle_id should be omitted: %s", data)
	}

	read, err := ReadPool(pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !read.OK || read.ID != pool.ID || read.Repo != pool.Repo || read.LeaseTTL != "15m" || read.MaxAttempts != 2 {
		t.Fatalf("pool round-trip mismatch: got %#v want %#v", read, pool)
	}
}

func TestAddTaskRoundTripListAndRedaction(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool, err := CreatePool(CreatePoolRequest{Repo: t.TempDir(), Name: "tasks"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := AddTask(pool.ID, AddTaskRequest{
		Title:              "first task",
		Instructions:       "use token=super-secret-value",
		Scope:              []string{"internal/core/workpool"},
		AcceptanceCriteria: []string{"tests pass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.PoolID != pool.ID || first.Status != "pending" || first.Attempts != 0 {
		t.Fatalf("task missing durable defaults: %#v", first)
	}
	if strings.Contains(first.Instructions, "super-secret-value") || !strings.Contains(first.Instructions, "<redacted>") {
		t.Fatalf("task instructions should redact secret-like assignments, got %q", first.Instructions)
	}
	if _, err := AddTask(pool.ID, AddTaskRequest{Title: "second task", Instructions: "plain"}); err != nil {
		t.Fatal(err)
	}

	read, err := ReadTask(pool.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != first.ID || read.Instructions != first.Instructions {
		t.Fatalf("task round-trip mismatch: got %#v want %#v", read, first)
	}
	tasks, err := ListTasks(pool.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Title != "first task" || tasks[1].Title != "second task" {
		t.Fatalf("ListTasks should return deterministic task order, got %#v", tasks)
	}
}

func TestReadPoolRefusesFutureSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool, err := CreatePool(CreatePoolRequest{Repo: t.TempDir(), Name: "future-schema"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(StateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("pool", pool.ID, []byte(`{"ok":true,"schema_version":99,"id":"`+pool.ID+`"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPool(pool.ID); err == nil || !strings.Contains(err.Error(), "unsupported workpool schema_version") {
		t.Fatalf("ReadPool err=%v, want future schema refusal", err)
	}
}

func TestReadTaskRefusesFutureSchema(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	pool, err := CreatePool(CreatePoolRequest{Repo: t.TempDir(), Name: "future-task"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := AddTask(pool.ID, AddTaskRequest{Title: "future task", Instructions: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(StateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("task:"+pool.ID, task.ID, []byte(`{"ok":true,"schema_version":99,"id":"`+task.ID+`","pool_id":"`+pool.ID+`"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTask(pool.ID, task.ID); err == nil || !strings.Contains(err.Error(), "unsupported workpool schema_version") {
		t.Fatalf("ReadTask err=%v, want future schema refusal", err)
	}
}
