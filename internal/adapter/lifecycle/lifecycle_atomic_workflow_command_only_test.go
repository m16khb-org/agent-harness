package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAtomicCommitWorkflowSurvivesCodexCommandOnlyPayload는 #331을 고정한다.
// Codex 0.146 stable PreToolUse payload는 exec_command의 workdir를 전달하지
// 않으므로 hook은 top-level source cwd만 본다. 그 상태에서도 current holder는
// canonical worktree에 봉인된 preflight script를 실행할 수 있어야 한다.
//
// 안전 근거는 cwd 추측이 아니라 **자기기술적 호출**이다: script 절대 경로와
// repo 인자 절대 경로가 같은 canonical worktree를 지목할 때만 열린다.
func TestAtomicCommitWorkflowSurvivesCodexCommandOnlyPayload(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)

	for _, script := range []string{"git_preflight.py", "api_doc_gate.py"} {
		t.Run(script, func(t *testing.T) {
			path := installWorktreeAtomicScript(t, worker, script)
			command := "python3 " + path + " " + worker

			// 실제 Codex command-only transport: tool_input에 workdir 없음,
			// top-level cwd는 source checkout.
			req := executionRequest(record, source, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			req.ExpectedWorktree = worker
			if root, ok := exactAtomicCommitWorkflowScript(req); !ok || root != worker {
				t.Fatalf("자기기술적 절대 호출은 canonical root로 분류돼야 한다: root=%q ok=%v", root, ok)
			}
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				code := ""
				if got.Deny != nil {
					code = got.Deny.Code
				}
				t.Fatalf("#331: command-only payload에서 holder의 봉인된 preflight는 허용돼야 한다: decision=%s code=%s reason=%s",
					got.Decision, code, got.Reason)
			}
		})
	}
}

// TestAtomicCommitWorkflowCommandOnlyPayloadStaysFailClosed는 위 허용이 임의
// Python 실행이나 다른 repo를 열지 않음을 고정한다.
func TestAtomicCommitWorkflowCommandOnlyPayloadStaysFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	preflight := installWorktreeAtomicScript(t, worker, "git_preflight.py")
	otherRepo := t.TempDir()

	for _, tc := range []struct {
		name    string
		command string
	}{
		{"다른 repo 인자", "python3 " + preflight + " " + otherRepo},
		{"repo 인자 없음", "python3 " + preflight},
		{"임의 python script", "python3 " + filepath.Join(worker, "tool.py") + " " + worker},
		{"bare python -c", "python3 -c 'print(1)'"},
		{"script 상대 경로", "python3 skills/atomic-commit-push/scripts/git_preflight.py " + worker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, source, "claude", "owner-session", tc.command)
			req.AgentID = "owner-agent"
			req.ExpectedWorktree = worker
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("command-only 허용이 경계를 넘었다: %q -> %s", tc.command, got.Decision)
			}
		})
	}
}

// TestAtomicCommitWorkflowCommandOnlyRejectsForeignHolder는 자기기술적 호출이
// holder 검사를 우회하지 않음을 고정한다.
func TestAtomicCommitWorkflowCommandOnlyRejectsForeignHolder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	preflight := installWorktreeAtomicScript(t, worker, "git_preflight.py")

	foreign := executionRequest(record, source, "claude", "foreign-session", "python3 "+preflight+" "+worker)
	foreign.AgentID = "owner-agent"
	foreign.ExpectedWorktree = worker
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" {
		t.Fatalf("foreign holder는 계속 차단돼야 한다: %s", got.Decision)
	}
}

func installWorktreeAtomicScript(t *testing.T, worktree, name string) string {
	t.Helper()
	directory := filepath.Join(worktree, "skills", "atomic-commit-push", "scripts")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
