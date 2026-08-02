package orca

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

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
