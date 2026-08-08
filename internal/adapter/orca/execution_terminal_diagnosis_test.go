package orca

import (
	"strings"
	"testing"

	"agent-harness/internal/port"
)

const diagnosisMarker = "agent-harness issueops-v1 lifecycle=io-1 operation=op-1 provider=github issue=319"

func diagnosisPrepared() port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{RuntimeID: "runtime-1", WorktreeID: "worktree-1"}
}

func diagnosisTerminal() port.OrcaTerminal {
	return port.OrcaTerminal{
		RuntimeID: "runtime-1", Handle: "term_1", PTYID: "pty-1", WorktreeID: "worktree-1",
		StableTabTitle: diagnosisMarker, Connected: true, Writable: true,
	}
}

// TestTerminalIdentityMismatchNamesTheFailingAxis는 #414를 고정한다.
//
// receipt 축(handle·PTY·worktree·runtime·connected·writable)과 marker 축(탭 제목)이
// 같은 문구로 실패하면, 12초를 기다리다 실패했을 때 무엇이 어긋났는지 알 수 없다.
// 실제 dogfood에서 `attempt 16 over 12s`만 남고 원인을 특정할 수 없었다.
func TestTerminalIdentityMismatchNamesTheFailingAxis(t *testing.T) {
	for _, tc := range []struct {
		name     string
		mutate   func(*port.OrcaTerminal)
		wantHint string
	}{
		{"handle 없음", func(term *port.OrcaTerminal) { term.Handle = "" }, "handle"},
		{"PTY 없음", func(term *port.OrcaTerminal) { term.PTYID = "" }, "pty"},
		{"worktree 불일치", func(term *port.OrcaTerminal) { term.WorktreeID = "other" }, "worktree"},
		{"runtime 불일치", func(term *port.OrcaTerminal) { term.RuntimeID = "other" }, "runtime"},
		{"연결 안 됨", func(term *port.OrcaTerminal) { term.Connected = false }, "connected"},
		{"쓰기 불가", func(term *port.OrcaTerminal) { term.Writable = false }, "writable"},
		{"stable 제목이 다른 lifecycle", func(term *port.OrcaTerminal) {
			term.StableTabTitle, term.Title = "agent-harness issueops-v1 lifecycle=io-other", "claude"
		}, "tab title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terminal := diagnosisTerminal()
			tc.mutate(&terminal)
			err := validateExecutionIntentTerminal(terminal, diagnosisPrepared(), diagnosisMarker)
			if err == nil {
				t.Fatal("불일치는 거부돼야 한다")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantHint) {
				t.Fatalf("어느 축이 어긋났는지 밝혀야 한다: got %q want contains %q", err, tc.wantHint)
			}
		})
	}
}

// TestTerminalTitleMismatchReportsBothObservedTitles는 제목 축 실패가 관측값을
// 함께 남기는지 고정한다. 기대값만 있고 관측값이 없으면 다음 사람이 같은
// 타이밍 문제를 처음부터 다시 조사해야 한다.
func TestTerminalTitleMismatchReportsBothObservedTitles(t *testing.T) {
	terminal := diagnosisTerminal()
	terminal.StableTabTitle = "agent-harness issueops-v1 lifecycle=io-other"
	terminal.Title = "claude working"

	err := validateExecutionIntentTerminal(terminal, diagnosisPrepared(), diagnosisMarker)
	if err == nil {
		t.Fatal("제목 불일치는 거부돼야 한다")
	}
	for _, needle := range []string{"lifecycle=io-other", "claude working", diagnosisMarker} {
		if !strings.Contains(err.Error(), needle) {
			t.Fatalf("관측값과 기대값이 모두 남아야 한다: %q에 %q 없음", err, needle)
		}
	}
}

// TestTerminalIdentityAcceptsAnAbsentStableTitle는 #414의 실측을 고정한다.
//
// relay 0.1.0+66c426c5173c는 `stableTabTitle`을 어떤 terminal에도 채우지 않고,
// live `title`은 Orca가 truncate한 뒤 에이전트가 자기 상태로 덮어쓴다
// (관측: stable_tab_title="" title="✳ Claude Code"). 그래서 marker 문자열
// 일치를 요구하면 12초든 그 이상이든 **구조적으로** 맞출 수 없다.
//
// 그 경우 결속 근거는 create 응답의 exact PTY/handle이다 —
// executionSoleCreatedTerminal이 그것으로 한 행만 고르고 둘 이상이면 거부한다.
func TestTerminalIdentityAcceptsAnAbsentStableTitle(t *testing.T) {
	terminal := diagnosisTerminal()
	terminal.StableTabTitle, terminal.Title = "", "✳ Claude Code"
	if err := validateExecutionResolvedTerminal(terminal, diagnosisPrepared(), diagnosisMarker); err != nil {
		t.Fatalf("stable tab title을 제공하지 않는 runtime에서도 결속돼야 한다: %v", err)
	}
	// created 응답 자체에는 완화를 적용하지 않는다 — 그 응답에는 PTY가 아직
	// 없을 수 있고, inventory 대기와 중복 검출이 필요하다.
	if err := validateExecutionIntentTerminal(terminal, diagnosisPrepared(), diagnosisMarker); err == nil {
		t.Fatal("create 응답 자체는 엄격한 marker 계약을 유지해야 한다")
	}
}

// TestTerminalIdentityAcceptsEitherTitleField는 기존 계약을 고정한다. Orca가
// 제목을 채우는 runtime에서는 둘 중 하나만 marker면 통과해야 한다.
func TestTerminalIdentityAcceptsEitherTitleField(t *testing.T) {
	stable := diagnosisTerminal()
	stable.Title = "claude working"
	if err := validateExecutionIntentTerminal(stable, diagnosisPrepared(), diagnosisMarker); err != nil {
		t.Fatalf("StableTabTitle이 marker면 통과해야 한다: %v", err)
	}

	live := diagnosisTerminal()
	live.StableTabTitle, live.Title = "", diagnosisMarker
	if err := validateExecutionIntentTerminal(live, diagnosisPrepared(), diagnosisMarker); err != nil {
		t.Fatalf("Title이 marker면 통과해야 한다: %v", err)
	}
}
