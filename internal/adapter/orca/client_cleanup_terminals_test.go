package orca

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

// cleanup의 요청자·종료 대상 판정은 Orca app pid와 런타임 상태를 같은 status
// 응답에서 읽는다(#477).
func TestClientStatusReportsAppPID(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca status --json"] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"app":{"running":true,"pid":903},"runtime":{"state":"ready","reachable":true,"runtimeId":"rt-1"}},"_meta":{"runtimeId":"rt-1"}}`)}
	status, err := NewClient(runner).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.AppPID != 903 || status.RuntimeState != "ready" || !status.RuntimeReachable {
		t.Fatalf("status = %+v, want app pid 903 and ready runtime", status)
	}
}

// 비등록 워크트리의 터미널 목록은 구조화된 selector_not_found다(실측). 그 코드는
// "터미널 없음"이고, 다른 오류만 관측 실패다(#477 brooks 2차 finding 3).
func TestClientListWorktreeTerminalsByPathTreatsSelectorNotFoundAsEmpty(t *testing.T) {
	runner := newFakeRunner(t)
	key := "orca terminal list --worktree path:/tmp/wt-477 --limit " + strconv.Itoa(port.OrcaMaxBaselineIDs) + " --json"
	runner.responses[key] = CommandOutput{Stdout: []byte(`{"id":"x","ok":false,"error":{"code":"selector_not_found","message":"selector_not_found"},"_meta":{"runtimeId":"rt-1"}}`)}
	client := NewClient(runner)
	rows, err := client.ListWorktreeTerminalsByPath(context.Background(), "/tmp/wt-477")
	if err != nil || len(rows) != 0 {
		t.Fatalf("selector_not_found must mean no terminals: rows=%v err=%v", rows, err)
	}

	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":false,"error":{"code":"runtime_unreachable","message":"down"},"_meta":{"runtimeId":"rt-1"}}`)}
	_, err = client.ListWorktreeTerminalsByPath(context.Background(), "/tmp/wt-477")
	if err == nil {
		t.Fatal("other Orca errors must stay errors")
	}
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "runtime_unreachable" {
		t.Fatalf("error must keep the Orca code: %v", err)
	}

	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminals":[{"handle":"term_b","ptyId":"2","worktreeId":"w::/tmp/wt-477","worktreePath":"/tmp/wt-477","tabId":"t","leafId":"l","title":"zsh","connected":true,"writable":true},{"handle":"term_a","ptyId":"1","worktreeId":"w::/tmp/wt-477","worktreePath":"/tmp/wt-477","tabId":"t","leafId":"m","title":"sleep","connected":true,"writable":true}],"totalCount":2,"truncated":false},"_meta":{"runtimeId":"rt-1"}}`)}
	rows, err = client.ListWorktreeTerminalsByPath(context.Background(), "/tmp/wt-477")
	if err != nil || len(rows) != 2 || rows[0].Handle != "term_b" || rows[1].WorktreePath != "/tmp/wt-477" {
		t.Fatalf("rows = %+v err=%v", rows, err)
	}
}

func TestClientListAllTerminalsOmitsSelector(t *testing.T) {
	runner := newFakeRunner(t)
	key := "orca terminal list --limit " + strconv.Itoa(port.OrcaMaxBaselineIDs) + " --json"
	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"terminals":[{"handle":"term_self","ptyId":"29","worktreeId":"w::/repo","worktreePath":"/repo","tabId":"tab-1","leafId":"leaf-1","title":"claude","connected":true,"writable":true}],"totalCount":1,"truncated":false},"_meta":{"runtimeId":"rt-1"}}`)}
	rows, err := NewClient(runner).ListAllTerminals(context.Background())
	if err != nil || len(rows) != 1 || rows[0].TabID != "tab-1" || rows[0].LeafID != "leaf-1" || rows[0].WorktreePath != "/repo" {
		t.Fatalf("rows = %+v err=%v", rows, err)
	}
}

// cleanup은 preview fingerprint가 승인한 exact handle만 닫는다. 성공은 close
// receipt의 handle 일치와 ptyKilled=true까지 확인해야 한다.
func TestClientCloseTerminalRequiresExactHandleAndPTYDeath(t *testing.T) {
	runner := newFakeRunner(t)
	key := "orca terminal close --terminal term_a --json"
	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"close":{"handle":"term_a","tabId":"tab-1","ptyKilled":true}},"_meta":{"runtimeId":"rt-1"}}`)}
	client := NewClient(runner)
	if err := client.CloseTerminal(context.Background(), "term_a"); err != nil {
		t.Fatalf("exact close must accept a confirmed PTY death: %v", err)
	}
	last := runner.calls[len(runner.calls)-1]
	if strings.Join(last, " ") != key {
		t.Fatalf("argv = %q, want %q", strings.Join(last, " "), key)
	}

	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"close":{"handle":"term_a","tabId":"tab-1","ptyKilled":false,"ptyStopVerdict":"live"}},"_meta":{"runtimeId":"rt-1"}}`)}
	if err := client.CloseTerminal(context.Background(), "term_a"); err == nil {
		t.Fatal("an unconfirmed PTY death must fail closed")
	}

	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":true,"result":{"close":{"handle":"term_b","tabId":"tab-1","ptyKilled":true}},"_meta":{"runtimeId":"rt-1"}}`)}
	if err := client.CloseTerminal(context.Background(), "term_a"); err == nil {
		t.Fatal("a mismatched close receipt handle must fail closed")
	}

	runner.responses[key] = CommandOutput{Stdout: []byte(`{"ok":false,"error":{"code":"runtime_error","message":"tab_not_found"},"_meta":{"runtimeId":"rt-1"}}`)}
	if err := client.CloseTerminal(context.Background(), "term_a"); err != nil {
		t.Fatalf("a torn-down background tab is already absent: %v", err)
	}
}

var _ port.CleanupOrcaTerminals = (*Client)(nil)
