package orca

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestExecutionV1ProvisionerCreatesOneWorktreeAndLaunchesOneOwner(t *testing.T) {
	workspace, request := executionV1Fixture(t)
	client := &executionV1Fake{workspace: workspace, probeRequest: request}
	provisioner := NewExecutionV1Client(client)

	prepared, err := provisioner.PrepareWorkspace(context.Background(), workspace, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("owner launch ran before the sealed packet existed: %v", client.calls)
	}
	launch := executionV1LaunchFixture(t, prepared.Workspace.Root)
	got, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"list", "create-worktree", "create-terminal", "create-task", "dispatch"}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("unexpected one-shot Orca sequence: got %v want %v", client.calls, wantCalls)
	}
	if client.worktreeRequest.Issue != 69 || client.worktreeRequest.Comment != request.Marker || client.worktreeRequest.BaseBranch != workspace.BaseHead {
		t.Fatalf("worktree create lost sealed identity: %#v", client.worktreeRequest)
	}
	if client.terminalRequest.Agent != "claude" || client.terminalRequest.Model != "caller-selected-model" || client.terminalRequest.ReasoningEffort != "high" {
		t.Fatalf("owner profile must be caller supplied: %#v", client.terminalRequest)
	}
	if client.taskRequest.Spec != launch.Prompt || !client.dispatchRequest.Inject || !client.dispatchRequest.ReturnPreamble {
		t.Fatalf("owner packet/dispatch contract lost: task=%#v dispatch=%#v", client.taskRequest, client.dispatchRequest)
	}
	if got.WorktreeID != "wt-69" || got.TaskID != "task-69" || got.DispatchID != "dispatch-69" || got.TerminalPTYID != "pty-69" {
		t.Fatalf("receipt did not preserve durable Orca locators: %#v", got)
	}
	if strings.Contains(strings.Join([]string{got.RuntimeID, got.RepoID, got.WorktreeID, got.TaskID, got.DispatchID, got.TerminalPTYID}, "\n"), "term-69") {
		t.Fatalf("runtime-scoped terminal handle leaked into durable receipt: %#v", got)
	}
}

func TestExecutionV1IntentStagesAreIndividuallyInspectableAndInvoked(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	client := &executionV1Fake{workspace: workspace, probeRequest: probe}
	provisioner := NewExecutionV1Client(client)

	worktreeRequest := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentWorktree, Marker: probe.Marker, Workspace: workspace, Probe: probe,
	}
	assertExecutionV1IntentZero(t, provisioner, worktreeRequest)
	worktreeReceipt, err := provisioner.InvokeIntent(context.Background(), worktreeRequest)
	if err != nil || worktreeReceipt.Workspace == nil {
		t.Fatalf("invoke worktree intent: receipt=%#v err=%v", worktreeReceipt, err)
	}
	client.worktrees = []port.OrcaWorktree{executionV1Worktree(workspace, probe)}
	assertExecutionV1IntentOne(t, provisioner, worktreeRequest)

	launch := executionV1LaunchFixture(t, workspace.Root)
	terminalRequest := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: worktreeReceipt.Workspace, Launch: &launch,
	}
	assertExecutionV1IntentZero(t, provisioner, terminalRequest)
	terminalReceipt, err := provisioner.InvokeIntent(context.Background(), terminalRequest)
	if err != nil || terminalReceipt.TerminalPTYID != "pty-69" {
		t.Fatalf("invoke terminal intent: receipt=%#v err=%v", terminalReceipt, err)
	}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-69", Handle: "term-69", PTYID: terminalReceipt.TerminalPTYID,
		WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
	}}
	assertExecutionV1IntentOne(t, provisioner, terminalRequest)

	taskRequest := terminalRequest
	taskRequest.Stage = port.ExecutionOrcaIntentTask
	taskRequest.TerminalPTYID = terminalReceipt.TerminalPTYID
	taskRequest.TerminalHandle = terminalReceipt.TerminalHandle
	assertExecutionV1IntentZero(t, provisioner, taskRequest)
	taskReceipt, err := provisioner.InvokeIntent(context.Background(), taskRequest)
	if err != nil || taskReceipt.TaskID != "task-69" {
		t.Fatalf("invoke task intent: receipt=%#v err=%v", taskReceipt, err)
	}
	client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: taskReceipt.TaskID, Title: executionV1TaskTitle(probe.Marker, launch.PromptSHA256), DisplayName: workspace.Branch, Status: "ready"}}
	assertExecutionV1IntentOne(t, provisioner, taskRequest)

	dispatchRequest := taskRequest
	dispatchRequest.Stage = port.ExecutionOrcaIntentDispatch
	dispatchRequest.TaskID = taskReceipt.TaskID
	assertExecutionV1IntentZero(t, provisioner, dispatchRequest)
	dispatchReceipt, err := provisioner.InvokeIntent(context.Background(), dispatchRequest)
	if err != nil || dispatchReceipt.DispatchID != "dispatch-69" {
		t.Fatalf("invoke dispatch intent: receipt=%#v err=%v", dispatchReceipt, err)
	}
	client.dispatch = port.OrcaDispatch{RuntimeID: "runtime-69", ID: dispatchReceipt.DispatchID, TaskID: taskReceipt.TaskID, AssigneeHandle: "term-69", Injected: true, Status: "dispatched"}
	assertExecutionV1IntentOne(t, provisioner, dispatchRequest)

	wantCalls := []string{
		"list", "create-worktree", "list", "list-terminals-inventory", "create-terminal", "list-terminals-inventory",
		"list-all-tasks-inventory", "create-task", "list-all-tasks-inventory", "show-dispatch-inventory",
		"list-terminals-inventory", "dispatch", "show-dispatch-inventory",
	}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("each intent must perform only its own inventory or mutation: got %v want %v", client.calls, wantCalls)
	}
}

func TestExecutionV1IntentInventoryPreservesZeroOneManyAndRejectsMismatches(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace, Probe: probe, Prepared: &prepared, Launch: &launch,
	}
	matching := port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}
	client := &executionV1Fake{terminals: []port.OrcaTerminal{matching, matching}}
	inventory, err := NewExecutionV1Client(client).InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 2 || inventory.AuthoritativeZero {
		t.Fatalf("ambiguous terminal inventory must remain 2 candidates: inventory=%#v err=%v", inventory, err)
	}

	request.Stage = port.ExecutionOrcaIntentTask
	client.terminals = nil
	client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Title: executionV1TaskTitle(probe.Marker, launch.PromptSHA256), DisplayName: "wrong", Status: "ready"}}
	if _, err := NewExecutionV1Client(client).InspectIntent(context.Background(), request); err == nil {
		t.Fatal("a marker-matching task with the wrong sealed identity must fail closed")
	}
}

func TestExecutionV1TaskTitleFitsOrcaAndBindsSealedIntent(t *testing.T) {
	marker := "agent-harness issueops-v1 lifecycle=io-c7e2d4e02b59 operation=c8b92dda09eaf3d"
	promptSHA256 := strings.Repeat("a", 64)

	title := executionV1TaskTitle(marker, promptSHA256)
	if len(title) > 80 {
		t.Fatalf("Orca task title exceeds the persisted limit: len=%d title=%q", len(title), title)
	}
	if changedMarker := executionV1TaskTitle(strings.Replace(marker, "c8b92dda09eaf3d", "different-intent", 1), promptSHA256); changedMarker == title {
		t.Fatalf("task title did not bind the full operation marker: %q", title)
	}
	if changedPrompt := executionV1TaskTitle(marker, strings.Repeat("b", 64)); changedPrompt == title {
		t.Fatalf("task title did not bind the prompt digest: %q", title)
	}
}

func TestExecutionV1IntentInventoryRejectsLegacyOrcaTaskTitle(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	probe.Marker = "agent-harness issueops-v1 lifecycle=io-c7e2d4e02b59 operation=c8b92dda09eaf3d"
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTask, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale",
	}
	legacyTitle := probe.Marker + " prompt=" + strings.ToLower(launch.PromptSHA256[:16])
	legacyTitle = legacyTitle[:77] + "..."
	client := &executionV1Fake{tasks: []port.OrcaTask{{
		RuntimeID: "runtime-69", ID: "task-legacy", Title: legacyTitle, DisplayName: workspace.Branch, Status: "ready",
	}}}

	inventory, err := NewExecutionV1Client(client).InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 0 || !inventory.AuthoritativeZero {
		t.Fatalf("legacy Orca title must be ignored: inventory=%#v err=%v", inventory, err)
	}
}

func assertExecutionV1IntentZero(t *testing.T, provisioner *ExecutionV1Provisioner, request port.ExecutionOrcaIntentRequest) {
	t.Helper()
	inventory, err := provisioner.InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 0 || !inventory.AuthoritativeZero {
		t.Fatalf("expected authoritative zero inventory: inventory=%#v err=%v", inventory, err)
	}
}

func assertExecutionV1IntentOne(t *testing.T, provisioner *ExecutionV1Provisioner, request port.ExecutionOrcaIntentRequest) {
	t.Helper()
	inventory, err := provisioner.InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 1 || inventory.AuthoritativeZero {
		t.Fatalf("expected exact one inventory candidate: inventory=%#v err=%v", inventory, err)
	}
}

func TestExecutionV1ProvisionerAdoptsExactlyOneMatchingReceiptWithoutCreate(t *testing.T) {
	workspace, request := executionV1Fixture(t)
	client := &executionV1Fake{workspace: workspace, probeRequest: request, worktrees: []port.OrcaWorktree{executionV1Worktree(workspace, request)}}
	if _, err := NewExecutionV1Client(client).PrepareWorkspace(context.Background(), workspace, request); err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"list"}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("matching prior receipt must not create a second worktree: got %v", client.calls)
	}
}

func TestExecutionV1ProvisionerRejectsAmbiguousOrMismatchedWorktree(t *testing.T) {
	workspace, request := executionV1Fixture(t)
	matching := executionV1Worktree(workspace, request)
	for name, rows := range map[string][]port.OrcaWorktree{
		"ambiguous":  {matching, matching},
		"wrong head": {func() port.OrcaWorktree { row := matching; row.Head = strings.Repeat("b", 40); return row }()},
	} {
		t.Run(name, func(t *testing.T) {
			client := &executionV1Fake{workspace: workspace, probeRequest: request, worktrees: rows}
			if _, err := NewExecutionV1Client(client).PrepareWorkspace(context.Background(), workspace, request); err == nil {
				t.Fatalf("%s worktree inventory must fail closed", name)
			}
			if len(client.calls) != 1 || client.calls[0] != "list" {
				t.Fatalf("failure must occur before owner launch: %v", client.calls)
			}
		})
	}
}

func TestExecutionV1ProvisionerRejectsUnsealedOwnerLaunchBeforeTerminalMutation(t *testing.T) {
	workspace, request := executionV1Fixture(t)
	client := &executionV1Fake{workspace: workspace, probeRequest: request}
	provisioner := NewExecutionV1Client(client)
	prepared, err := provisioner.PrepareWorkspace(context.Background(), workspace, request)
	if err != nil {
		t.Fatal(err)
	}
	launch := executionV1LaunchFixture(t, workspace.Root)
	launch.ContextPacketSHA256 = strings.Repeat("0", 64)
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch); err == nil {
		t.Fatal("packet digest mismatch must fail closed")
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("invalid launch mutated Orca owner resources: %v", client.calls)
	}

	launch = executionV1LaunchFixture(t, workspace.Root)
	launch.Prompt += "\n{UNRESOLVED_PLACEHOLDER}"
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch); err == nil {
		t.Fatal("unresolved owner prompt placeholder must fail closed")
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("unresolved prompt mutated Orca owner resources: %v", client.calls)
	}
}

func TestExecutionV1OwnerInventoryReportsExactTerminalAndTaskLiveness(t *testing.T) {
	client := &executionV1Fake{
		terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
		tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "running"}},
		dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "running"},
	}
	got, err := NewExecutionV1Client(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.TerminalLive || !got.TaskLive || got.TerminalID != "pty-69" || got.TaskStatus != "running" || got.DispatchStatus != "running" {
		t.Fatalf("live owner inventory was not preserved exactly: %#v", got)
	}

	client.terminals = nil
	client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}}
	client.dispatch.Status = "completed"
	got, err = NewExecutionV1Client(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil || got.TerminalLive || got.TaskLive {
		t.Fatalf("terminal task inventory should be quiescent: got=%#v err=%v", got, err)
	}
}

func TestExecutionV1OwnerInventoryUsesCompleteTaskAndDispatchInventory(t *testing.T) {
	client := &executionV1Fake{
		readyTasks: []port.OrcaTask{},
		tasks:      []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "dispatched"}},
		dispatch:   port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "running"},
	}
	got, err := NewExecutionV1Client(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.TaskLive || got.TaskStatus != "dispatched" || got.DispatchStatus != "running" {
		t.Fatalf("dispatched owner missing from ready inventory must remain live: %#v", got)
	}
	if !reflect.DeepEqual(client.calls, []string{"list-terminals-inventory", "list-all-tasks-inventory", "show-dispatch-inventory"}) {
		t.Fatalf("owner inventory did not use complete task and dispatch views: %v", client.calls)
	}
}

func TestExecutionV1OwnerInventoryRejectsMissingOrChangedRuntime(t *testing.T) {
	request := port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	}
	for _, tc := range []struct {
		name      string
		terminals []port.OrcaTerminal
		tasks     []port.OrcaTask
		dispatch  port.OrcaDispatch
	}{
		{name: "terminal missing runtime", terminals: []port.OrcaTerminal{{PTYID: "pty-69", WorktreeID: "wt-69"}}},
		{name: "terminal wrong runtime", terminals: []port.OrcaTerminal{{RuntimeID: "runtime-other", PTYID: "pty-69", WorktreeID: "wt-69"}}},
		{name: "task missing runtime", tasks: []port.OrcaTask{{ID: "task-69", Status: "completed"}}},
		{name: "task wrong runtime", tasks: []port.OrcaTask{{RuntimeID: "runtime-other", ID: "task-69", Status: "completed"}}},
		{name: "dispatch missing runtime", tasks: []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}}, dispatch: port.OrcaDispatch{ID: "dispatch-69", TaskID: "task-69", Status: "completed"}},
		{name: "dispatch wrong runtime", tasks: []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}}, dispatch: port.OrcaDispatch{RuntimeID: "runtime-other", ID: "dispatch-69", TaskID: "task-69", Status: "completed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionV1Fake{terminals: tc.terminals, tasks: tc.tasks, dispatch: tc.dispatch}
			if _, err := NewExecutionV1Client(client).InspectOwner(context.Background(), request); err == nil {
				t.Fatal("owner inventory with an absent or changed runtime identity must fail closed")
			}
		})
	}
}

func TestExecutionV1IntentReResolvesRotatedTerminalHandle(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentDispatch, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale", TaskID: "task-69",
	}
	client := &executionV1Fake{terminals: []port.OrcaTerminal{{
		RuntimeID: "runtime-69", Handle: "term-current", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
	}}}
	provisioner := NewExecutionV1Client(client)
	receipt, err := provisioner.InvokeIntent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.DispatchID != "dispatch-69" || client.dispatchRequest.ToHandle != "term-current" {
		t.Fatalf("dispatch did not use the handle re-resolved from worktree/PTI: receipt=%#v request=%#v", receipt, client.dispatchRequest)
	}

	client.calls = nil
	client.dispatch = port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-historical", Status: "running"}
	inventory, err := provisioner.InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 1 {
		t.Fatalf("dispatch inventory rejected the installed dispatch-show shape after handle rotation: inventory=%#v err=%v", inventory, err)
	}
	if !reflect.DeepEqual(client.calls, []string{"show-dispatch-inventory"}) {
		t.Fatalf("existing dispatch reconciliation must not resolve a current handle or invoke another dispatch: %v", client.calls)
	}
}

func TestExecutionV1IntentEmptyInventoryRequiresSealedRuntimeEnvelope(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	empty := ""
	wrong := "runtime-other"
	same := "runtime-69"
	for _, tc := range []struct {
		name            string
		stage           port.ExecutionOrcaIntentStage
		terminalRuntime *string
		taskRuntime     *string
		dispatchRuntime *string
		wantError       bool
	}{
		{name: "terminal same runtime", stage: port.ExecutionOrcaIntentTerminal, terminalRuntime: &same},
		{name: "terminal missing runtime", stage: port.ExecutionOrcaIntentTerminal, terminalRuntime: &empty, wantError: true},
		{name: "terminal changed runtime", stage: port.ExecutionOrcaIntentTerminal, terminalRuntime: &wrong, wantError: true},
		{name: "task same runtime", stage: port.ExecutionOrcaIntentTask, taskRuntime: &same},
		{name: "task missing runtime", stage: port.ExecutionOrcaIntentTask, taskRuntime: &empty, wantError: true},
		{name: "task changed runtime", stage: port.ExecutionOrcaIntentTask, taskRuntime: &wrong, wantError: true},
		{name: "dispatch same runtime", stage: port.ExecutionOrcaIntentDispatch, dispatchRuntime: &same},
		{name: "dispatch missing runtime", stage: port.ExecutionOrcaIntentDispatch, dispatchRuntime: &empty, wantError: true},
		{name: "dispatch changed runtime", stage: port.ExecutionOrcaIntentDispatch, dispatchRuntime: &wrong, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := port.ExecutionOrcaIntentRequest{
				Stage: tc.stage, Marker: probe.Marker, Workspace: workspace, Probe: probe,
				Prepared: &prepared, Launch: &launch,
			}
			if tc.stage == port.ExecutionOrcaIntentTask || tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TerminalPTYID = "pty-69"
			}
			if tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TaskID = "task-69"
			}
			client := &executionV1Fake{
				terminalInventoryRuntime: tc.terminalRuntime,
				taskInventoryRuntime:     tc.taskRuntime,
				dispatchInventoryRuntime: tc.dispatchRuntime,
			}
			inventory, err := NewExecutionV1Client(client).InspectIntent(context.Background(), request)
			if tc.wantError {
				if err == nil {
					t.Fatalf("empty inventory without the sealed runtime envelope was authoritative: %#v", inventory)
				}
				return
			}
			if err != nil || !inventory.AuthoritativeZero || len(inventory.Candidates) != 0 {
				t.Fatalf("same-runtime empty inventory must be authoritative zero: inventory=%#v err=%v", inventory, err)
			}
		})
	}
}

func TestExecutionV1OwnerEmptyInventoryRequiresSealedRuntimeEnvelope(t *testing.T) {
	request := port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	}
	empty := ""
	wrong := "runtime-other"
	same := "runtime-69"
	for _, tc := range []struct {
		name            string
		terminalRuntime *string
		taskRuntime     *string
		dispatchRuntime *string
		wantError       bool
	}{
		{name: "all same runtime", terminalRuntime: &same, taskRuntime: &same, dispatchRuntime: &same},
		{name: "terminal missing runtime", terminalRuntime: &empty, taskRuntime: &same, dispatchRuntime: &same, wantError: true},
		{name: "terminal changed runtime", terminalRuntime: &wrong, taskRuntime: &same, dispatchRuntime: &same, wantError: true},
		{name: "task missing runtime", terminalRuntime: &same, taskRuntime: &empty, dispatchRuntime: &same, wantError: true},
		{name: "task changed runtime", terminalRuntime: &same, taskRuntime: &wrong, dispatchRuntime: &same, wantError: true},
		{name: "dispatch missing runtime", terminalRuntime: &same, taskRuntime: &same, dispatchRuntime: &empty, wantError: true},
		{name: "dispatch changed runtime", terminalRuntime: &same, taskRuntime: &same, dispatchRuntime: &wrong, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionV1Fake{
				terminalInventoryRuntime: tc.terminalRuntime,
				taskInventoryRuntime:     tc.taskRuntime,
				dispatchInventoryRuntime: tc.dispatchRuntime,
			}
			inventory, err := NewExecutionV1Client(client).InspectOwner(context.Background(), request)
			if tc.wantError {
				if err == nil {
					t.Fatalf("owner absence without the sealed runtime envelope was accepted: %#v", inventory)
				}
				return
			}
			if err != nil || inventory.TerminalLive || inventory.TaskLive || inventory.TerminalID != "" || inventory.TaskStatus != "" || inventory.DispatchStatus != "" {
				t.Fatalf("same-runtime absent owner inventory must be quiescent: inventory=%#v err=%v", inventory, err)
			}
		})
	}
}

func TestExecutionV1IntentRejectsUnsealedRuntimeReceipts(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	title := executionV1TaskTitle(probe.Marker, launch.PromptSHA256)
	for _, tc := range []struct {
		name     string
		stage    port.ExecutionOrcaIntentStage
		runtime  string
		terminal []port.OrcaTerminal
		tasks    []port.OrcaTask
		dispatch port.OrcaDispatch
	}{
		{name: "terminal missing runtime", stage: port.ExecutionOrcaIntentTerminal, terminal: []port.OrcaTerminal{{Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}}},
		{name: "terminal wrong runtime", stage: port.ExecutionOrcaIntentTerminal, terminal: []port.OrcaTerminal{{RuntimeID: "runtime-other", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}}},
		{name: "task missing runtime", stage: port.ExecutionOrcaIntentTask, tasks: []port.OrcaTask{{ID: "task-69", Title: title, DisplayName: workspace.Branch}}},
		{name: "task wrong runtime", stage: port.ExecutionOrcaIntentTask, tasks: []port.OrcaTask{{RuntimeID: "runtime-other", ID: "task-69", Title: title, DisplayName: workspace.Branch}}},
		{name: "dispatch missing runtime", stage: port.ExecutionOrcaIntentDispatch, terminal: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}}, dispatch: port.OrcaDispatch{ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Injected: true}},
		{name: "dispatch wrong runtime", stage: port.ExecutionOrcaIntentDispatch, terminal: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}}, dispatch: port.OrcaDispatch{RuntimeID: "runtime-other", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Injected: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := port.ExecutionOrcaIntentRequest{Stage: tc.stage, Marker: probe.Marker, Workspace: workspace, Probe: probe, Prepared: &prepared, Launch: &launch}
			if tc.stage == port.ExecutionOrcaIntentTask || tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TerminalPTYID = "pty-69"
				request.TerminalHandle = "term-stale"
			}
			if tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TaskID = "task-69"
			}
			client := &executionV1Fake{terminals: tc.terminal, tasks: tc.tasks, dispatch: tc.dispatch}
			if _, err := NewExecutionV1Client(client).InspectIntent(context.Background(), request); err == nil {
				t.Fatal("runtime-mismatched receipt must fail closed")
			}
		})
	}
}

func TestExecutionV1TaskCreateValidatesTheSealedReceipt(t *testing.T) {
	workspace, probe := executionV1Fixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionV1LaunchFixture(t, workspace.Root)
	wantTitle := executionV1TaskTitle(probe.Marker, launch.PromptSHA256)
	for _, tc := range []struct {
		name string
		task port.OrcaTask
	}{
		{name: "missing runtime", task: port.OrcaTask{ID: "task-69", Title: wantTitle, DisplayName: workspace.Branch}},
		{name: "wrong runtime", task: port.OrcaTask{RuntimeID: "runtime-other", ID: "task-69", Title: wantTitle, DisplayName: workspace.Branch}},
		{name: "wrong title", task: port.OrcaTask{RuntimeID: "runtime-69", ID: "task-69", Title: "wrong", DisplayName: workspace.Branch}},
		{name: "wrong display name", task: port.OrcaTask{RuntimeID: "runtime-69", ID: "task-69", Title: wantTitle, DisplayName: "wrong"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionV1Fake{createdTask: &tc.task}
			request := port.ExecutionOrcaIntentRequest{
				Stage: port.ExecutionOrcaIntentTask, Marker: probe.Marker, Workspace: workspace, Probe: probe,
				Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale",
			}
			if _, err := NewExecutionV1Client(client).InvokeIntent(context.Background(), request); err == nil {
				t.Fatal("task create receipt that differs from the sealed intent must fail")
			}
			if !reflect.DeepEqual(client.calls, []string{"create-task"}) {
				t.Fatalf("invalid task receipt must make zero dispatch calls: %v", client.calls)
			}
		})
	}
}

func executionV1Fixture(t *testing.T) (port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "69-redesign")
	workspace := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: filepath.Dir(root), Root: root,
		Branch: "69-redesign", BaseBranch: "main", BaseHead: strings.Repeat("a", 40), Confirm: true,
	}
	request := port.ExecutionOrcaProbeRequest{
		Repo: workspace.SourceRoot, Host: "claude", Model: "caller-selected-model", Effort: "high",
		Provider: "github", Issue: 69, Marker: "agent-harness issueops-v1 lifecycle=io-69",
	}
	return workspace, request
}

func executionV1LaunchFixture(t *testing.T, root string) port.ExecutionOrcaLaunchRequest {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	packet := []byte("{\"schema_version\":1}\n")
	packetPath := filepath.Join(root, ".agent-harness", "state", "issueops-v1", "context.json")
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, packet, 0o600); err != nil {
		t.Fatal(err)
	}
	packetDigest := fmt.Sprintf("%x", sha256.Sum256(packet))
	prompt := "sealed IssueOps v1 prompt\npacket=" + packetPath + "\ndigest=" + packetDigest
	promptPath := filepath.Join(filepath.Dir(packetPath), "owner-prompt.txt")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		t.Fatal(err)
	}
	return port.ExecutionOrcaLaunchRequest{
		Prompt: prompt, PromptPath: promptPath, PromptSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(prompt))),
		ContextPacketPath: packetPath, ContextPacketSHA256: packetDigest,
	}
}

func executionV1Worktree(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) port.OrcaWorktree {
	return port.OrcaWorktree{
		RuntimeID: "runtime-69", ID: "wt-69", InstanceID: "instance-69", RepoID: "repo-69",
		Path: workspace.Root, Head: workspace.BaseHead, Branch: workspace.Branch, Comment: request.Marker, Issue: 69,
	}
}

type executionV1Fake struct {
	calls                    []string
	workspace                port.ExecutionWorkspaceRequest
	probeRequest             port.ExecutionOrcaProbeRequest
	worktrees                []port.OrcaWorktree
	worktreeRequest          port.OrcaCreateWorktreeRequest
	terminalRequest          port.OrcaCreateTerminalRequest
	taskRequest              port.OrcaCreateTaskRequest
	dispatchRequest          port.OrcaDispatchRequest
	terminals                []port.OrcaTerminal
	readyTasks               []port.OrcaTask
	tasks                    []port.OrcaTask
	dispatch                 port.OrcaDispatch
	createdTask              *port.OrcaTask
	terminalInventoryRuntime *string
	taskInventoryRuntime     *string
	dispatchInventoryRuntime *string
}

func (f *executionV1Fake) Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	f.calls = append(f.calls, "probe")
	return port.OrcaProbeResult{Available: true, Ready: true}, nil
}

func (f *executionV1Fake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.calls = append(f.calls, "list")
	return append([]port.OrcaWorktree(nil), f.worktrees...), nil
}

func (f *executionV1Fake) CreateWorktree(_ context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	f.calls = append(f.calls, "create-worktree")
	f.worktreeRequest = req
	return executionV1Worktree(f.workspace, f.probeRequest), nil
}

func (f *executionV1Fake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.calls = append(f.calls, "create-terminal")
	f.terminalRequest = req
	return port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: req.Title, Connected: true, Writable: true}, nil
}

func (f *executionV1Fake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.calls = append(f.calls, "create-task")
	f.taskRequest = req
	if f.createdTask != nil {
		return *f.createdTask, nil
	}
	return port.OrcaTask{RuntimeID: "runtime-69", ID: "task-69", Title: req.Title, DisplayName: req.DisplayName, Status: "ready"}, nil
}

func (f *executionV1Fake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.calls = append(f.calls, "dispatch")
	f.dispatchRequest = req
	return port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: req.TaskID, AssigneeHandle: req.ToHandle, Injected: true}, nil
}

func (f *executionV1Fake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.calls = append(f.calls, "list-terminals")
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *executionV1Fake) listTerminalsInventory(context.Context, string) (executionV1TerminalInventory, error) {
	f.calls = append(f.calls, "list-terminals-inventory")
	return executionV1TerminalInventory{RuntimeID: executionV1FakeRuntime(f.terminalInventoryRuntime), Rows: append([]port.OrcaTerminal(nil), f.terminals...)}, nil
}

func (f *executionV1Fake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	f.calls = append(f.calls, "list-ready-tasks")
	return append([]port.OrcaTask(nil), f.readyTasks...), nil
}

func (f *executionV1Fake) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	f.calls = append(f.calls, "list-all-tasks")
	return append([]port.OrcaTask(nil), f.tasks...), nil
}

func (f *executionV1Fake) listAllTasksInventory(context.Context) (executionV1TaskInventory, error) {
	f.calls = append(f.calls, "list-all-tasks-inventory")
	return executionV1TaskInventory{RuntimeID: executionV1FakeRuntime(f.taskInventoryRuntime), Rows: append([]port.OrcaTask(nil), f.tasks...)}, nil
}

func (f *executionV1Fake) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	f.calls = append(f.calls, "show-dispatch")
	if f.dispatch.ID == "" {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "not_found"}
	}
	return f.dispatch, nil
}

func (f *executionV1Fake) showDispatchInventory(context.Context, string) (executionV1DispatchInventory, error) {
	f.calls = append(f.calls, "show-dispatch-inventory")
	result := executionV1DispatchInventory{RuntimeID: executionV1FakeRuntime(f.dispatchInventoryRuntime)}
	if f.dispatch.ID != "" {
		dispatch := f.dispatch
		result.Dispatch = &dispatch
	}
	return result, nil
}

func executionV1FakeRuntime(configured *string) string {
	if configured != nil {
		return *configured
	}
	return "runtime-69"
}
