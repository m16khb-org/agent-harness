package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// 프로세스 게이트는 무엇이 워크트리를 잡고 있는지 이미 안다. InspectProcesses가
// PID와 명령명을 수집하는데 게이트는 개수만 쓰고 버린다. 그래서 차단당한
// 사용자는 lsof를 직접 돌려야 하고, 그 lsof마저 워크트리 경로를 인자로 주면
// lifecycle 가드에 걸린다(이슈 #154).
//
// 관측 결과를 결과에 담으면 무엇을 종료할지 바로 알 수 있다.
func TestCleanupFinishNamesTheProcessesHoldingTheWorktree(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.InspectProcesses = func(string) ([]string, error) {
		return []string{"37393:gopls", "21327:claude"}, nil
	}

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err == nil || !containsString(result.Missing, "workspace_processes_quiescent") {
		t.Fatalf("점유 프로세스는 여전히 차단해야 한다: %v %v", err, result.Missing)
	}
	if len(result.WorkspaceProcesses) == 0 {
		t.Fatal("게이트가 관측한 프로세스를 결과에 담지 않으면 사용자는 직접 lsof를 돌려야 한다")
	}
	joined := strings.Join(result.WorkspaceProcesses, " ")
	for _, want := range []string{"37393", "gopls"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("결과 %v가 %q를 지목해야 한다", result.WorkspaceProcesses, want)
		}
	}
}

// lsof가 실패한 것과 프로세스가 있는 것은 다른 상황이며 다음 행동도 다르다.
// 전자는 관측 도구를 고쳐야 하고 후자는 프로세스를 종료해야 한다. 지금은 둘 다
// 같은 슬러그로 합쳐져 구분되지 않는다.
func TestCleanupFinishSeparatesUnobservableFromOccupied(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	deps := finishDeps(&fakeFinishGit{branchOID: "abc123"})
	deps.InspectProcesses = func(string) ([]string, error) {
		return nil, errors.New("lsof: command not found")
	}

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), deps)
	if err == nil {
		t.Fatal("관측 불가는 fail-closed로 여전히 차단해야 한다")
	}
	if containsString(result.Missing, "workspace_processes_quiescent") {
		t.Fatalf("관측 불가를 프로세스 존재와 같은 슬러그로 보고하면 다음 행동을 정할 수 없다: %v", result.Missing)
	}
	if !containsString(result.Missing, "workspace_processes_observable") {
		t.Fatalf("관측 불가는 자기 슬러그를 가져야 한다: %v", result.Missing)
	}
}

// 해소 명령이 하나로 정해지는 missing은 그 명령을 알려준다. completion_reflected는
// remote reflect-completion으로만 해소되는데, 그것을 알려면 소스에서
// IssueBodyCompletionStartMarker를 grep해야 했다.
func TestCleanupFinishGuidesTheDeterministicRemedy(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)
	req := finishRequest(record.ID, false, "")
	req.CompletionReflected = false

	result, err := CleanupFinish(context.Background(), stateRoot, req, finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	if err == nil || !containsString(result.Missing, "completion_reflected") {
		t.Fatalf("완료 섹션 미반영은 차단되어야 한다: %v %v", err, result.Missing)
	}
	if !strings.Contains(result.NextCommand, "reflect-completion") {
		t.Fatalf("확정적 해소 명령을 안내해야 한다: %q", result.NextCommand)
	}
	if !strings.Contains(result.NextCommand, record.ID) {
		t.Fatalf("안내 명령이 이 사이클을 지목해야 한다: %q", result.NextCommand)
	}
}

// 정상 경로는 그대로다. 새 필드는 차단 시에만 채워진다.
func TestCleanupFinishKeepsQuietPreviewUnchanged(t *testing.T) {
	stateRoot, record, _ := finishTestRecord(t, true)

	result, err := CleanupFinish(context.Background(), stateRoot, finishRequest(record.ID, false, ""), finishDeps(&fakeFinishGit{branchOID: "abc123"}))
	if err != nil {
		t.Fatalf("게이트를 모두 통과한 preview는 fingerprint를 발급해야 한다: %v", err)
	}
	if len(result.WorkspaceProcesses) != 0 {
		t.Fatalf("점유가 없으면 프로세스 목록은 비어 있어야 한다: %v", result.WorkspaceProcesses)
	}
	if strings.Contains(result.NextCommand, "reflect-completion") {
		t.Fatalf("통과한 게이트의 해소 명령을 안내하면 안 된다: %q", result.NextCommand)
	}
}
