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

func TestClientListRunsProjectsInstalledShape(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "orchestration", "run-list", "--json"}
	runner.responses[strings.Join(argv, " ")] = fixtureOutput(t, "run_list.json")

	got, err := NewClient(runner).ListRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuntimeID != "runtime-1" || got[0].ID != "run_issueops_1" {
		t.Fatalf("run list projection = %#v", got)
	}
	if !slices.Equal(runner.calls[0], argv) {
		t.Fatalf("run-list argv = %#v", runner.calls)
	}
}

func TestClientCreateRunUsesAuthenticatedCurrentCoordinator(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "orchestration", "run-create", "--objective", "agent-harness issueops-v1 lifecycle=io-test generation=1 intent=op-test", "--json"}
	runner.responses[strings.Join(argv, " ")] = fixtureOutput(t, "run_create.json")

	got, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: "agent-harness issueops-v1 lifecycle=io-test generation=1 intent=op-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "run_issueops_1" || got.RuntimeID != "runtime-1" || !slices.Equal(runner.calls[0], argv) {
		t.Fatalf("run-create projection=%#v calls=%#v", got, runner.calls)
	}
}

func TestClientCurrentRunAndUseRunUseAuthenticatedCurrentCoordinator(t *testing.T) {
	runner := newFakeRunner(t)
	currentArgv := []string{"orca", "orchestration", "run-current", "--json"}
	useArgv := []string{"orca", "orchestration", "run-use", "--id", "run_issueops_1", "--json"}
	runner.responses[strings.Join(currentArgv, " ")] = fixtureOutput(t, "run_current.json")
	runner.responses[strings.Join(useArgv, " ")] = fixtureOutput(t, "run_use.json")
	client := NewClient(runner)

	current, err := client.CurrentRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	used, err := client.UseRun(context.Background(), "run_issueops_1")
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != "run_issueops_1" || used.ID != "run_issueops_1" || !slices.Equal(runner.calls[0], currentArgv) || !slices.Equal(runner.calls[1], useArgv) {
		t.Fatalf("current=%#v used=%#v calls=%#v", current, used, runner.calls)
	}
}

func TestClientProbeRejectsRunInventoryFromAnotherRuntime(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca orchestration run-current --json"] = CommandOutput{Stdout: []byte(`{
		"ok":true,
		"result":{"run":null},
		"_meta":{"runtimeId":"runtime-other"}
	}`)}

	got, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready || got.Code != "orchestration_unready" {
		t.Fatalf("다른 runtime의 빈 Run inventory를 readiness로 승인했다: %#v", got)
	}
}

func TestClientProbeTreatsUnrelatedOpaqueRunAsValidInventory(t *testing.T) {
	runner := newFakeRunner(t)
	runner.lookPaths["orca"] = "/usr/local/bin/orca"
	runner.lookPaths["codex"] = "/usr/local/bin/codex"
	runner.responses["orca status --json"] = fixtureOutput(t, "status_ready.json")
	runner.responses["orca repo show --repo path:/repo --json"] = fixtureOutput(t, "repo_show.json")
	addCompleteProbeLeafHelp(runner)
	runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{
		"ok":true,
		"result":{"runs":[
			{"id":"run_legacy_local","objective":"Legacy orchestration state (inspect only)","legacy":1}
		]},
		"_meta":{"runtimeId":"runtime-1"}
	}`)}

	got, err := NewClient(runner).Probe(context.Background(), port.OrcaProbeRequest{Repo: "/repo", Agent: "codex", Provider: "github"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Ready || got.Code != "" {
		t.Fatalf("syntactically valid unrelated Run blocked Orca readiness: %#v", got)
	}
}

func TestClientCurrentRunRejectsMissingProjection(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration run-current --json"] = CommandOutput{Stdout: []byte(`{
		"ok":true,
		"result":{},
		"_meta":{"runtimeId":"runtime-1"}
	}`)}

	current, err := NewClient(runner).CurrentRun(context.Background())
	if err == nil || current != nil {
		t.Fatalf("누락된 run 필드를 명시적 null current Run으로 승인했다: current=%#v err=%v", current, err)
	}
}

func TestClientCoordinatorMutationRejectsMissingHandleBeforeInvocation(t *testing.T) {
	runner := newFakeRunner(t)
	t.Setenv("ORCA_TERMINAL_HANDLE", "")

	_, err := NewClient(runner).CreateRun(context.Background(), port.OrcaCreateRunRequest{Objective: "agent-harness issueops-v1 lifecycle=io-test generation=1 intent=op-test"})
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "coordinator_identity_unavailable" || orcaErr.Invoked || len(runner.calls) != 0 {
		t.Fatalf("missing coordinator error=%v calls=%#v", err, runner.calls)
	}
}

func TestClientRunScopedTaskMutationUsesRunAndAuthenticatedCoordinator(t *testing.T) {
	runner := newFakeRunner(t)
	createArgv := []string{"orca", "orchestration", "task-create", "--spec", "spec", "--task-title", "agent-harness marker", "--display-name", "16-demo", "--run", "run_issueops_1", "--json"}
	dispatchArgv := []string{"orca", "orchestration", "dispatch", "--task", "task-1", "--to", "term_worker", "--run", "run_issueops_1", "--inject", "--json"}
	runner.responses[strings.Join(createArgv, " ")] = fixtureOutput(t, "task_create.json")
	runner.responses[strings.Join(dispatchArgv, " ")] = fixtureOutput(t, "dispatch_create.json")
	client := NewClient(runner)

	task, err := client.CreateTask(context.Background(), port.OrcaCreateTaskRequest{RunID: "run_issueops_1", Spec: "spec", Title: "agent-harness marker", DisplayName: "16-demo"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Dispatch(context.Background(), port.OrcaDispatchRequest{RunID: "run_issueops_1", TaskID: task.ID, ToHandle: "term_worker", Inject: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(runner.calls[0], createArgv) || !slices.Equal(runner.calls[1], dispatchArgv) {
		t.Fatalf("run-scoped mutation calls=%#v", runner.calls)
	}
}

func TestClientTaskInventoryKeepsSameTaskIDDistinctAcrossRuns(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{
		"ok":true,
		"result":{"runs":[
			{"id":"run_a","objective":"agent-harness issueops-v1 lifecycle=io-a"},
			{"id":"run_b","objective":"agent-harness issueops-v1 lifecycle=io-b"}
		]},
		"_meta":{"runtimeId":"runtime-1"}
	}`)}
	for _, runID := range []string{"run_a", "run_b"} {
		command := "orca orchestration task-list --brief --run " + runID + " --json"
		runner.responses[command] = CommandOutput{Stdout: []byte(`{
			"ok":true,
			"result":{"tasks":[{"id":"task-shared","status":"ready"}],"count":1},
			"_meta":{"runtimeId":"runtime-1"}
		}`)}
	}

	got, err := NewClient(runner).ListAllTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].RunID != "run_a" || got[1].RunID != "run_b" || got[0].ID != got[1].ID {
		t.Fatalf("cross-Run task inventory = %#v", got)
	}
}

func TestClientTaskInventoryReadsOpaqueRunRowsUniformly(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{
		"ok":true,
		"result":{"runs":[
			{"id":"run_legacy_local","objective":"retired orchestration state"},
			{"id":"run_a","objective":"agent-harness issueops-v1 lifecycle=io-a"}
		]},
		"_meta":{"runtimeId":"runtime-1"}
	}`)}
	for _, runID := range []string{"run_a", "run_legacy_local"} {
		command := "orca orchestration task-list --brief --run " + runID + " --json"
		runner.responses[command] = CommandOutput{Stdout: []byte(`{
			"ok":true,
			"result":{"tasks":[],"count":0},
			"_meta":{"runtimeId":"runtime-1"}
		}`)}
	}

	got, err := NewClient(runner).ListAllTasks(context.Background())
	if err != nil || len(got) != 0 || len(runner.calls) != 3 {
		t.Fatalf("opaque read-only Run inventory: got=%#v err=%v calls=%#v", got, err, runner.calls)
	}
}

func TestClientRunInventoryReadersReuseRunsWithBoundedDeterministicQueries(t *testing.T) {
	runner := newRunInventoryRunner(10)
	client := NewClient(runner)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	inventory, err := client.ListRunInventory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.RuntimeID != "runtime-1" || len(inventory.Runs) != 10 || inventory.Runs[0].ID != "run-00" || inventory.Runs[9].ID != "run-09" {
		t.Fatalf("run inventory = %#v", inventory)
	}
	original := append([]port.OrcaRun(nil), inventory.Runs...)

	tasks, err := client.ListAllTasksFromRuns(ctx, inventory)
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := client.ListDispatchedTasksFromRuns(ctx, inventory)
	if err != nil {
		t.Fatal(err)
	}
	gates, err := client.ListGatesFromRuns(ctx, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(inventory.Runs, original) {
		t.Fatalf("FromRuns mutated inventory: before=%#v after=%#v", original, inventory.Runs)
	}
	if len(tasks) != 10 || len(dispatched) != 10 || len(gates) != 10 || tasks[0].RunID != "run-00" || dispatched[9].RunID != "run-09" || gates[0].ID != "gate-run-00" {
		t.Fatalf("task/gate projections tasks=%#v dispatched=%#v gates=%#v", tasks, dispatched, gates)
	}
	if runner.peak() != 8 {
		t.Fatalf("per-reader concurrency peak = %d, want 8", runner.peak())
	}
	for _, run := range inventory.Runs {
		for _, flags := range [][]string{{"--brief"}, {"--status", "dispatched"}} {
			argv := append([]string{"orca", "orchestration", "task-list"}, flags...)
			argv = append(argv, "--run", run.ID, "--json")
			if !runner.called(argv) {
				t.Fatalf("missing task query %#v", argv)
			}
		}
		if !runner.called([]string{"orca", "orchestration", "gate-list", "--run", run.ID, "--json"}) {
			t.Fatalf("missing gate query for %s", run.ID)
		}
	}
}

func TestClientRunInventoryReadersFailClosed(t *testing.T) {
	t.Run("zero runs retain runtime", func(t *testing.T) {
		runner := newFakeRunner(t)
		runner.responses["orca orchestration run-list --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"runs":[]},"_meta":{"runtimeId":"runtime-1"}}`)}
		inventory, err := NewClient(runner).ListRunInventory(context.Background())
		if err != nil || inventory.RuntimeID != "runtime-1" || len(inventory.Runs) != 0 {
			t.Fatalf("zero Run inventory = %#v err=%v", inventory, err)
		}
		if tasks, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), inventory); err != nil || len(tasks) != 0 {
			t.Fatalf("zero Run tasks = %#v err=%v", tasks, err)
		}
	})

	for _, tt := range []struct {
		name string
		call func(*Client, port.OrcaRunInventory) error
	}{
		{
			name: "task count mismatch",
			call: func(client *Client, inventory port.OrcaRunInventory) error {
				_, err := client.ListAllTasksFromRuns(context.Background(), inventory)
				return err
			},
		},
		{
			name: "task runtime mismatch",
			call: func(client *Client, inventory port.OrcaRunInventory) error {
				_, err := client.ListDispatchedTasksFromRuns(context.Background(), inventory)
				return err
			},
		},
		{
			name: "gate duplicate",
			call: func(client *Client, inventory port.OrcaRunInventory) error {
				_, err := client.ListGatesFromRuns(context.Background(), inventory)
				return err
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			inventory := port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{{ID: "run-a", RuntimeID: "runtime-1"}, {ID: "run-b", RuntimeID: "runtime-1"}}}
			switch tt.name {
			case "task count mismatch":
				runner.responses["orca orchestration task-list --brief --run run-a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
				runner.responses["orca orchestration task-list --brief --run run-b --json"] = runner.responses["orca orchestration task-list --brief --run run-a --json"]
			case "task runtime mismatch":
				runner.responses["orca orchestration task-list --status dispatched --run run-a --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[],"count":0},"_meta":{"runtimeId":"runtime-other"}}`)}
				runner.responses["orca orchestration task-list --status dispatched --run run-b --json"] = runner.responses["orca orchestration task-list --status dispatched --run run-a --json"]
			case "gate duplicate":
				response := CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[{"id":"gate-shared","task_id":"task-shared","status":"open"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}
				runner.responses["orca orchestration gate-list --run run-a --json"] = response
				runner.responses["orca orchestration gate-list --run run-b --json"] = response
			}
			if err := tt.call(NewClient(runner), inventory); err == nil {
				t.Fatal("invalid Run-scoped inventory was accepted")
			}
		})
	}

	runner := newFakeRunner(t)
	invalid := port.OrcaRunInventory{RuntimeID: "runtime-1", Runs: []port.OrcaRun{{ID: "run-a", RuntimeID: "runtime-1"}, {ID: "run-a", RuntimeID: "runtime-1"}}}
	if _, err := NewClient(runner).ListAllTasksFromRuns(context.Background(), invalid); err == nil || len(runner.calls) != 0 {
		t.Fatalf("duplicate Run inventory was invoked or accepted: err=%v calls=%#v", err, runner.calls)
	}
}

type runInventoryRunner struct {
	mu          sync.Mutex
	calls       [][]string
	active      int
	max         int
	release     chan struct{}
	releaseOnce sync.Once
	runCount    int
}

func newRunInventoryRunner(runCount int) *runInventoryRunner {
	return &runInventoryRunner{release: make(chan struct{}), runCount: runCount}
}

func (f *runInventoryRunner) LookPath(string) (string, error) {
	return "", errors.New("not found")
}

func (f *runInventoryRunner) Run(ctx context.Context, _ string, _ time.Duration, argv []string) (CommandOutput, error) {
	copyArgv := append([]string(nil), argv...)
	if slices.Equal(argv, []string{"orca", "orchestration", "run-list", "--json"}) {
		return CommandOutput{Stdout: []byte(f.runListJSON())}, nil
	}
	runID, gate, dispatched, ok := runInventoryQuery(argv)
	if !ok {
		return CommandOutput{}, errors.New("unexpected command")
	}
	f.mu.Lock()
	f.calls = append(f.calls, copyArgv)
	f.active++
	if f.active > f.max {
		f.max = f.active
	}
	f.mu.Unlock()
	if f.waitForEight() {
		f.releaseOnce.Do(func() { close(f.release) })
	}
	select {
	case <-f.release:
	case <-ctx.Done():
		return CommandOutput{}, ctx.Err()
	}
	f.mu.Lock()
	f.active--
	f.mu.Unlock()
	if gate {
		return CommandOutput{Stdout: []byte(`{"ok":true,"result":{"gates":[{"id":"gate-` + runID + `","task_id":"task-` + runID + `","status":"open"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}, nil
	}
	status := "ready"
	if dispatched {
		status = "dispatched"
	}
	return CommandOutput{Stdout: []byte(`{"ok":true,"result":{"tasks":[{"id":"task-` + runID + `","status":"` + status + `"}],"count":1},"_meta":{"runtimeId":"runtime-1"}}`)}, nil
}

func (f *runInventoryRunner) runListJSON() string {
	runs := make([]string, 0, f.runCount)
	for index := 0; index < f.runCount; index++ {
		runs = append(runs, fmt.Sprintf(`{"id":"run-%02d","objective":"inventory"}`, index))
	}
	return `{"ok":true,"result":{"runs":[` + strings.Join(runs, ",") + `]},"_meta":{"runtimeId":"runtime-1"}}`
}

func (f *runInventoryRunner) waitForEight() bool {
	f.mu.Lock()
	active := f.active
	f.mu.Unlock()
	return active == 8
}

func (f *runInventoryRunner) peak() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.max
}

func (f *runInventoryRunner) called(want []string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.ContainsFunc(f.calls, func(call []string) bool { return slices.Equal(call, want) })
}

func runInventoryQuery(argv []string) (runID string, gate, dispatched, ok bool) {
	if len(argv) < 6 || argv[0] != "orca" || argv[1] != "orchestration" {
		return "", false, false, false
	}
	gate = argv[2] == "gate-list"
	if !gate && argv[2] != "task-list" {
		return "", false, false, false
	}
	for index := 3; index+1 < len(argv); index++ {
		if argv[index] == "--run" {
			runID = argv[index+1]
		}
		if index+1 < len(argv) && argv[index] == "--status" && argv[index+1] == "dispatched" {
			dispatched = true
		}
	}
	return runID, gate, dispatched, runID != "" && argv[len(argv)-1] == "--json"
}
