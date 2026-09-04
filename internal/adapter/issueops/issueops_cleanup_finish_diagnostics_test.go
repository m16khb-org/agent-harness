package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 프로세스 게이트는 무엇이 워크트리를 잡고 있는지 안다(#154). 이제 그 목록은
// 차단 근거가 아니라 apply가 종료할 대상이며, receipt와 자손 수를 함께 싣는다(#477).
func TestCleanupFinishNamesTheProcessesHoldingTheWorktree(t *testing.T) {
	stateRoot, record, worktree := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	gopls := issueops.CleanupWorkspaceProcess{PID: 37393, Command: "gopls", StartedAt: "2026-08-27T00:00:01Z", Executable: "gopls"}
	claude := issueops.CleanupWorkspaceProcess{PID: 21327, Command: "claude", StartedAt: "2026-08-27T00:00:02Z", Executable: "claude"}
	deps.Processes = worldCleanupProcesses(occupiedWorld(t, gopls, claude), nil)
	deps.OrcaTerminals = readyOrca(t, worktree)

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("점유 프로세스는 preview를 막지 않고 종료 대상으로 나열돼야 한다: %v %v", err, result.Missing)
	}
	if len(result.WorkspaceProcesses) != 2 || result.WorkspaceProcesses[0].PID != 21327 || result.WorkspaceProcesses[1].Command != "gopls" {
		t.Fatalf("결과가 점유자를 pid 순으로 receipt와 함께 지목해야 한다: %+v", result.WorkspaceProcesses)
	}
}

// lsof/ps가 실패한 것과 프로세스가 있는 것은 다른 상황이며 다음 행동도 다르다.
// 전자는 관측 도구를 고쳐야 하고 후자는 apply가 종료한다.
func TestCleanupFinishSeparatesUnobservableFromOccupied(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.Processes.Observe = func(string) (port.CleanupWorkspaceOccupancy, error) {
		return port.CleanupWorkspaceOccupancy{}, errors.New("lsof: command not found")
	}

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err == nil {
		t.Fatal("관측 불가는 fail-closed로 여전히 차단해야 한다")
	}
	if !containsString(result.Missing, "workspace_processes_observable") {
		t.Fatalf("관측 불가는 자기 슬러그를 가져야 한다: %v", result.Missing)
	}
	if containsString(result.Missing, "workspace_processes_quiescent") || len(result.WorkspaceProcesses) != 0 {
		t.Fatalf("관측 불가를 점유로 보고하면 안 된다: %v %+v", result.Missing, result.WorkspaceProcesses)
	}
}

// 해소 명령이 하나로 정해지는 missing은 그 명령을 알려준다. completion_reflected는
// reflect-completion 하나로 풀린다.
func TestCleanupFinishGuidesTheDeterministicRemedy(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	req := finishRequest(record.ID, false, "")
	req.CompletionReflected = false
	result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	if err == nil {
		t.Fatal("missing completion must block")
	}
	if !strings.Contains(result.NextCommand, "remote reflect-completion") || !strings.Contains(result.NextCommand, record.ID) {
		t.Fatalf("remedy command must name reflect-completion for this record: %q", result.NextCommand)
	}
}

// 점유가 없으면 결과 목록은 비어 있고 preview는 종전과 같다.
func TestCleanupFinishKeepsQuietPreviewUnchanged(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.WorkspaceProcesses) != 0 || len(result.WorkspaceProcessesStopped) != 0 || len(result.OrcaTerminals) != 0 {
		t.Fatalf("점유가 없으면 프로세스·터미널 목록은 비어 있어야 한다: %+v", result)
	}
}
