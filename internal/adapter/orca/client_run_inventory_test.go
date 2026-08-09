package orca

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-harness/internal/port"
)

func TestClientRunInventoryReaderUsesExactPerRunQueries(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runtime":{"runtimeId":"runtime-1"}}}`)}
	runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runs":[{"id":"run_b","objective":"b"},{"id":"run_a","objective":"a"}]},"_meta":{"runtimeId":"runtime-1"}}`)}
	for _, runID := range []string{"run_a", "run_b"} {
		runner.responses["orca orchestration task-list --brief --run "+runID+" --json"] = taskListOutput(runID, "ready")
		runner.responses["orca orchestration task-list --status dispatched --run "+runID+" --json"] = taskListOutput(runID, "dispatched")
		runner.responses["orca orchestration gate-list --run "+runID+" --json"] = gateListOutput(runID)
	}
	client := NewClient(runner)

	inventory, err := client.ListRunInventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if inventory.RuntimeID != "runtime-1" || len(inventory.Runs) != 2 || inventory.Runs[0].ID != "run_a" || inventory.Runs[1].ID != "run_b" {
		t.Fatalf("run inventory = %#v", inventory)
	}
	all, err := client.ListAllTasksFromRuns(context.Background(), inventory)
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := client.ListDispatchedTasksFromRuns(context.Background(), inventory)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := client.ListGatesFromRuns(context.Background(), inventory)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].RunID != "run_a" || all[1].RunID != "run_b" || len(dispatched) != 2 || dispatched[0].Status != "dispatched" || len(gates) != 2 || gates[0].ID != "gate-run_a" || gates[1].ID != "gate-run_b" {
		t.Fatalf("all=%#v dispatched=%#v gates=%#v", all, dispatched, gates)
	}
	want := []string{
		"orca orchestration task-list --brief --run run_a --json",
		"orca orchestration task-list --brief --run run_b --json",
		"orca orchestration task-list --status dispatched --run run_a --json",
		"orca orchestration task-list --status dispatched --run run_b --json",
		"orca orchestration gate-list --run run_a --json",
		"orca orchestration gate-list --run run_b --json",
	}
	got := make([]string, 0, len(runner.calls))
	for _, call := range runner.calls {
		got = append(got, strings.Join(call, " "))
	}
	for _, command := range want {
		if !slices.Contains(got, command) {
			t.Fatalf("missing exact per-Run query %q in %#v", command, got)
		}
	}
}

func TestClientRunInventoryRejectsZeroRunRuntimeMismatch(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runtime":{"runtimeId":"runtime-current"}}}`)}
	runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runs":[]},"_meta":{"runtimeId":"runtime-stale"}}`)}

	_, err := NewClient(runner).ListRunInventory(context.Background())
	if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
		t.Fatalf("zero-Run runtime mismatch error = %v", err)
	}
}

func TestClientRunInventoryReaderFailsClosed(t *testing.T) {
	t.Run("duplicate snapshot Run", func(t *testing.T) {
		client := NewClient(newFakeRunner(t))
		_, err := client.ListAllTasksFromRuns(context.Background(), port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{{RuntimeID: "runtime-1", ID: "run_a", Objective: "a"}, {RuntimeID: "runtime-1", ID: "run_a", Objective: "a"}}})
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("duplicate snapshot error = %v", err)
		}
	})

	t.Run("task runtime mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration task-list --brief --run run_a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[],"count":0},"_meta":{"runtimeId":"runtime-other"}}`)}
		_, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), oneRunInventory())
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("task runtime mismatch error = %v", err)
		}
	})

	t.Run("task Run identity mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration task-list --brief --run run_a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[{"id":"task-1","run_id":"run_b","status":"ready"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
		_, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), oneRunInventory())
		if err == nil || !strings.Contains(err.Error(), "different Run") {
			t.Fatalf("task Run identity mismatch error = %v", err)
		}
	})

	t.Run("gate count mismatch", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration gate-list --run run_a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
		_, err := NewClient(runner).ListGatesFromRuns(context.Background(), oneRunInventory())
		if err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("gate count mismatch error = %v", err)
		}
	})

	t.Run("duplicate gate identity", func(t *testing.T) {
		runner := newFakeRunner(t)
		for _, runID := range []string{"run_a", "run_b"} {
			runner.responses["orca orchestration gate-list --run "+runID+" --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[{"id":"gate-1","task_id":"task-1","status":"pending"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
		}
		_, err := NewClient(runner).ListGatesFromRuns(context.Background(), twoRunInventory())
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("duplicate gate identity error = %v", err)
		}
	})
}

func TestClientRunInventoryReaderReturnsLowestRunIndexError(t *testing.T) {
	runner := newFakeRunner(t)
	runner.errors["orca orchestration task-list --brief --run run_a --json"] = errors.New("run_a failed")
	runner.errors["orca orchestration task-list --brief --run run_b --json"] = errors.New("run_b failed")

	_, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), twoRunInventory())
	if err == nil || !strings.Contains(err.Error(), "run_a failed") {
		t.Fatalf("lowest Run index error = %v", err)
	}
}

func TestClientProbeRequiresRunScopedGateCapability(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca orchestration gate-list --help"] = CommandOutput{Stdout: []byte("--json")}

	got, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex", Provider: "github"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready || got.Code != "capability_missing" {
		t.Fatalf("missing gate --run capability was accepted: %#v", got)
	}
}

func TestClientRunInventoryReadersLimitConcurrencyToEight(t *testing.T) {
	tests := []struct {
		name string
		read func(*Client, context.Context, port.OrcaRunInventory) error
	}{
		{name: "all", read: func(client *Client, ctx context.Context, inventory port.OrcaRunInventory) error {
			_, err := client.ListAllTasksFromRuns(ctx, inventory)
			return err
		}},
		{name: "dispatched", read: func(client *Client, ctx context.Context, inventory port.OrcaRunInventory) error {
			_, err := client.ListDispatchedTasksFromRuns(ctx, inventory)
			return err
		}},
		{name: "gates", read: func(client *Client, ctx context.Context, inventory port.OrcaRunInventory) error {
			_, err := client.ListGatesFromRuns(ctx, inventory)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newBlockingRunInventoryRunner(9)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- test.read(NewClient(runner), ctx, runner.inventory()) }()
			defer runner.unblock()
			for range 8 {
				select {
				case <-runner.started:
				case <-ctx.Done():
					t.Fatalf("reader did not start eight concurrent Run queries: %v", ctx.Err())
				}
			}
			if peak := runner.peak(); peak != 8 {
				t.Fatalf("peak concurrent Run queries = %d, want 8", peak)
			}
			runner.unblock()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if peak := runner.peak(); peak > 8 {
				t.Fatalf("concurrency limit exceeded: %d", peak)
			}
		})
	}
}

func taskListOutput(runID, status string) CommandOutput {
	return CommandOutput{Stdout: []byte(fmt.Sprintf(`{"ok":true,"result":{"tasks":[{"id":"task-%s","status":"%s"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`, runID, status))}
}

func gateListOutput(runID string) CommandOutput {
	return CommandOutput{Stdout: []byte(fmt.Sprintf(`{"ok":true,"result":{"gates":[{"id":"gate-%s","task_id":"task-%s","status":"pending"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`, runID, runID))}
}

func oneRunInventory() port.OrcaRunInventory {
	return port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{{RuntimeID: "runtime-1", ID: "run_a", Objective: "a"}}}
}

func twoRunInventory() port.OrcaRunInventory {
	return port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{{RuntimeID: "runtime-1", ID: "run_a", Objective: "a"}, {RuntimeID: "runtime-1", ID: "run_b", Objective: "b"}}}
}

type blockingRunInventoryRunner struct {
	runs    []port.OrcaRun
	started chan struct{}
	release chan struct{}
	once    sync.Once

	mu     sync.Mutex
	active int
	max    int
}

func newBlockingRunInventoryRunner(count int) *blockingRunInventoryRunner {
	runs := make([]port.OrcaRun, count)
	for index := range runs {
		runs[index] = port.OrcaRun{RuntimeID: "runtime-1", ID: fmt.Sprintf("run-%02d", index), Objective: fmt.Sprintf("run %02d", index)}
	}
	return &blockingRunInventoryRunner{runs: runs, started: make(chan struct{}, count), release: make(chan struct{})}
}

func (r *blockingRunInventoryRunner) LookPath(string) (string, error) { return "/orca", nil }

func (r *blockingRunInventoryRunner) Run(_ context.Context, _ string, _ time.Duration, argv []string) (CommandOutput, error) {
	if slices.Equal(argv, []string{"orca", "orchestration", "run-list", "--json"}) {
		rows := make([]string, 0, len(r.runs))
		for _, run := range r.runs {
			rows = append(rows, fmt.Sprintf(`{"id":%q,"objective":%q}`, run.ID, run.Objective))
		}
		return CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runs":[` + strings.Join(rows, ",") + `]},"_meta":{"runtimeId":"runtime-1"}}`)}, nil
	}
	r.mu.Lock()
	r.active++
	if r.active > r.max {
		r.max = r.active
	}
	r.mu.Unlock()
	r.started <- struct{}{}
	<-r.release
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	runID := argv[len(argv)-2]
	if slices.Contains(argv, "gate-list") {
		return gateListOutput(runID), nil
	}
	status := "ready"
	if slices.Contains(argv, "dispatched") {
		status = "dispatched"
	}
	return taskListOutput(runID, status), nil
}

func (r *blockingRunInventoryRunner) inventory() port.OrcaRunInventory {
	return port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: append([]port.OrcaRun(nil), r.runs...)}
}

func (r *blockingRunInventoryRunner) unblock() { r.once.Do(func() { close(r.release) }) }

func (r *blockingRunInventoryRunner) peak() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.max
}
