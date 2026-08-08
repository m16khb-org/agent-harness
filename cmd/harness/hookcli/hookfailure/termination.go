package hookfailure

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
)

// terminationSignals는 진단을 남길 수 있는 종료 신호다. SIGKILL은 handler를
// 설치할 수 없으므로 여기 없다 — 그 경우는 host runner가 signal receipt의
// 유일한 소유자이고, upstream 추적 대상이다.
var terminationSignals = []os.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP}

// RecordTermination은 hook이 자기 오류가 아니라 외부 신호로 끝났음을 기록한다.
//
// 왜 필요한가: host가 hook 자식을 signal로 끝내면 hook은 exit code를 남기지
// 못하고, 사용자는 "hook exited without a status code"라는 사유 없는 문구만
// 본다. 어느 hook이 어떤 신호로 끝났는지는 죽는 쪽만 알 수 있으므로 여기서
// event·handler identity·termination reason을 함께 남긴다(#268).
func RecordTermination(args []string, stdin []byte, reason string) {
	hook := "unknown"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		hook = strings.TrimSpace(args[0])
	}
	cwd, _ := os.Getwd()
	repo := ArgValue(args, "--repo")
	if repo == "" {
		repo = hookinput.RepoFromHookInput(stdin)
	}
	_, _ = RecordHookFailureEvent(hookfailurecontract.HookFailureEvent{
		Hook:           hook,
		Host:           ArgValue(args, "--host"),
		Repo:           repo,
		CWD:            cwd,
		Tool:           hookinput.ToolNameFromHookInput(stdin),
		Argv:           args,
		CommandSnippet: hookinput.CommandFromHookInput(stdin),
		Error:          "hook terminated before completion: " + reason,
		Termination:    reason,
	})
}

// TerminationReason은 signal을 진단 문자열로 옮긴다. 알 수 없는 신호도 이름을
// 그대로 보존해, 목록에 없는 신호가 진단을 비우지 않게 한다.
func TerminationReason(received os.Signal) string {
	if received == nil {
		return "signal:unknown"
	}
	return "signal:" + received.String()
}

// InstallTerminationDiagnostics는 종료 신호를 관측해 진단을 남기고, 정지를
// 위해 기본 동작을 그대로 재현한다. 반환된 함수는 handler를 해제한다.
//
// 기본 동작을 재현하는 이유: 진단을 남긴 뒤 신호를 삼키면 hook이 종료되지 않아
// host가 기다리게 된다. handler를 풀고 자신에게 같은 신호를 다시 보내 원래
// 종료 의미를 보존한다.
func InstallTerminationDiagnostics(args []string, stdin []byte) func() {
	received := make(chan os.Signal, 1)
	signal.Notify(received, terminationSignals...)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-received:
			RecordTermination(args, stdin, TerminationReason(sig))
			signal.Stop(received)
			if process, err := os.FindProcess(os.Getpid()); err == nil {
				_ = process.Signal(sig)
			}
		case <-done:
		}
	}()
	return func() {
		close(done)
		signal.Stop(received)
	}
}
