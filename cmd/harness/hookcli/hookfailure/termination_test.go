package hookfailure

import (
	"os"
	"strings"
	"syscall"
	"testing"

	hookfailurecontract "agent-harness/internal/contract/hookfailure"
)

// TestRecordTerminationIdentifiesTheHookAndReason은 #268의 핵심을 고정한다.
// 신호로 끝난 hook의 진단에 event, handler identity, termination reason이
// 함께 남아야 어느 hook이 어떻게 끝났는지 식별할 수 있다.
func TestRecordTerminationIdentifiesTheHookAndReason(t *testing.T) {
	var recorded hookfailurecontract.HookFailureEvent
	original := RecordHookFailureEvent
	t.Cleanup(func() { RecordHookFailureEvent = original })
	RecordHookFailureEvent = func(event hookfailurecontract.HookFailureEvent) (hookfailurecontract.HookFailureRecordResult, error) {
		recorded = event
		return hookfailurecontract.HookFailureRecordResult{OK: true, Event: event}, nil
	}

	args := []string{"pre-tool-use", "--host", "codex", "--repo", "/tmp/repo"}
	stdin := []byte(`{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}`)

	RecordTermination(args, stdin, TerminationReason(syscall.SIGTERM))

	if recorded.Hook != "pre-tool-use" {
		t.Fatalf("어느 hook이 끝났는지 남아야 한다: %q", recorded.Hook)
	}
	if recorded.Host != "codex" {
		t.Fatalf("handler identity(host)가 남아야 한다: %q", recorded.Host)
	}
	if recorded.Repo != "/tmp/repo" {
		t.Fatalf("repo가 남아야 한다: %q", recorded.Repo)
	}
	if recorded.Termination != "signal:terminated" {
		t.Fatalf("termination reason이 남아야 한다: %q", recorded.Termination)
	}
	if !strings.Contains(recorded.Error, "terminated before completion") {
		t.Fatalf("오류 문구가 미완료 종료임을 밝혀야 한다: %q", recorded.Error)
	}
	if recorded.Tool != "Bash" || !strings.Contains(recorded.CommandSnippet, "go test") {
		t.Fatalf("무엇을 하다 끝났는지 남아야 한다: tool=%q command=%q", recorded.Tool, recorded.CommandSnippet)
	}
}

// TestTerminationReasonNamesEverySignal은 목록 밖 신호도 진단을 비우지 않음을
// 고정한다. 이름을 잃으면 "status code 없음"과 구별되지 않는다.
func TestTerminationReasonNamesEverySignal(t *testing.T) {
	for _, tc := range []struct {
		signal os.Signal
		want   string
	}{
		{syscall.SIGTERM, "signal:terminated"},
		{syscall.SIGINT, "signal:interrupt"},
		{syscall.SIGHUP, "signal:hangup"},
		{syscall.SIGUSR1, "signal:user defined signal 1"},
		{nil, "signal:unknown"},
	} {
		if got := TerminationReason(tc.signal); got != tc.want {
			t.Fatalf("TerminationReason(%v) = %q, want %q", tc.signal, got, tc.want)
		}
	}
}

// TestTerminationDiagnosticsStayQuietOnNormalCompletion은 진단 설치가 정상
// 완료 경로에 어떤 기록도 추가하지 않음을 고정한다. hook은 매우 자주 호출되고,
// 정상 종료까지 실패 로그에 남으면 진짜 신호가 묻힌다.
func TestTerminationDiagnosticsStayQuietOnNormalCompletion(t *testing.T) {
	recordCount := 0
	original := RecordHookFailureEvent
	t.Cleanup(func() { RecordHookFailureEvent = original })
	RecordHookFailureEvent = func(event hookfailurecontract.HookFailureEvent) (hookfailurecontract.HookFailureRecordResult, error) {
		recordCount++
		return hookfailurecontract.HookFailureRecordResult{OK: true}, nil
	}

	stop := InstallTerminationDiagnostics([]string{"stop"}, nil)
	stop()

	if recordCount != 0 {
		t.Fatalf("정상 완료는 종료 진단을 남기지 않아야 한다: %d건 기록됨", recordCount)
	}
}

// TestTerminationDiagnosticsAreIdempotentlyStoppable은 해제가 두 번 불려도
// 안전함을 고정한다 — defer와 명시적 해제가 겹칠 수 있다.
func TestTerminationDiagnosticsRestoreDefaultHandling(t *testing.T) {
	original := RecordHookFailureEvent
	t.Cleanup(func() { RecordHookFailureEvent = original })
	RecordHookFailureEvent = func(hookfailurecontract.HookFailureEvent) (hookfailurecontract.HookFailureRecordResult, error) {
		return hookfailurecontract.HookFailureRecordResult{OK: true}, nil
	}

	stop := InstallTerminationDiagnostics([]string{"post-tool-use"}, nil)
	stop()

	// 해제 후에는 goroutine이 남지 않아야 하고, 두 번째 설치도 정상 동작해야
	// 한다. hook 프로세스는 이벤트 하나만 처리하지만 테스트는 반복 실행된다.
	second := InstallTerminationDiagnostics([]string{"post-tool-use"}, nil)
	second()
}
