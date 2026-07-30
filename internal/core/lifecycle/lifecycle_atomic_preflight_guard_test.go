package lifecycle

import (
	"path/filepath"
	"testing"
)

func TestAtomicCommitWorkflowScriptsAreAdmittedOnlyForCurrentHolder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)

	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	installedSkill := filepath.Join(
		codexHome,
		"skills",
		"atomic-commit-push",
		"scripts",
		"git_preflight.py",
	)
	sourceSkill := filepath.Join(source, "skills", "atomic-commit-push", "scripts", "git_preflight.py")
	workerSkill := filepath.Join(worker, "skills", "atomic-commit-push", "scripts", "git_preflight.py")
	for name, command := range map[string]string{
		"저장소 상대 preflight":        "python3 skills/atomic-commit-push/scripts/git_preflight.py .",
		"cwd 기본 preflight":        "python3 skills/atomic-commit-push/scripts/git_preflight.py",
		"설치된 skill preflight":     "python3 " + installedSkill + " .",
		"source skill preflight":  "python3 " + sourceSkill + " .",
		"expected worktree skill": "python3 " + workerSkill + " .",
		"API 문서 gate":             "python3 skills/atomic-commit-push/scripts/api_doc_gate.py " + worker,
	} {
		t.Run(name, func(t *testing.T) {
			holder := executionRequest(record, worker, "claude", "owner-session", command)
			holder.AgentID = "owner-agent"
			holder.ExpectedWorktree = worker
			if executionObservation(holder) {
				t.Fatalf("저장소 또는 설치 경로의 Python 코드를 일반 관찰 권한으로 승격하면 안 된다: %q", command)
			}
			if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
				t.Fatalf("현재 holder의 atomic workflow preflight는 허용해야 한다: %+v", got)
			}

			foreign := executionRequest(record, worker, "claude", "foreign-session", command)
			foreign.AgentID = "owner-agent"
			foreign.ExpectedWorktree = worker
			got := BuildLifecyclePreToolUseDecision(foreign)
			if got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
				t.Fatalf("동일 명령이어도 non-holder는 차단해야 한다: %+v", got)
			}
		})
	}
}

func TestAtomicCommitWorkflowScriptShapeStaysFailClosed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"다른 스크립트":         "python3 skills/atomic-commit-push/scripts/publish.py .",
		"추가 인자":           "python3 skills/atomic-commit-push/scripts/git_preflight.py . --write",
		"다른 인터프리터":        "python skills/atomic-commit-push/scripts/git_preflight.py .",
		"파일 접미사 위장":       "python3 skills/atomic-commit-push/scripts/git_preflight.py.bak .",
		"skill 디렉터리 위장":   "python3 /tmp/atomic-commit-push/scripts/git_preflight.py .",
		"외부 skills 경로 위장": "python3 /tmp/x/skills/atomic-commit-push/scripts/git_preflight.py .",
		"부모 상대 경로 위장":     "python3 ../skills/atomic-commit-push/scripts/api_doc_gate.py .",
		"스크립트 공백 argv":    "python3 'skills/atomic-commit-push/scripts/git_preflight.py ' .",
		"저장소 공백 argv":     "python3 skills/atomic-commit-push/scripts/git_preflight.py '. '",
		"외부 대상 저장소":       "python3 skills/atomic-commit-push/scripts/git_preflight.py " + filepath.Dir(worker),
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("고정 atomic workflow 형태 밖 명령은 차단해야 한다: %+v", got)
			}
		})
	}
}

func TestAtomicCommitWorkflowUsesCodexExecCommandWorkdir(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	req := executionRequest(record, source, "claude", "owner-session", command)
	req.AgentID = "owner-agent"
	req.Tool = "exec_command"
	req.ToolInput = map[string]any{"cmd": command, "workdir": worker}
	if root, ok := exactAtomicCommitWorkflowScript(req); !ok || root != worker {
		t.Fatalf("Codex exec_command의 실제 workdir를 atomic 대상 root로 사용해야 한다: root=%q ok=%v", root, ok)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("현재 holder가 canonical workdir에서 실행한 Codex preflight는 허용해야 한다: %+v", got)
	}

	outside := filepath.Dir(worker)
	req.CWD = worker
	req.ToolInput = map[string]any{"cmd": command, "workdir": outside}
	if root, ok := exactAtomicCommitWorkflowScript(req); !ok || root != outside {
		t.Fatalf("atomic workflow 분류 결과는 top-level cwd가 아니라 실제 외부 workdir를 보존해야 한다: root=%q ok=%v", root, ok)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("canonical root 밖 실제 workdir의 Codex preflight는 차단해야 한다: %+v", got)
	}
}

func TestAtomicCommitWorkflowRejectsNonShellToolSpoof(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	req := executionRequest(record, worker, "claude", "owner-session", command)
	req.AgentID = "owner-agent"
	req.Tool = "apply_patch"
	req.ToolInput = map[string]any{"patch": "*** Begin Patch\n*** End Patch\n"}
	req.Paths = []string{filepath.Join(filepath.Dir(worker), "outside.txt")}
	req.ExpectedWorktree = worker
	if root, ok := exactAtomicCommitWorkflowScript(req); ok || root != "" {
		t.Fatalf("비-shell tool은 command 문자열만으로 atomic workflow를 위장할 수 없어야 한다: root=%q ok=%v", root, ok)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("비-shell tool의 외부 mutation target은 그대로 차단해야 한다: %+v", got)
	}
}

func TestAtomicCommitWorkflowRejectsRelativeScriptFromWorktreeSubdirectory(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	subdir := filepath.Join(worker, "internal", "core")
	command := "python3 skills/atomic-commit-push/scripts/git_preflight.py ."

	req := executionRequest(record, subdir, "claude", "owner-session", command)
	req.AgentID = "owner-agent"
	if root, ok := exactAtomicCommitWorkflowScript(req); !ok || root != subdir {
		t.Fatalf("회귀 전제: 상대 script 호출은 실제 하위 cwd로 분류돼야 한다: root=%q ok=%v", root, ok)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("상대 skills 경로는 canonical worktree root 밖 cwd에서 허용하면 안 된다: %+v", got)
	}
}

func TestAtomicCommitWorkflowRejectsAbsoluteScriptUnderGenericRepoSubdirectory(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	subdir := filepath.Join(worker, "internal", "core")
	script := filepath.Join(subdir, "skills", "atomic-commit-push", "scripts", "git_preflight.py")
	command := "python3 " + script + " ."

	req := executionRequest(record, subdir, "claude", "owner-session", command)
	req.AgentID = "owner-agent"
	req.Repo = subdir
	if root, ok := exactAtomicCommitWorkflowScript(req); ok || root != "" {
		t.Fatalf("generic repo 하위 절대 script는 atomic workflow로 분류하면 안 된다: root=%q ok=%v", root, ok)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("generic repo 값만으로 하위 절대 atomic script를 신뢰하면 안 된다: %+v", got)
	}
}
