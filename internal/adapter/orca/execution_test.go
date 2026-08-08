package orca

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestExecutionProvisionerCreatesOneWorktreeAndLaunchesOneOwner(t *testing.T) {
	workspace, request := executionFixture(t)
	client := &executionFake{workspace: workspace, probeRequest: request}
	provisioner := NewExecutionClient(client)

	prepared, err := provisioner.PrepareWorkspace(context.Background(), workspace, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("owner launch ran before the sealed packet existed: %v", client.calls)
	}
	launch := executionLaunchFixture(t, prepared.Workspace.Root)
	got, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch)
	if err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"list", "create-worktree", "create-terminal", "create-run", "use-run", "create-task", "dispatch"}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("unexpected one-shot Orca sequence: got %v want %v", client.calls, wantCalls)
	}
	if client.worktreeRequest.Issue != 69 || client.worktreeRequest.Comment != request.Marker ||
		client.worktreeRequest.BaseBranch != workspace.BaseHead ||
		client.worktreeRequest.ParentWorktree != workspace.ParentWorktree ||
		client.worktreeRequest.UpstreamBranch != "" {
		t.Fatalf("worktree create lost sealed identity: %#v", client.worktreeRequest)
	}
	if client.terminalRequest.Agent != "claude" || client.terminalRequest.Model != "caller-selected-model" || client.terminalRequest.ReasoningEffort != "high" {
		t.Fatalf("owner profile must be caller supplied: %#v", client.terminalRequest)
	}
	if client.runRequest.Objective != request.Marker || client.taskRequest.RunID != "run-69" ||
		client.dispatchRequest.RunID != "run-69" || client.taskRequest.Spec != launch.Prompt ||
		!client.dispatchRequest.Inject || !client.dispatchRequest.ReturnPreamble {
		t.Fatalf("owner packet/dispatch contract lost: task=%#v dispatch=%#v", client.taskRequest, client.dispatchRequest)
	}
	if got.WorktreeID != "wt-69" || got.RunID != "run-69" || got.TaskID != "task-69" || got.DispatchID != "dispatch-69" || got.TerminalPTYID != "pty-69" {
		t.Fatalf("receipt did not preserve durable Orca locators: %#v", got)
	}
	if strings.Contains(strings.Join([]string{got.RuntimeID, got.RepoID, got.WorktreeID, got.TaskID, got.DispatchID, got.TerminalPTYID}, "\n"), "term-69") {
		t.Fatalf("runtime-scoped terminal handle leaked into durable receipt: %#v", got)
	}
}

func TestExecutionProvisionerAcceptsGitLabMarkerWithoutNativeMetadata(t *testing.T) {
	workspace, request := executionFixture(t)
	request = executionGitLabProbe(request)
	client := &executionFake{workspace: workspace, probeRequest: request}

	if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err != nil {
		t.Fatalf("공개 Orca CLI가 native GitLab 필드를 쓰지 못해도 봉인 marker로 준비해야 한다: %v", err)
	}
	if client.worktreeRequest.Provider != "gitlab" || client.worktreeRequest.Issue != 69 ||
		client.worktreeRequest.Comment != request.Marker {
		t.Fatalf("GitLab IID 봉인이 worktree 요청에서 손실됐다: %#v", client.worktreeRequest)
	}
}

func TestExecutionProvisionerRejectsMismatchedNativeGitLabMetadata(t *testing.T) {
	workspace, request := executionFixture(t)
	request = executionGitLabProbe(request)
	row := executionWorktree(workspace, request)
	wrong := 70
	row.GitLabIssue = &wrong
	client := &executionFake{workspace: workspace, probeRequest: request, worktrees: []port.OrcaWorktree{row}}

	if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err == nil {
		t.Fatal("Orca native GitLab IID가 봉인된 IID와 다르면 거부해야 한다")
	}
	if !reflect.DeepEqual(client.calls, []string{"list"}) {
		t.Fatalf("불일치 receipt 뒤에 worktree mutation을 실행했다: %v", client.calls)
	}
}

func TestExecutionProvisionerRequiresExactGitLabMarker(t *testing.T) {
	workspace, request := executionFixture(t)
	request = executionGitLabProbe(request)
	for _, marker := range []string{
		strings.TrimSuffix(request.Marker, " provider=gitlab issue=69"),
		strings.Replace(request.Marker, "provider=gitlab", "provider=github", 1),
		strings.Replace(request.Marker, "issue=69", "issue=70", 1),
	} {
		t.Run(marker, func(t *testing.T) {
			request.Marker = marker
			client := &executionFake{workspace: workspace, probeRequest: request}
			if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err == nil {
				t.Fatal("GitLab provider와 IID가 정확히 봉인되지 않은 marker를 허용했다")
			}
			if len(client.calls) != 0 {
				t.Fatalf("잘못된 marker가 Orca inventory에 도달했다: %v", client.calls)
			}
		})
	}
}

func TestExecutionInvokeIntentPreflightFailureProvesNotInvoked(t *testing.T) {
	workspace, github := executionFixture(t)
	gitlab := executionGitLabProbe(github)
	tests := []struct {
		name  string
		probe port.ExecutionOrcaProbeRequest
	}{
		{
			name:  "github marker names gitlab",
			probe: withExecutionMarker(github, strings.Replace(github.Marker, "provider=github", "provider=gitlab", 1)),
		},
		{
			name:  "github marker has wrong issue",
			probe: withExecutionMarker(github, strings.Replace(github.Marker, "issue=69", "issue=70", 1)),
		},
		{
			name:  "gitlab marker names github",
			probe: withExecutionMarker(gitlab, strings.Replace(gitlab.Marker, "provider=gitlab", "provider=github", 1)),
		},
		{
			name:  "gitlab marker has wrong issue",
			probe: withExecutionMarker(gitlab, strings.Replace(gitlab.Marker, "issue=69", "issue=70", 1)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &executionFake{workspace: workspace, probeRequest: test.probe}
			_, err := NewExecutionClient(client).InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
				Stage: port.ExecutionOrcaIntentWorktree, Marker: test.probe.Marker,
				Workspace: workspace, Probe: test.probe,
			})
			var typed *port.OrcaError
			if !errors.As(err, &typed) || typed.Code != "intent_preflight_rejected" || typed.Invoked {
				t.Fatalf("preflight error = %#v (%v)", typed, err)
			}
			if len(client.calls) != 0 {
				t.Fatalf("preflight failure reached Orca client: %v", client.calls)
			}
		})
	}
}

func TestExecutionInvokeIntentLocalFailuresRemainNotInvoked(t *testing.T) {
	workspace, probe := executionFixture(t)
	worktreeRequest := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentWorktree, Marker: probe.Marker,
		Workspace: workspace, Probe: probe,
	}
	t.Run("missing client", func(t *testing.T) {
		_, err := (*ExecutionProvisioner)(nil).InvokeIntent(context.Background(), worktreeRequest)
		assertExecutionPreflightError(t, err)
	})
	t.Run("unsupported stage", func(t *testing.T) {
		client := &executionFake{workspace: workspace, probeRequest: probe}
		request := worktreeRequest
		request.Stage = port.ExecutionOrcaIntentStage("unknown")
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		assertExecutionPreflightError(t, err)
		if len(client.calls) != 0 {
			t.Fatalf("unsupported stage reached Orca client: %v", client.calls)
		}
	})
	t.Run("dispatch terminal cannot be resolved", func(t *testing.T) {
		client := &executionFake{}
		launch := executionLaunchFixture(t, workspace.Root)
		prepared := &port.ExecutionOrcaWorkspaceReceipt{
			Workspace: port.ExecutionWorkspaceReceipt{
				SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
				BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree,
				Driver: "orca", Exists: true,
			},
			RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69",
		}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
			Stage: port.ExecutionOrcaIntentDispatch, Marker: probe.Marker,
			Workspace: workspace, Probe: probe, Prepared: prepared, Launch: &launch,
			TerminalPTYID: "pty-69", RunID: "run-69", RunBound: true, TaskID: "task-69",
		})
		assertExecutionPreflightError(t, err)
		if !reflect.DeepEqual(client.calls, []string{"list-terminals-inventory"}) {
			t.Fatalf("dispatch preflight performed a mutation: %v", client.calls)
		}
	})
}

func assertExecutionPreflightError(t *testing.T, err error) {
	t.Helper()
	var typed *port.OrcaError
	if !errors.As(err, &typed) || typed.Code != "intent_preflight_rejected" || typed.Invoked {
		t.Fatalf("preflight error = %#v (%v)", typed, err)
	}
}

func TestExecutionIntentStagesAreIndividuallyInspectableAndInvoked(t *testing.T) {
	workspace, probe := executionFixture(t)
	client := &executionFake{workspace: workspace, probeRequest: probe}
	provisioner := NewExecutionClient(client)

	worktreeRequest := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentWorktree, Marker: probe.Marker, Workspace: workspace, Probe: probe,
	}
	assertExecutionIntentZero(t, provisioner, worktreeRequest)
	worktreeReceipt, err := provisioner.InvokeIntent(context.Background(), worktreeRequest)
	if err != nil || worktreeReceipt.Workspace == nil {
		t.Fatalf("invoke worktree intent: receipt=%#v err=%v", worktreeReceipt, err)
	}
	client.worktrees = []port.OrcaWorktree{executionWorktree(workspace, probe)}
	assertExecutionIntentOne(t, provisioner, worktreeRequest)

	launch := executionLaunchFixture(t, workspace.Root)
	terminalRequest := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: worktreeReceipt.Workspace, Launch: &launch,
	}
	assertExecutionIntentZero(t, provisioner, terminalRequest)
	terminalReceipt, err := provisioner.InvokeIntent(context.Background(), terminalRequest)
	if err != nil || terminalReceipt.TerminalPTYID != "pty-69" {
		t.Fatalf("invoke terminal intent: receipt=%#v err=%v", terminalReceipt, err)
	}
	client.terminals = []port.OrcaTerminal{{
		RuntimeID: "runtime-69", Handle: "term-69", PTYID: terminalReceipt.TerminalPTYID,
		WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
	}}
	assertExecutionIntentOne(t, provisioner, terminalRequest)

	runRequest := terminalRequest
	runRequest.Stage = port.ExecutionOrcaIntentRun
	runRequest.TerminalPTYID = terminalReceipt.TerminalPTYID
	assertExecutionIntentZero(t, provisioner, runRequest)
	runReceipt, err := provisioner.InvokeIntent(context.Background(), runRequest)
	if err != nil || runReceipt.RunID != "run-69" {
		t.Fatalf("invoke Run intent: receipt=%#v err=%v", runReceipt, err)
	}
	assertExecutionIntentOne(t, provisioner, runRequest)

	bindRequest := runRequest
	bindRequest.Stage = port.ExecutionOrcaIntentRunBind
	bindRequest.RunID = runReceipt.RunID
	assertExecutionIntentZero(t, provisioner, bindRequest)
	bindReceipt, err := provisioner.InvokeIntent(context.Background(), bindRequest)
	if err != nil || bindReceipt.RunID != "run-69" || !bindReceipt.RunBound {
		t.Fatalf("invoke Run bind intent: receipt=%#v err=%v", bindReceipt, err)
	}
	assertExecutionIntentOne(t, provisioner, bindRequest)

	taskRequest := bindRequest
	taskRequest.Stage = port.ExecutionOrcaIntentTask
	taskRequest.RunBound = true
	taskRequest.TerminalHandle = terminalReceipt.TerminalHandle
	assertExecutionIntentZero(t, provisioner, taskRequest)
	taskReceipt, err := provisioner.InvokeIntent(context.Background(), taskRequest)
	if err != nil || taskReceipt.TaskID != "task-69" {
		t.Fatalf("invoke task intent: receipt=%#v err=%v", taskReceipt, err)
	}
	client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", RunID: runReceipt.RunID, ID: taskReceipt.TaskID, Title: executionTaskTitle(probe.Marker, launch.PromptSHA256), DisplayName: workspace.Branch, Status: "ready"}}
	assertExecutionIntentOne(t, provisioner, taskRequest)

	dispatchRequest := taskRequest
	dispatchRequest.Stage = port.ExecutionOrcaIntentDispatch
	dispatchRequest.TaskID = taskReceipt.TaskID
	assertExecutionIntentZero(t, provisioner, dispatchRequest)
	dispatchReceipt, err := provisioner.InvokeIntent(context.Background(), dispatchRequest)
	if err != nil || dispatchReceipt.DispatchID != "dispatch-69" {
		t.Fatalf("invoke dispatch intent: receipt=%#v err=%v", dispatchReceipt, err)
	}
	client.dispatch = port.OrcaDispatch{RuntimeID: "runtime-69", ID: dispatchReceipt.DispatchID, TaskID: taskReceipt.TaskID, AssigneeHandle: "term-69", Injected: true, Status: "dispatched"}
	assertExecutionIntentOne(t, provisioner, dispatchRequest)

	wantCalls := []string{
		"list", "create-worktree", "list", "list-terminals-inventory", "create-terminal", "list-terminals-inventory",
		"list-runs", "create-run", "list-runs", "current-run", "current-run", "use-run", "current-run",
		"list-run-tasks-inventory", "create-task", "list-run-tasks-inventory", "show-dispatch-inventory",
		"list-terminals-inventory", "dispatch", "show-dispatch-inventory",
	}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("each intent must perform only its own inventory or mutation: got %v want %v", client.calls, wantCalls)
	}
}

func TestExecutionIntentRefreshesTruncatedTerminalCreateReceipt(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69",
	}
	client := &executionFake{
		workspace: workspace, probeRequest: probe,
		createdTerminal: &port.OrcaTerminal{
			RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69",
			WorktreeID: "wt-69", Title: probe.Marker[:12], Connected: true, Writable: true,
		},
		terminals: []port.OrcaTerminal{{
			RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69",
			WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
		}},
	}
	launch := executionLaunchFixture(t, workspace.Root)
	got, err := NewExecutionClient(client).InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace,
		Probe: probe, Prepared: &prepared, Launch: &launch,
	})
	if err != nil || got.TerminalPTYID != "pty-69" {
		t.Fatalf("terminal create receipt was not refreshed from authoritative inventory: receipt=%#v err=%v", got, err)
	}
	if !reflect.DeepEqual(client.calls, []string{"create-terminal", "list-terminals-inventory"}) {
		t.Fatalf("unexpected terminal refresh sequence: %v", client.calls)
	}
}

func TestExecutionIntentRefreshesTerminalCreateReceiptWithoutPTY(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
		},
		RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69",
	}
	client := &executionFake{
		workspace: workspace, probeRequest: probe,
		createdTerminal: &port.OrcaTerminal{
			RuntimeID: "runtime-69", Handle: "term-69",
			WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
		},
		terminals: []port.OrcaTerminal{{
			RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69",
			WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
		}},
	}
	launch := executionLaunchFixture(t, workspace.Root)
	got, err := NewExecutionClient(client).InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace,
		Probe: probe, Prepared: &prepared, Launch: &launch,
	})
	if err != nil || got.TerminalPTYID != "pty-69" {
		t.Fatalf("PTY 없는 생성 응답을 authoritative inventory로 복구하지 못했다: receipt=%#v err=%v", got, err)
	}
	if !reflect.DeepEqual(client.calls, []string{"create-terminal", "list-terminals-inventory"}) {
		t.Fatalf("unexpected terminal refresh sequence: %v", client.calls)
	}
}

func TestExecutionIntentInventoryPreservesZeroOneManyAndRejectsMismatches(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace, Probe: probe, Prepared: &prepared, Launch: &launch,
	}
	matching := port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true}
	client := &executionFake{terminals: []port.OrcaTerminal{matching, matching}}
	inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 2 || inventory.AuthoritativeZero {
		t.Fatalf("ambiguous terminal inventory must remain 2 candidates: inventory=%#v err=%v", inventory, err)
	}

	request.Stage = port.ExecutionOrcaIntentTask
	request.TerminalPTYID = "pty-69"
	request.RunID = "run-69"
	request.RunBound = true
	client.terminals = nil
	client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Title: executionTaskTitle(probe.Marker, launch.PromptSHA256), DisplayName: "wrong", Status: "ready"}}
	if _, err := NewExecutionClient(client).InspectIntent(context.Background(), request); err == nil {
		t.Fatal("a marker-matching task with the wrong sealed identity must fail closed")
	}
}

// 복구용 인벤토리 조회는 이미 삭제된 worktree의 prompt/context 파일을 다시
// 요구하면 안 된다. 봉인 메타데이터로 조회만 허용하되, 실제 mutation은 기존
// 파일 검증을 그대로 통과해야 한다.
func TestExecutionIntentInspectionSurvivesDeletedWorkspaceWithoutWeakeningInvoke(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTerminal, Marker: probe.Marker, Workspace: workspace,
		Probe: probe, Prepared: &prepared, Launch: &launch,
	}
	if err := os.RemoveAll(workspace.Root); err != nil {
		t.Fatal(err)
	}

	client := &executionFake{}
	inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
	if err != nil || !inventory.AuthoritativeZero || len(inventory.Candidates) != 0 {
		t.Fatalf("삭제된 workspace의 봉인 intent를 조회하지 못했다: inventory=%#v err=%v", inventory, err)
	}

	_, err = NewExecutionClient(client).InvokeIntent(context.Background(), request)
	assertExecutionPreflightError(t, err)
	if !reflect.DeepEqual(client.calls, []string{"list-terminals-inventory"}) {
		t.Fatalf("파일 없는 invoke가 Orca mutation에 도달했다: %v", client.calls)
	}
}

func TestExecutionTaskTitleFitsOrcaAndBindsSealedIntent(t *testing.T) {
	marker := "agent-harness issueops-v1 lifecycle=io-c7e2d4e02b59 operation=c8b92dda09eaf3d"
	promptSHA256 := strings.Repeat("a", 64)

	title := executionTaskTitle(marker, promptSHA256)
	if len(title) > 80 {
		t.Fatalf("Orca task title exceeds the persisted limit: len=%d title=%q", len(title), title)
	}
	if changedMarker := executionTaskTitle(strings.Replace(marker, "c8b92dda09eaf3d", "different-intent", 1), promptSHA256); changedMarker == title {
		t.Fatalf("task title did not bind the full operation marker: %q", title)
	}
	if changedPrompt := executionTaskTitle(marker, strings.Repeat("b", 64)); changedPrompt == title {
		t.Fatalf("task title did not bind the prompt digest: %q", title)
	}
}

func TestExecutionIntentInventoryRejectsRetiredTaskTitle(t *testing.T) {
	workspace, probe := executionFixture(t)
	probe.Marker = "agent-harness issueops-v1 lifecycle=io-c7e2d4e02b59 operation=c8b92dda09eaf3d provider=github issue=69"
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentTask, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale",
		RunID: "run-69", RunBound: true,
	}
	retiredTitle := probe.Marker + " prompt=" + strings.ToLower(launch.PromptSHA256[:16])
	retiredTitle = retiredTitle[:77] + "..."
	client := &executionFake{tasks: []port.OrcaTask{{
		RuntimeID: "runtime-69", ID: "task-retired", Title: retiredTitle, DisplayName: workspace.Branch, Status: "ready",
	}}}

	inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 0 || !inventory.AuthoritativeZero {
		t.Fatalf("retired Orca title must be ignored: inventory=%#v err=%v", inventory, err)
	}
}

func assertExecutionIntentZero(t *testing.T, provisioner *ExecutionProvisioner, request port.ExecutionOrcaIntentRequest) {
	t.Helper()
	inventory, err := provisioner.InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 0 || !inventory.AuthoritativeZero {
		t.Fatalf("expected authoritative zero inventory: inventory=%#v err=%v", inventory, err)
	}
}

func assertExecutionIntentOne(t *testing.T, provisioner *ExecutionProvisioner, request port.ExecutionOrcaIntentRequest) {
	t.Helper()
	inventory, err := provisioner.InspectIntent(context.Background(), request)
	if err != nil || len(inventory.Candidates) != 1 || inventory.AuthoritativeZero {
		t.Fatalf("expected exact one inventory candidate: inventory=%#v err=%v", inventory, err)
	}
}

func TestExecutionProvisionerAdoptsExactlyOneMatchingReceiptWithoutCreate(t *testing.T) {
	workspace, request := executionFixture(t)
	client := &executionFake{workspace: workspace, probeRequest: request, worktrees: []port.OrcaWorktree{executionWorktree(workspace, request)}}
	if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err != nil {
		t.Fatal(err)
	}
	wantCalls := []string{"list"}
	if !reflect.DeepEqual(client.calls, wantCalls) {
		t.Fatalf("matching prior receipt must not create a second worktree: got %v", client.calls)
	}
}

func TestExecutionProvisionerAcceptsExplicitManualParentLineage(t *testing.T) {
	workspace, request := executionFixture(t)
	matching := executionWorktree(workspace, request)
	matching.LineageSource = "manual-action"
	client := &executionFake{workspace: workspace, probeRequest: request, worktrees: []port.OrcaWorktree{matching}}

	if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err != nil {
		t.Fatalf("명시적 수동 parent 영수증은 같은 canonical parent를 증명해야 한다: %v", err)
	}
	if !reflect.DeepEqual(client.calls, []string{"list"}) {
		t.Fatalf("동등한 parent 영수증을 채택할 때 새 worktree를 만들면 안 된다: %v", client.calls)
	}
}

func TestExecutionProvisionerRejectsAmbiguousOrMismatchedWorktree(t *testing.T) {
	workspace, request := executionFixture(t)
	matching := executionWorktree(workspace, request)
	for name, rows := range map[string][]port.OrcaWorktree{
		"ambiguous":  {matching, matching},
		"wrong head": {func() port.OrcaWorktree { row := matching; row.Head = strings.Repeat("b", 40); return row }()},
		"missing parent": {func() port.OrcaWorktree {
			row := matching
			row.ParentWorktreeID = ""
			return row
		}()},
		"wrong parent": {func() port.OrcaWorktree {
			row := matching
			row.ParentWorktreeID = row.RepoID + "::" + filepath.Join(filepath.Dir(workspace.ParentWorktree), "67-other")
			return row
		}()},
		"inferred lineage": {func() port.OrcaWorktree {
			row := matching
			row.LineageSource = "cwd-context"
			return row
		}()},
		"manual inference": {func() port.OrcaWorktree {
			row := matching
			row.LineageSource = "manual-action"
			row.LineageConfidence = "inferred"
			return row
		}()},
	} {
		t.Run(name, func(t *testing.T) {
			client := &executionFake{workspace: workspace, probeRequest: request, worktrees: rows}
			if _, err := NewExecutionClient(client).PrepareWorkspace(context.Background(), workspace, request); err == nil {
				t.Fatalf("%s worktree inventory must fail closed", name)
			}
			if len(client.calls) != 1 || client.calls[0] != "list" {
				t.Fatalf("failure must occur before owner launch: %v", client.calls)
			}
		})
	}
}

func TestExecutionProvisionerRejectsUnsealedOwnerLaunchBeforeTerminalMutation(t *testing.T) {
	workspace, request := executionFixture(t)
	client := &executionFake{workspace: workspace, probeRequest: request}
	provisioner := NewExecutionClient(client)
	prepared, err := provisioner.PrepareWorkspace(context.Background(), workspace, request)
	if err != nil {
		t.Fatal(err)
	}
	launch := executionLaunchFixture(t, workspace.Root)
	launch.ContextPacketSHA256 = strings.Repeat("0", 64)
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch); err == nil {
		t.Fatal("packet digest mismatch must fail closed")
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("invalid launch mutated Orca owner resources: %v", client.calls)
	}

	launch = executionLaunchFixture(t, workspace.Root)
	launch.Prompt += "\n{UNRESOLVED_PLACEHOLDER}"
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, request, launch); err == nil {
		t.Fatal("unresolved owner prompt placeholder must fail closed")
	}
	if !reflect.DeepEqual(client.calls, []string{"list", "create-worktree"}) {
		t.Fatalf("unresolved prompt mutated Orca owner resources: %v", client.calls)
	}
}

func TestExecutionOwnerInventoryReportsExactTerminalAndTaskLiveness(t *testing.T) {
	client := &executionFake{
		terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
		tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "running"}},
		dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "running"},
	}
	got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", RunID: "run-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
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
	got, err = NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil || got.TerminalLive || got.TaskLive {
		t.Fatalf("terminal task inventory should be quiescent: got=%#v err=%v", got, err)
	}
}

func TestExecutionOwnerInventoryReportsDispatchAssigneePresence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		terminals []port.OrcaTerminal
		present   bool
	}{
		{name: "assignee absent"},
		{name: "assignee present", terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-assignee", PTYID: "pty-assignee", WorktreeID: "wt-69"}}, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionFake{
				terminals: tc.terminals,
				tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
				dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-assignee", Status: "dispatched"},
			}
			got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
				RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-old",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.DispatchAssigneeHandle != "term-assignee" || got.DispatchAssigneePresent != tc.present {
				t.Fatalf("dispatch assignee evidence=%#v want handle=%q present=%t", got, "term-assignee", tc.present)
			}
		})
	}
}

func TestExecutionOwnerInventoryReportsTerminalInventoryCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name     string
		complete bool
	}{
		{name: "complete", complete: true},
		{name: "incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			complete := tc.complete
			client := &executionFake{
				terminalInventoryComplete: &complete,
				tasks:                     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
				dispatch:                  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-old", Status: "dispatched"},
			}
			got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
				RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-old",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.TerminalInventoryComplete != tc.complete {
				t.Fatalf("terminal inventory completeness=%t want=%t", got.TerminalInventoryComplete, tc.complete)
			}
		})
	}
}

func TestExecutionOwnerInventoryRejectsAmbiguousOwnerTerminal(t *testing.T) {
	terminal := port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-old", PTYID: "pty-old", WorktreeID: "wt-69"}
	client := &executionFake{
		terminals: []port.OrcaTerminal{terminal, terminal},
		tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "completed"}},
		dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-old", Status: "dispatched"},
	}
	if got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-old",
	}); err == nil {
		t.Fatalf("ambiguous owner terminal inventory=%#v", got)
	}
}

func TestExecutionOwnerInventoryUsesCurrentPaneRuntimeForTerminalLiveness(t *testing.T) {
	currentRuntime := "runtime-70"
	paneLive := 1
	paneGhost := -1
	request := port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69",
		TerminalPTYID: "pty-69", AllowRuntimeRollover: true,
	}

	for _, tc := range []struct {
		name     string
		pane     *int
		wantLive bool
		wantErr  bool
	}{
		{name: "현재 pane", pane: &paneLive, wantLive: true},
		{name: "runtime 재시작 뒤 남은 ghost 행", pane: &paneGhost},
		{name: "pane runtime 증거 누락", pane: nil, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &executionFake{
				terminals: []port.OrcaTerminal{{
					RuntimeID: currentRuntime, Handle: "term-69", PTYID: "pty-69",
					WorktreeID: "wt-69", Connected: true, Writable: true,
				}},
				tasks:                    []port.OrcaTask{{RuntimeID: currentRuntime, ID: "task-69", Status: "completed"}},
				dispatch:                 port.OrcaDispatch{RuntimeID: currentRuntime, ID: "dispatch-69", TaskID: "task-69", Status: "completed"},
				terminalInventoryRuntime: &currentRuntime,
				taskInventoryRuntime:     &currentRuntime,
				dispatchInventoryRuntime: &currentRuntime,
				terminalDetail: &executionTerminalDetailInventory{
					RuntimeID: currentRuntime,
					Terminal: port.OrcaTerminal{
						RuntimeID: currentRuntime, Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69",
						Connected: true, Writable: true,
					},
					PaneRuntimeID: tc.pane,
				},
			}
			got, err := NewExecutionClient(client).InspectOwner(context.Background(), request)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("pane runtime 증거가 없는데 owner 교체를 허용했다: %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.TerminalID != "pty-69" || got.TerminalLive != tc.wantLive {
				t.Fatalf("pane runtime liveness 판정이 다르다: got=%#v want_live=%t", got, tc.wantLive)
			}
		})
	}
}

func TestExecutionOwnerInventoryRejectsContradictoryTerminalDetail(t *testing.T) {
	currentRuntime := "runtime-70"
	paneLive := 1
	client := &executionFake{
		terminals: []port.OrcaTerminal{{
			RuntimeID: currentRuntime, Handle: "term-69", PTYID: "pty-69",
			WorktreeID: "wt-69", Connected: true, Writable: true,
		}},
		tasks:                    []port.OrcaTask{{RuntimeID: currentRuntime, ID: "task-69", Status: "completed"}},
		dispatch:                 port.OrcaDispatch{RuntimeID: currentRuntime, ID: "dispatch-69", TaskID: "task-69", Status: "completed"},
		terminalInventoryRuntime: &currentRuntime,
		taskInventoryRuntime:     &currentRuntime,
		dispatchInventoryRuntime: &currentRuntime,
		terminalDetail: &executionTerminalDetailInventory{
			RuntimeID: currentRuntime,
			Terminal: port.OrcaTerminal{
				RuntimeID: currentRuntime, Handle: "term-69", PTYID: "pty-other", WorktreeID: "wt-69",
				Connected: true, Writable: true,
			},
			PaneRuntimeID: &paneLive,
		},
	}
	if got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69",
		TerminalPTYID: "pty-69", AllowRuntimeRollover: true,
	}); err == nil {
		t.Fatalf("목록과 상세 조회의 terminal identity 모순을 허용했다: %#v", got)
	}

	client.terminalDetail = nil
	client.terminalDetailErr = fmt.Errorf("terminal show failed")
	if got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69",
		TerminalPTYID: "pty-69", AllowRuntimeRollover: true,
	}); err == nil {
		t.Fatalf("terminal 상세 조회 실패를 quiescent로 해석했다: %#v", got)
	}
}

func TestExecutionOwnerInventoryUsesCompleteTaskAndDispatchInventory(t *testing.T) {
	client := &executionFake{
		readyTasks: []port.OrcaTask{},
		tasks:      []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "dispatched"}},
		dispatch:   port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", Status: "running"},
	}
	got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", RunID: "run-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.TaskLive || got.TaskStatus != "dispatched" || got.DispatchStatus != "running" {
		t.Fatalf("dispatched owner missing from ready inventory must remain live: %#v", got)
	}
	if !reflect.DeepEqual(client.calls, []string{"list-terminals-inventory", "list-run-tasks-inventory", "show-dispatch-inventory"}) {
		t.Fatalf("owner inventory did not use complete task and dispatch views: %v", client.calls)
	}
}

func TestExecutionOwnerInventoryRejectsMissingOrChangedRuntime(t *testing.T) {
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
			client := &executionFake{terminals: tc.terminals, tasks: tc.tasks, dispatch: tc.dispatch}
			if _, err := NewExecutionClient(client).InspectOwner(context.Background(), request); err == nil {
				t.Fatal("owner inventory with an absent or changed runtime identity must fail closed")
			}
		})
	}
}

func TestExecutionOwnerInventoryAllowsExplicitRuntimeRolloverForSettledOwner(t *testing.T) {
	currentRuntime := "runtime-70"
	client := &executionFake{
		tasks:                    []port.OrcaTask{{RuntimeID: currentRuntime, ID: "task-69", Status: "completed"}},
		dispatch:                 port.OrcaDispatch{RuntimeID: currentRuntime, ID: "dispatch-69", TaskID: "task-69", Status: "failed"},
		terminalInventoryRuntime: &currentRuntime,
		taskInventoryRuntime:     &currentRuntime,
		dispatchInventoryRuntime: &currentRuntime,
	}
	request := port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	}
	if _, err := NewExecutionClient(client).InspectOwner(context.Background(), request); err == nil {
		t.Fatal("명시적 허용이 없는 runtime rollover는 계속 거부해야 한다")
	}

	request.AllowRuntimeRollover = true
	got, err := NewExecutionClient(client).InspectOwner(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got.RuntimeID != currentRuntime || got.TerminalID != "" || got.TerminalLive || got.TaskLive ||
		got.TaskStatus != "completed" || got.DispatchStatus != "failed" {
		t.Fatalf("종결된 owner의 현재 runtime 인벤토리를 보존하지 못했다: %#v", got)
	}
}

func TestExecutionOwnerInventoryRuntimeRolloverRequiresOneCurrentRuntime(t *testing.T) {
	terminalRuntime := "runtime-70"
	taskRuntime := "runtime-71"
	dispatchRuntime := "runtime-70"
	client := &executionFake{
		tasks:                    []port.OrcaTask{{RuntimeID: taskRuntime, ID: "task-69", Status: "completed"}},
		dispatch:                 port.OrcaDispatch{RuntimeID: dispatchRuntime, ID: "dispatch-69", TaskID: "task-69", Status: "failed"},
		terminalInventoryRuntime: &terminalRuntime,
		taskInventoryRuntime:     &taskRuntime,
		dispatchInventoryRuntime: &dispatchRuntime,
	}
	if _, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", TaskID: "task-69", DispatchID: "dispatch-69",
		TerminalPTYID: "pty-69", AllowRuntimeRollover: true,
	}); err == nil {
		t.Fatal("서로 다른 current runtime을 보고한 인벤토리는 rollover로 수용하면 안 된다")
	}
}

func TestExecutionIntentReResolvesRotatedTerminalHandle(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentDispatch, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale",
		RunID: "run-69", RunBound: true, TaskID: "task-69",
	}
	client := &executionFake{terminals: []port.OrcaTerminal{{
		RuntimeID: "runtime-69", Handle: "term-current", PTYID: "pty-69", WorktreeID: "wt-69", Title: probe.Marker, Connected: true, Writable: true,
	}}}
	provisioner := NewExecutionClient(client)
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

func TestExecutionIntentDispatchReusesTheSealedTerminalAcrossResumeMarkers(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	request := port.ExecutionOrcaIntentRequest{
		Stage: port.ExecutionOrcaIntentDispatch, Marker: probe.Marker, Workspace: workspace, Probe: probe,
		Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69",
		RunID: "run-69", RunBound: true, TaskID: "task-69",
	}
	client := &executionFake{terminals: []port.OrcaTerminal{{
		RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69",
		Title: "agent-harness issueops-v1 prior-operation", Connected: true, Writable: true,
	}}}
	receipt, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.DispatchID != "dispatch-69" || client.dispatchRequest.ToHandle != "term-69" {
		t.Fatalf("dispatch did not reuse the exact sealed terminal: receipt=%#v request=%#v", receipt, client.dispatchRequest)
	}
}

func TestExecutionIntentEmptyInventoryRequiresSealedRuntimeEnvelope(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
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
				request.RunID = "run-69"
				request.RunBound = true
			}
			if tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TaskID = "task-69"
			}
			client := &executionFake{
				terminalInventoryRuntime: tc.terminalRuntime,
				taskInventoryRuntime:     tc.taskRuntime,
				dispatchInventoryRuntime: tc.dispatchRuntime,
			}
			inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
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

func TestExecutionRunEmptyInventoryRequiresSealedRuntimeEnvelope(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	for _, tc := range []struct {
		name    string
		stage   port.ExecutionOrcaIntentStage
		command string
		payload string
	}{
		{
			name:  "Run create",
			stage: port.ExecutionOrcaIntentRun, command: "orca orchestration run-list --json",
			payload: `{"ok":true,"result":{"runs":[]},"_meta":{"runtimeId":"runtime-other"}}`,
		},
		{
			name:  "Run bind",
			stage: port.ExecutionOrcaIntentRunBind, command: "orca orchestration run-current --json",
			payload: `{"ok":true,"result":{"run":null},"_meta":{"runtimeId":"runtime-other"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := newFakeRunner(t)
			runner.responses[tc.command] = CommandOutput{Stdout: []byte(tc.payload)}
			request := port.ExecutionOrcaIntentRequest{
				Stage: tc.stage, Marker: probe.Marker, Workspace: workspace, Probe: probe,
				Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69",
			}
			if tc.stage == port.ExecutionOrcaIntentRunBind {
				request.RunID = "run-69"
			}
			inventory, err := NewExecutionClient(NewClient(runner)).InspectIntent(context.Background(), request)
			if err == nil {
				t.Fatalf("다른 runtime의 빈 Run inventory를 authoritative zero로 승인했다: %#v", inventory)
			}
		})
	}
}

func TestExecutionOwnerEmptyInventoryRequiresSealedRuntimeEnvelope(t *testing.T) {
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
			client := &executionFake{
				terminalInventoryRuntime: tc.terminalRuntime,
				taskInventoryRuntime:     tc.taskRuntime,
				dispatchInventoryRuntime: tc.dispatchRuntime,
			}
			inventory, err := NewExecutionClient(client).InspectOwner(context.Background(), request)
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

func TestExecutionIntentRejectsUnsealedRuntimeReceipts(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	title := executionTaskTitle(probe.Marker, launch.PromptSHA256)
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
				request.RunID = "run-69"
				request.RunBound = true
			}
			if tc.stage == port.ExecutionOrcaIntentDispatch {
				request.TaskID = "task-69"
			}
			client := &executionFake{terminals: tc.terminal, tasks: tc.tasks, dispatch: tc.dispatch}
			if _, err := NewExecutionClient(client).InspectIntent(context.Background(), request); err == nil {
				t.Fatal("runtime-mismatched receipt must fail closed")
			}
		})
	}
}

func TestExecutionTaskCreateValidatesTheSealedReceipt(t *testing.T) {
	workspace, probe := executionFixture(t)
	prepared := port.ExecutionOrcaWorkspaceReceipt{Workspace: port.ExecutionWorkspaceReceipt{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch, BaseHead: workspace.BaseHead, Driver: "orca", Exists: true,
	}, RuntimeID: "runtime-69", RepoID: "repo-69", WorktreeID: "wt-69"}
	launch := executionLaunchFixture(t, workspace.Root)
	wantTitle := executionTaskTitle(probe.Marker, launch.PromptSHA256)
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
			client := &executionFake{createdTask: &tc.task}
			request := port.ExecutionOrcaIntentRequest{
				Stage: port.ExecutionOrcaIntentTask, Marker: probe.Marker, Workspace: workspace, Probe: probe,
				Prepared: &prepared, Launch: &launch, TerminalPTYID: "pty-69", TerminalHandle: "term-stale",
				RunID: "run-69", RunBound: true,
			}
			if _, err := NewExecutionClient(client).InvokeIntent(context.Background(), request); err == nil {
				t.Fatal("task create receipt that differs from the sealed intent must fail")
			}
			if !reflect.DeepEqual(client.calls, []string{"create-task"}) {
				t.Fatalf("invalid task receipt must make zero dispatch calls: %v", client.calls)
			}
		})
	}
}

func executionFixture(t *testing.T) (port.ExecutionWorkspaceRequest, port.ExecutionOrcaProbeRequest) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "69-redesign")
	workspace := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: filepath.Dir(root), Root: root,
		Branch: "69-redesign", BaseBranch: "68-umbrella", BaseHead: strings.Repeat("a", 40),
		ParentWorktree: filepath.Join(filepath.Dir(root)+".worktrees", "68-umbrella"),
		Confirm:        true,
	}
	request := port.ExecutionOrcaProbeRequest{
		Repo: workspace.SourceRoot, Host: "claude", Model: "caller-selected-model", Effort: "high",
		Provider: "github", Issue: 69,
		Marker: "agent-harness issueops-v1 lifecycle=io-69 operation=operation-69 provider=github issue=69",
	}
	return workspace, request
}

func executionGitLabProbe(request port.ExecutionOrcaProbeRequest) port.ExecutionOrcaProbeRequest {
	request.Provider = "gitlab"
	request.Marker = strings.Replace(request.Marker, "provider=github", "provider=gitlab", 1)
	return request
}

func withExecutionMarker(request port.ExecutionOrcaProbeRequest, marker string) port.ExecutionOrcaProbeRequest {
	request.Marker = marker
	return request
}

func executionLaunchFixture(t *testing.T, root string) port.ExecutionOrcaLaunchRequest {
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

func executionWorktree(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) port.OrcaWorktree {
	return port.OrcaWorktree{
		RuntimeID: "runtime-69", ID: "wt-69", InstanceID: "instance-69", RepoID: "repo-69",
		Path: workspace.Root, Head: workspace.BaseHead, Branch: workspace.Branch, Comment: request.Marker, Issue: 69,
		ParentWorktreeID: "repo-69::" + workspace.ParentWorktree,
		LineageSource:    "explicit-cli-flag", LineageConfidence: "explicit",
	}
}

type executionFake struct {
	calls                     []string
	workspace                 port.ExecutionWorkspaceRequest
	probeRequest              port.ExecutionOrcaProbeRequest
	worktrees                 []port.OrcaWorktree
	worktreeRequest           port.OrcaCreateWorktreeRequest
	terminalRequest           port.OrcaCreateTerminalRequest
	createdTerminal           *port.OrcaTerminal
	runs                      []port.OrcaRun
	currentRun                *port.OrcaRun
	runRequest                port.OrcaCreateRunRequest
	taskRequest               port.OrcaCreateTaskRequest
	dispatchRequest           port.OrcaDispatchRequest
	terminals                 []port.OrcaTerminal
	readyTasks                []port.OrcaTask
	tasks                     []port.OrcaTask
	dispatch                  port.OrcaDispatch
	createdTask               *port.OrcaTask
	terminalDetail            *executionTerminalDetailInventory
	terminalDetailErr         error
	terminalInventoryRuntime  *string
	terminalInventoryComplete *bool
	taskInventoryRuntime      *string
	dispatchInventoryRuntime  *string
}

func (f *executionFake) Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	f.calls = append(f.calls, "probe")
	return port.OrcaProbeResult{Available: true, Ready: true}, nil
}

func (f *executionFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.calls = append(f.calls, "list")
	return append([]port.OrcaWorktree(nil), f.worktrees...), nil
}

func (f *executionFake) CreateWorktree(_ context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	f.calls = append(f.calls, "create-worktree")
	f.worktreeRequest = req
	return executionWorktree(f.workspace, f.probeRequest), nil
}

func (f *executionFake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.calls = append(f.calls, "create-terminal")
	f.terminalRequest = req
	if f.createdTerminal != nil {
		return *f.createdTerminal, nil
	}
	return port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: req.Title, Connected: true, Writable: true}, nil
}

func (f *executionFake) ListRuns(context.Context) ([]port.OrcaRun, error) {
	inventory, err := f.listRunsInventory(context.Background())
	return inventory.Rows, err
}

func (f *executionFake) listRunsInventory(context.Context) (executionRunInventory, error) {
	f.calls = append(f.calls, "list-runs")
	return executionRunInventory{RuntimeID: "runtime-69", Rows: append([]port.OrcaRun(nil), f.runs...)}, nil
}

func (f *executionFake) CreateRun(_ context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error) {
	f.calls = append(f.calls, "create-run")
	f.runRequest = req
	run := port.OrcaRun{RuntimeID: "runtime-69", ID: "run-69", Objective: req.Objective}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *executionFake) CurrentRun(context.Context) (*port.OrcaRun, error) {
	inventory, err := f.currentRunInventory(context.Background())
	return inventory.Run, err
}

func (f *executionFake) currentRunInventory(context.Context) (executionCurrentRunInventory, error) {
	f.calls = append(f.calls, "current-run")
	if f.currentRun == nil {
		return executionCurrentRunInventory{RuntimeID: "runtime-69"}, nil
	}
	current := *f.currentRun
	return executionCurrentRunInventory{RuntimeID: "runtime-69", Run: &current}, nil
}

func (f *executionFake) UseRun(_ context.Context, runID string) (port.OrcaRun, error) {
	f.calls = append(f.calls, "use-run")
	for _, run := range f.runs {
		if run.ID == runID {
			current := run
			f.currentRun = &current
			return current, nil
		}
	}
	return port.OrcaRun{}, fmt.Errorf("run not found")
}

func (f *executionFake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.calls = append(f.calls, "create-task")
	f.taskRequest = req
	if f.createdTask != nil {
		return *f.createdTask, nil
	}
	return port.OrcaTask{RuntimeID: "runtime-69", RunID: req.RunID, ID: "task-69", Title: req.Title, DisplayName: req.DisplayName, Status: "ready"}, nil
}

func (f *executionFake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.calls = append(f.calls, "dispatch")
	f.dispatchRequest = req
	return port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: req.TaskID, AssigneeHandle: req.ToHandle, Injected: true}, nil
}

func (f *executionFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.calls = append(f.calls, "list-terminals")
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *executionFake) listTerminalsInventory(context.Context, string) (executionTerminalInventory, error) {
	f.calls = append(f.calls, "list-terminals-inventory")
	complete := true
	if f.terminalInventoryComplete != nil {
		complete = *f.terminalInventoryComplete
	}
	return executionTerminalInventory{RuntimeID: executionFakeRuntime(f.terminalInventoryRuntime), Rows: append([]port.OrcaTerminal(nil), f.terminals...), Complete: complete}, nil
}

func (f *executionFake) showTerminalInventory(_ context.Context, handle string) (executionTerminalDetailInventory, error) {
	f.calls = append(f.calls, "show-terminal-inventory")
	if f.terminalDetailErr != nil {
		return executionTerminalDetailInventory{}, f.terminalDetailErr
	}
	if f.terminalDetail != nil {
		return *f.terminalDetail, nil
	}
	for _, terminal := range f.terminals {
		if terminal.Handle != handle {
			continue
		}
		paneRuntimeID := 1
		return executionTerminalDetailInventory{
			RuntimeID: executionFakeRuntime(f.terminalInventoryRuntime),
			Terminal:  terminal, PaneRuntimeID: &paneRuntimeID,
		}, nil
	}
	return executionTerminalDetailInventory{}, fmt.Errorf("terminal detail not found")
}

func (f *executionFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	f.calls = append(f.calls, "list-ready-tasks")
	return append([]port.OrcaTask(nil), f.readyTasks...), nil
}

func (f *executionFake) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	f.calls = append(f.calls, "list-all-tasks")
	return append([]port.OrcaTask(nil), f.tasks...), nil
}

func (f *executionFake) listAllTasksInventory(context.Context) (executionTaskInventory, error) {
	f.calls = append(f.calls, "list-all-tasks-inventory")
	return executionTaskInventory{RuntimeID: executionFakeRuntime(f.taskInventoryRuntime), Rows: append([]port.OrcaTask(nil), f.tasks...)}, nil
}

func (f *executionFake) listRunTasksInventory(_ context.Context, runID string, _ ...string) (executionTaskInventory, error) {
	f.calls = append(f.calls, "list-run-tasks-inventory")
	rows := append([]port.OrcaTask(nil), f.tasks...)
	for index := range rows {
		if rows[index].RunID == "" {
			rows[index].RunID = runID
		}
	}
	return executionTaskInventory{RuntimeID: executionFakeRuntime(f.taskInventoryRuntime), Rows: rows}, nil
}

func (f *executionFake) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	f.calls = append(f.calls, "show-dispatch")
	if f.dispatch.ID == "" {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "not_found"}
	}
	return f.dispatch, nil
}

func (f *executionFake) showDispatchInventory(context.Context, string) (executionDispatchInventory, error) {
	f.calls = append(f.calls, "show-dispatch-inventory")
	result := executionDispatchInventory{RuntimeID: executionFakeRuntime(f.dispatchInventoryRuntime)}
	if f.dispatch.ID != "" {
		dispatch := f.dispatch
		result.Dispatch = &dispatch
	}
	return result, nil
}

func executionFakeRuntime(configured *string) string {
	if configured != nil {
		return *configured
	}
	return "runtime-69"
}
