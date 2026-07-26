package orca

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/port"
)

const (
	executionLaunchMarker     = "agent-harness issueops-v1 lifecycle=io-timing operation=op-timing"
	executionLaunchWorktreeID = "wt-timing"
	executionLaunchRuntimeID  = "runtime-timing"
)

// executionLaunchTimingFake는 orca가 탭 제목을 늦게 채우는 실환경을 흉내낸다.
//
// terminalInventoryTitles가 조회별 StableTabTitle이다. 첫 항목이 첫 재조회의
// 응답이고, 목록을 다 쓰면 마지막 값을 반복한다 — 상한까지 마커가 나타나지 않는
// 경우를 표현할 수 있다.
//
// 기존 executionFake를 확장하지 않고 별도로 두는 이유는 그 fake가 여러 테스트의
// 공유 픽스처라 조회별 응답 변화를 넣으면 다른 테스트의 전제가 흔들린다는 것이다.
type executionLaunchTimingFake struct {
	calls                   []string
	terminalCreateCalls     int
	inventoryCalls          int
	terminalInventoryTitles []string
	createdTerminal         *port.OrcaTerminal
}

func executionLaunchFake(t *testing.T) *executionLaunchTimingFake {
	t.Helper()
	return &executionLaunchTimingFake{}
}

// executionLaunchSealed는 validateExecutionOwnerLaunch가 요구하는 봉인 파일을
// 실제로 디스크에 만든다. 그 검증은 packet과 prompt를 워크트리 안에서 읽고
// digest까지 대조하므로, 손으로 채운 구조체로는 통과할 수 없다.
func executionLaunchSealed(t *testing.T) (port.ExecutionOrcaWorkspaceReceipt, port.ExecutionOrcaLaunchRequest) {
	t.Helper()
	root := t.TempDir()
	packetPath := filepath.Join(root, "context.json")
	packet := []byte(`{"lifecycle":"io-timing"}`)
	if err := os.WriteFile(packetPath, packet, 0o600); err != nil {
		t.Fatal(err)
	}
	packetDigest := digestExecutionBytes(packet)
	prompt := "owner packet binds " + packetPath + " digest " + packetDigest
	promptPath := filepath.Join(root, "owner-prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared := port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: filepath.Dir(root), Root: root, Branch: "169-timing",
			BaseHead: "0123456789abcdef0123456789abcdef01234567", Driver: "orca", Exists: true,
		},
		RuntimeID: executionLaunchRuntimeID, RepoID: "repo-timing", WorktreeID: executionLaunchWorktreeID,
	}
	launch := port.ExecutionOrcaLaunchRequest{
		ContextPacketPath: packetPath, ContextPacketSHA256: packetDigest,
		PromptPath: promptPath, Prompt: prompt, PromptSHA256: digestExecutionBytes([]byte(prompt)),
	}
	return prepared, launch
}

func executionLaunchProbe() port.ExecutionOrcaProbeRequest {
	return port.ExecutionOrcaProbeRequest{
		Repo: "/repo", Host: "claude", Model: "claude-opus-5", Effort: "high", Marker: executionLaunchMarker,
	}
}

func (f *executionLaunchTimingFake) terminalWithTitle(stableTabTitle string) port.OrcaTerminal {
	return port.OrcaTerminal{
		RuntimeID: executionLaunchRuntimeID, Handle: "term-timing", PTYID: "pty-timing",
		WorktreeID: executionLaunchWorktreeID, Title: "✳ Claude Code", StableTabTitle: stableTabTitle,
		Connected: true, Writable: true,
	}
}

func (f *executionLaunchTimingFake) Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	f.calls = append(f.calls, "probe")
	return port.OrcaProbeResult{Available: true, Ready: true}, nil
}

func (f *executionLaunchTimingFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.calls = append(f.calls, "list")
	return nil, nil
}

func (f *executionLaunchTimingFake) CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	f.calls = append(f.calls, "create-worktree")
	return port.OrcaWorktree{}, errors.New("executionLaunchTimingFake: worktree creation is out of scope")
}

// CreateTerminal의 응답에는 마커가 없다 — 실환경에서 orca가 터미널 제목을
// 에이전트 상태로 덮어쓰고 마커는 탭 제목에 두기 때문이다. createdTerminal로
// 그 전제를 뒤집을 수 있다.
func (f *executionLaunchTimingFake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.calls = append(f.calls, "create-terminal")
	f.terminalCreateCalls++
	if f.createdTerminal != nil {
		return *f.createdTerminal, nil
	}
	return f.terminalWithTitle(""), nil
}

func (f *executionLaunchTimingFake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	f.calls = append(f.calls, "create-task")
	return port.OrcaTask{
		RuntimeID: executionLaunchRuntimeID, ID: "task-timing",
		Title: req.Title, DisplayName: req.DisplayName, Status: "ready",
	}, nil
}

func (f *executionLaunchTimingFake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	f.calls = append(f.calls, "dispatch")
	return port.OrcaDispatch{
		RuntimeID: executionLaunchRuntimeID, ID: "dispatch-timing",
		TaskID: req.TaskID, AssigneeHandle: req.ToHandle, Injected: true,
	}, nil
}

func (f *executionLaunchTimingFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	f.calls = append(f.calls, "list-terminals")
	return nil, nil
}

func (f *executionLaunchTimingFake) listTerminalsInventory(context.Context, string) (executionTerminalInventory, error) {
	f.calls = append(f.calls, "list-terminals-inventory")
	title := ""
	if len(f.terminalInventoryTitles) > 0 {
		index := f.inventoryCalls
		if index >= len(f.terminalInventoryTitles) {
			index = len(f.terminalInventoryTitles) - 1
		}
		title = f.terminalInventoryTitles[index]
	}
	f.inventoryCalls++
	return executionTerminalInventory{
		RuntimeID: executionLaunchRuntimeID,
		Rows:      []port.OrcaTerminal{f.terminalWithTitle(title)},
	}, nil
}

func (f *executionLaunchTimingFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, nil
}

func (f *executionLaunchTimingFake) ListAllTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, nil
}

func (f *executionLaunchTimingFake) listAllTasksInventory(context.Context) (executionTaskInventory, error) {
	return executionTaskInventory{RuntimeID: executionLaunchRuntimeID}, nil
}

func (f *executionLaunchTimingFake) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, nil
}

func (f *executionLaunchTimingFake) showDispatchInventory(context.Context, string) (executionDispatchInventory, error) {
	return executionDispatchInventory{RuntimeID: executionLaunchRuntimeID}, nil
}

func (f *executionLaunchTimingFake) RemoveWorktree(context.Context, string, bool) error {
	return nil
}

func (f *executionLaunchTimingFake) StopTerminals(context.Context, string) error {
	return nil
}

// asOrcaError는 errors.As의 얇은 래퍼다. 테스트가 오류 코드와 Invoked 계약을
// 단언할 때 쓴다.
func asOrcaError(err error, target **port.OrcaError) bool {
	return errors.As(err, target)
}
