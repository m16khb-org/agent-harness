package issueops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

// preview는 mutation 전용 actor 검사를 생략할 수 있지만, confirm이 거부할
// 비정규 CWD를 성공으로 표시해서는 안 된다.
func TestOrcaPreparePreviewRejectsTheSameNoncanonicalCWDAsConfirm(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	badCWD := filepath.Join(record.Repo, "not-the-source-or-worktree")

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: badCWD,
		OwnerHost: "claude",
	}, ExecutionPrepareDependencies{
		Orca: readyOrcaFake(), ReadIssue: executionIssueSnapshotReader,
	})
	if err == nil || !strings.Contains(err.Error(), "Orca prepare cwd") {
		t.Fatalf("preview must reject the same noncanonical cwd as confirm: %v", err)
	}
}

// owner prompt는 원격 이슈를 구현 SSOT로 사용한다. confirm이 봉인할 수 없는
// 본문을 preview에서 성공으로 표시하면 명시적 승인 뒤에야 첫 실패가 드러난다.
func TestOrcaPreparePreviewValidatesTheRemoteIssueOwnerContract(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	readCalls := 0
	incomplete := func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		readCalls++
		return port.ExecutionIssueSnapshot{
			URL:  request.URL,
			Body: "## 문제\n\n수용 기준과 정확한 검증 명령 블록이 아직 없다.\n",
		}, nil
	}

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo,
		OwnerHost: "claude",
	}, ExecutionPrepareDependencies{
		Orca: readyOrcaFake(), ReadIssue: incomplete,
	})
	if err == nil || !strings.Contains(err.Error(), "acceptance IDs") {
		t.Fatalf("preview must reject an owner contract that confirm cannot seal: %v", err)
	}
	if readCalls != 1 {
		t.Fatalf("preview must read the remote issue exactly once, got %d", readCalls)
	}
}
