package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/adapter/gitworktree"
)

// auto의 목적은 실행 가능한 모드를 고르는 것이다. 그런데 IssueOps 정식 순서를
// 따르면 브랜치가 원격에 이미 있으므로 orca 사전 확인(#149·#154)이 반드시 막는다.
// auto가 orca를 골라놓고 막히면 사용자가 손으로 --mode direct를 다시 줘야 한다 —
// auto가 자기 일을 하지 않은 것이다(#152).
func TestAutoFallsBackToDirectWhenBranchNameIsTaken(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	orca := readyOrcaFake()

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "auto-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatalf("auto는 실행 가능한 모드를 골라야 한다: %v", err)
	}
	if result.ResolvedMode != "direct" {
		t.Fatalf("브랜치 이름이 쓰이고 있으면 auto는 direct로 가야 한다: %+v", result)
	}
	if orca.prepareCalls != 0 {
		t.Fatalf("폴백은 Orca mutation 이전에 결정되어야 한다: prepareCalls=%d", orca.prepareCalls)
	}
}

// 폴백은 조용해서는 안 된다. 왜 direct가 됐는지 알려주지 않으면 사용자는 orca로
// 실행됐다고 오해하거나, orca를 쓰려면 무엇을 바꿔야 하는지 알 수 없다.
func TestAutoFallbackNamesTheBranchConflict(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "auto-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result.FallbackCode) == "" {
		t.Fatalf("조용한 폴백은 사용자를 오해시킨다: %+v", result)
	}
	if !strings.Contains(result.FallbackCode, "branch") {
		t.Fatalf("폴백 코드 %q가 원인을 지목해야 한다", result.FallbackCode)
	}
	if result.RequestedMode != ExecutionModeAuto {
		t.Fatalf("요청 모드는 auto로 남아야 한다: %+v", result)
	}
}

// 명시적으로 orca를 고른 사용자의 의도를 대신 바꾸지 않는다. auto에서만 폴백한다.
func TestExplicitOrcaStillFailsOnBranchConflict(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	orca := readyOrcaFake()

	_, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "explicit-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err == nil {
		t.Fatal("명시적 orca 요청을 조용히 direct로 바꾸면 사용자 의도를 무시한다")
	}
	if orca.prepareCalls != 0 {
		t.Fatalf("mutation 이전에 실패해야 한다: prepareCalls=%d", orca.prepareCalls)
	}
}

// 이름이 어디에도 없으면 auto는 종전대로 orca를 고른다. 폴백이 정상 경로를
// 잡아먹으면 orca 모드가 사라진다.
func TestAutoStillChoosesOrcaWhenTheNameIsFree(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	orca := readyOrcaFake()

	result, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "auto-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: orca, Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResolvedMode != "orca" {
		t.Fatalf("이름이 비어 있으면 auto는 orca를 고른다: %+v", result)
	}
	if strings.TrimSpace(result.FallbackCode) != "" {
		t.Fatalf("폴백하지 않았는데 코드를 채우면 안 된다: %q", result.FallbackCode)
	}
	if orca.prepareCalls != 1 {
		t.Fatalf("Orca가 한 번 호출되어야 한다: prepareCalls=%d", orca.prepareCalls)
	}
}

// preview는 mutation이 아니지만 어느 모드로 갈지는 알려줘야 한다. 그러지 않으면
// confirm에서 모드가 바뀌어 운영자가 본 것과 다른 일이 일어난다.
func TestAutoPreviewShowsTheSameResolvedModeAsConfirm(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)

	preview, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: ExecutionModeAuto, CWD: record.Repo,
		Actor: executionActor("codex", "auto-session"), OwnerHost: "claude",
	}, ExecutionPrepareDependencies{Orca: readyOrcaFake(), Direct: gitworktree.New(), ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ResolvedMode != "direct" || strings.TrimSpace(preview.FallbackCode) == "" {
		t.Fatalf("preview가 confirm과 다른 모드를 보여주면 안 된다: %+v", preview)
	}
}
