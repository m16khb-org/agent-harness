package orca

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

// #97: orca CLI가 비영 종료해도 stdout에 정상 ok:false envelope을 남기면
// typed 오류 코드를 복원해야 한다. command_failed로 뭉개면 배선층의
// not_found 멱등 정규화가 무력화되어 cleanup finish가 영원히 수렴하지
// 못한다(실측 io-4bd36030750e).
func TestRunJSONRestoresTypedCodeFromFailedCommandEnvelope(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "worktree", "rm", "--worktree", "id:wt-1", "--json"}
	key := strings.Join(argv, " ")
	runner.responses[key] = CommandOutput{Invoked: true, ExitCode: 1, Stdout: []byte(`{"ok":false,"error":{"code":"selector_not_found","message":"selector_not_found"},"_meta":{"runtimeId":"rt-1"}}`)}
	runner.errors[key] = &port.OrcaError{Code: "command_failed", Detail: "stdout: bounded diagnostic", Invoked: true}
	err := NewClient(runner).RemoveWorktree(context.Background(), "wt-1", false)
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "selector_not_found" {
		t.Fatalf("expected typed selector_not_found from failed-command envelope, got %v", err)
	}
}

// envelope이 없는 비영 종료(진짜 전송 실패)는 원래 command_failed를 유지해
// 실패 신호를 삼키지 않는다.
func TestRunJSONKeepsCommandFailedWithoutDecodableEnvelope(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "worktree", "rm", "--worktree", "id:wt-2", "--json"}
	key := strings.Join(argv, " ")
	runner.responses[key] = CommandOutput{Invoked: true, ExitCode: 1, Stdout: []byte("boom: not an envelope")}
	runner.errors[key] = &port.OrcaError{Code: "command_failed", Detail: "stdout: boom", Invoked: true}
	err := NewClient(runner).RemoveWorktree(context.Background(), "wt-2", false)
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "command_failed" {
		t.Fatalf("expected original command_failed without envelope, got %v", err)
	}
}

// 비영 종료인데 envelope이 ok:true인 모순 케이스도 원래 오류를 유지한다.
func TestRunJSONKeepsCommandFailedOnContradictoryOKEnvelope(t *testing.T) {
	runner := newFakeRunner(t)
	argv := []string{"orca", "worktree", "rm", "--worktree", "id:wt-3", "--json"}
	key := strings.Join(argv, " ")
	runner.responses[key] = CommandOutput{Invoked: true, ExitCode: 1, Stdout: []byte(`{"ok":true,"result":{}}`)}
	runner.errors[key] = &port.OrcaError{Code: "command_failed", Detail: "stderr: crashed after write", Invoked: true}
	err := NewClient(runner).RemoveWorktree(context.Background(), "wt-3", false)
	var orcaErr *port.OrcaError
	if !errors.As(err, &orcaErr) || orcaErr.Code != "command_failed" {
		t.Fatalf("expected original command_failed on ok:true contradiction, got %v", err)
	}
}
