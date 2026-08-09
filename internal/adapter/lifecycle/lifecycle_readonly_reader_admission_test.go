package lifecycle

import (
	"os"
	"path/filepath"
	"testing"
)

// TestActiveLeaseReadOnlyReaderAdmission는 active IssueOps mutation authority에서
// 계획된 검증 루프가 쓰는 읽기 전용 reader form을 고정한다. 각 form은 실행이
// 없는 관찰이므로 owner의 검증을 막아서는 안 된다.
//
// 원 관측: #266 (bash -n), #272 (repo-local SKILL.md reader), #299 (self-verify),
// #301 (bounded process probe), #321 (quoted 정규식 rg).
func TestActiveLeaseReadOnlyReaderAdmission(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)

	writeWorktreeFile(t, worker, filepath.Join("scripts", "install-native.sh"), "#!/bin/bash\ntrue\n")
	writeWorktreeFile(t, worker, filepath.Join("scripts", "verify-child-host-smoke.sh"), "#!/bin/bash\ntrue\n")
	writeWorktreeFile(t, worker, filepath.Join("skills", "issueops", "SKILL.md"), "---\nname: issueops\n---\n")
	writeWorktreeFile(t, worker, filepath.Join("bin", "agent-harness"), "trusted binary\n")

	for _, tc := range []struct {
		name    string
		command string
		issue   string
	}{
		{"bash -n 단일", "bash -n scripts/install-native.sh", "#266"},
		{"bash -n 복수", "bash -n scripts/install-native.sh scripts/verify-child-host-smoke.sh", "#266"},
		{"absolute bash -n", "/bin/bash -n scripts/install-native.sh", "#266"},
		{"cat SKILL.md", "/bin/cat skills/issueops/SKILL.md", "#272"},
		{"sed -n SKILL.md", "/usr/bin/sed -n 1,9999p skills/issueops/SKILL.md", "#272"},
		{"self-verify", "./bin/agent-harness self-verify --seed=100 --target-score=95 --json", "#299"},
		{"full self-verify", "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json", "#448"},
		{"ps 조회", "ps -o pid,command -p 1234", "#301"},
		{"pgrep 조회", "pgrep -f 'go test'", "#301"},
		// #321 quoted 정규식 rg는 이미 허용된다. 회귀 방지로 남긴다.
		{"quoted 정규식 rg", `rg -n 'ValidateExecution|Workspace\[0-9\]' internal`, "#321"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", tc.command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				code := ""
				if got.Deny != nil {
					code = got.Deny.Code
				}
				t.Fatalf("%s: active holder의 읽기 전용 reader는 허용해야 한다: decision=%s code=%s reason=%s",
					tc.issue, got.Decision, code, got.Reason)
			}
		})
	}
}

func TestActiveLeaseDoctorVerificationRequiresExactHolderAndCanonicalRoot(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	writeWorktreeFile(t, worker, filepath.Join("bin", "agent-harness"), "trusted binary\n")
	exact := "./bin/agent-harness doctor --repo '" + worker + "' --json"

	owner := executionRequest(record, worker, "claude", "owner-session", exact)
	owner.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(owner); got.Decision != "allow" {
		t.Fatalf("active holder의 exact doctor verification은 허용해야 한다: %+v", got)
	}

	foreign := owner
	foreign.SessionID = "foreign-session"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" {
		t.Fatalf("다른 session의 doctor verification은 차단해야 한다: %+v", got)
	}

	outside := owner
	outside.Command = "./bin/agent-harness doctor --repo /tmp/outside --json"
	if got := BuildLifecyclePreToolUseDecision(outside); got.Decision != "block" {
		t.Fatalf("canonical root 밖 doctor verification은 차단해야 한다: %+v", got)
	}
}

// TestActiveLeaseReadOnlyReaderDenyMatrix는 위 허용이 shell 실행이나 workspace
// 밖 접근을 함께 열지 않음을 고정한다.
func TestActiveLeaseReadOnlyReaderDenyMatrix(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	writeWorktreeFile(t, worker, filepath.Join("scripts", "install-native.sh"), "#!/bin/bash\ntrue\n")
	writeWorktreeFile(t, worker, filepath.Join("skills", "issueops", "SKILL.md"), "---\nname: issueops\n---\n")

	for _, tc := range []struct {
		name    string
		command string
	}{
		{"bash -c 실행", "bash -c 'rm -rf /'"},
		{"bash 실행", "bash scripts/install-native.sh"},
		{"bash -n stdin", "bash -n"},
		{"cat glob", "/bin/cat skills/*/SKILL.md"},
		{"cat 파이프라인", "/bin/cat skills/issueops/SKILL.md | tee /tmp/x"},
		{"sed -i 치환", "/usr/bin/sed -i s/a/b/ skills/issueops/SKILL.md"},
		{"self-verify 명령치환", "./bin/agent-harness self-verify --seed=$(id -u) --json"},
		{"full self-verify unknown write flag", "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --save-state --json"},
		{"full self-verify redirect", "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json > result.json"},
		{"full self-verify non-listed worktree executable redirect", "./tools/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json > result.json"},
		{"kill 확장", "pkill -f 'go test'"},
		{"ps 파이프 kill", "ps -o pid -p 1234 | xargs kill"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", tc.command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("읽기 허용 확장이 실행·escape 경로를 함께 열었다: command=%q decision=%s", tc.command, got.Decision)
			}
		})
	}
}

func writeWorktreeFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
