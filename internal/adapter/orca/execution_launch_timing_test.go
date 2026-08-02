package orca

import (
	"context"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/port"
)

// orca는 터미널을 만든 뒤 탭 제목을 비동기로 설정한다. 마커는 그 탭 제목에 있고
// 터미널 제목은 에이전트가 자기 상태로 덮어쓴다. 우리 검증이 그 갱신보다 빠르면
// 정상 생성된 터미널을 "봉인과 불일치"로 거부한다.
//
// 실측(#169): 생성 08:32:59.706 → 실패 08:33:02.607. 3초 창에 걸렸고 그 뒤
// reconcile을 세 번 밟아야 준비가 끝났다. 즉 prepare --mode orca --confirm이
// 실환경에서 한 번도 완주하지 못했다.
func TestLaunchOwnerWaitsForTheDelayedTabTitle(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	// 첫 조회에는 마커가 없다 — orca가 아직 visualLayouts를 갱신하지 않았다.
	fake.terminalInventoryTitles = []string{"", executionLaunchMarker}

	provisioner := &ExecutionProvisioner{client: fake}
	receipt, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch)
	if err != nil {
		t.Fatalf("탭 제목 갱신 지연 때문에 정상 준비가 실패하면 orca 모드를 쓸 수 없다: %v", err)
	}
	if strings.TrimSpace(receipt.TaskID) == "" || strings.TrimSpace(receipt.DispatchID) == "" {
		t.Fatalf("터미널 검증을 통과했으면 task와 dispatch까지 진행해야 한다: %+v", receipt)
	}
	if fake.terminalCreateCalls != 1 {
		t.Fatalf("재조회는 관측이다. mutation을 반복하면 #90의 오류다: createCalls=%d", fake.terminalCreateCalls)
	}
}

// Orca terminal create는 terminal list에 새 PTY가 나타나기 전에 반환할 수 있다.
// live #190 실행은 12초 제목 대기 상한이 있는데도 약 1초 만에 실패했다. 첫
// inventory에 PTY가 없을 때 settle loop를 건너뛰던 경로를 재현한다.
func TestLaunchOwnerWaitsForTheCreatedPTYToAppearInInventory(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.terminalInventoryPresent = []bool{false, true}
	fake.terminalInventoryTitles = []string{"", executionLaunchMarker}

	provisioner := &ExecutionProvisioner{
		client: fake, terminalSettleBudget: 30 * time.Millisecond, terminalSettleInterval: 5 * time.Millisecond,
	}
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch); err != nil {
		t.Fatalf("a newly created PTY may appear after the first inventory read: %v", err)
	}
	if fake.inventoryCalls != 2 {
		t.Fatalf("the PTY should settle through read-only inventory, got %d calls", fake.inventoryCalls)
	}
	if fake.terminalCreateCalls != 1 {
		t.Fatalf("waiting must not repeat terminal creation: %d", fake.terminalCreateCalls)
	}
}

// live #190에서 terminal create 응답은 handle과 worktree만 주고 ptyId는 비어
// 있었다. 같은 handle의 단일 terminal이 inventory에 나타나면 그 행에서 PTY를
// 회수해야 한다. 생성 mutation은 이미 끝났으므로 재실행하지 않는다.
func TestLaunchOwnerRecoversPTYOmittedFromCreateReceiptByHandle(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.createdTerminal = &port.OrcaTerminal{
		RuntimeID: executionLaunchRuntimeID, Handle: "term-timing",
		WorktreeID: executionLaunchWorktreeID, Connected: true, Writable: true,
	}
	fake.terminalInventoryTitles = []string{executionLaunchMarker}

	provisioner := &ExecutionProvisioner{
		client: fake, terminalSettleBudget: 30 * time.Millisecond, terminalSettleInterval: 5 * time.Millisecond,
	}
	receipt, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch)
	if err != nil {
		t.Fatalf("create 응답에서 빠진 PTY를 단일 handle 행으로 회수해야 한다: %v", err)
	}
	if receipt.TerminalPTYID != "pty-timing" {
		t.Fatalf("inventory의 PTY가 durable receipt에 남아야 한다: %+v", receipt)
	}
	if fake.inventoryCalls != 1 || fake.terminalCreateCalls != 1 {
		t.Fatalf("조회 1회와 생성 1회만 허용한다: inventory=%d create=%d", fake.inventoryCalls, fake.terminalCreateCalls)
	}
}

func TestLaunchOwnerBoundsAPersistentlyAbsentPTY(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.terminalInventoryPresent = []bool{false}

	provisioner := &ExecutionProvisioner{
		client: fake, terminalSettleBudget: 30 * time.Millisecond, terminalSettleInterval: 5 * time.Millisecond,
	}
	_, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch)
	if err == nil {
		t.Fatal("a PTY that never appears must remain a bounded identity failure")
	}
	var orcaErr *port.OrcaError
	if !asOrcaError(err, &orcaErr) || !strings.Contains(orcaErr.Detail, "attempt") {
		t.Fatalf("bounded absence must preserve attempt evidence: %v", err)
	}
	if fake.inventoryCalls < 2 {
		t.Fatalf("persistent absence must consume the bounded settle window: %d", fake.inventoryCalls)
	}
}

func TestLaunchOwnerDoesNotWaitOnAmbiguousPTYInventory(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.terminalInventoryCopies = []int{2}

	provisioner := &ExecutionProvisioner{
		client: fake, terminalSettleBudget: 30 * time.Millisecond, terminalSettleInterval: 5 * time.Millisecond,
	}
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch); err == nil {
		t.Fatal("duplicate PTY rows are ambiguity, not a settling delay")
	}
	if fake.inventoryCalls != 1 {
		t.Fatalf("ambiguity must fail immediately instead of waiting: %d", fake.inventoryCalls)
	}
}

// 마커가 끝내 나타나지 않으면 여전히 거부한다. 대기는 봉인 검증을 늦추는 것이지
// 없애는 것이 아니다.
func TestLaunchOwnerStillRefusesWhenTheMarkerNeverAppears(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.terminalInventoryTitles = []string{""}

	// 상한을 밀리초로 줄인다 — 이 테스트가 검증하는 것은 "상한을 넘으면
	// 거부한다"이고, 그 상한의 실제 길이는 상수 주석이 근거를 갖는다.
	provisioner := &ExecutionProvisioner{
		client: fake, terminalSettleBudget: 30 * time.Millisecond, terminalSettleInterval: 5 * time.Millisecond,
	}
	_, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch)
	if err == nil {
		t.Fatal("마커가 없는 터미널을 소유자로 받아들이면 봉인이 무의미하다")
	}
	var orcaErr *port.OrcaError
	if !asOrcaError(err, &orcaErr) || orcaErr.Code != "terminal_identity_mismatch" {
		t.Fatalf("기존 오류 코드와 Invoked 계약이 유지돼야 한다: %v", err)
	}
	if !orcaErr.Invoked {
		t.Fatalf("터미널을 만든 뒤 실패했으므로 Invoked여야 한다: %+v", orcaErr)
	}
	// 조용한 재시도는 다음 사람이 타이밍 문제를 다시 발견하게 만든다.
	if !strings.Contains(orcaErr.Detail, "attempt") {
		t.Fatalf("몇 번 다시 읽었는지가 진단에 남아야 한다: %q", orcaErr.Detail)
	}
	if fake.inventoryCalls < 2 {
		t.Fatalf("상한까지 다시 읽어야 한다: inventoryCalls=%d", fake.inventoryCalls)
	}
}

// 첫 조회에 마커가 있으면 추가 대기가 없다. 정상 환경을 느리게 만들지 않는다.
func TestLaunchOwnerDoesNotWaitWhenTheMarkerIsAlreadyThere(t *testing.T) {
	prepared, launch := executionLaunchSealed(t)
	fake := executionLaunchFake(t)
	fake.createdTerminal = &port.OrcaTerminal{
		RuntimeID: executionLaunchRuntimeID, Handle: "term-timing", PTYID: "pty-timing",
		WorktreeID: executionLaunchWorktreeID, Title: executionLaunchMarker, Connected: true, Writable: true,
	}

	provisioner := &ExecutionProvisioner{client: fake}
	if _, err := provisioner.LaunchOwner(context.Background(), prepared, executionLaunchProbe(), launch); err != nil {
		t.Fatal(err)
	}
	if fake.inventoryCalls != 0 {
		t.Fatalf("생성 응답이 이미 봉인과 맞으면 재조회할 이유가 없다: inventoryCalls=%d", fake.inventoryCalls)
	}
}
