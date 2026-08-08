package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOwnerMutationExecutableFormParity는 같은 바이너리를 가리키는 세 executable
// form이 동일하게 분류됨을 고정한다(#292). 신뢰 근거는 basename이 아니라
// trusted checkout 경계 안의 `bin/agent-harness`라는 사실이다.
func TestOwnerMutationExecutableFormParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	worktreeBinary := installTrustedHarnessBinary(t, worker)

	tail := " issueops phase --id " + record.ID + " --to implement" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"

	for _, form := range []struct {
		name       string
		executable string
	}{
		{"PATH token", "agent-harness"},
		{"repo-relative", "./bin/agent-harness"},
		{"canonical worktree absolute", worktreeBinary},
	} {
		t.Run(form.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", form.executable+tail)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				code := ""
				if got.Deny != nil {
					code = got.Deny.Code
				}
				t.Fatalf("#292: 동일 바이너리의 %s form은 같게 분류돼야 한다: decision=%s code=%s", form.name, got.Decision, code)
			}
		})
	}
}

// TestReadOnlyIssueOpsReaderExecutableFormParity는 읽기 전용 IssueOps reader의
// absolute form도 같은 경계에서 인정됨을 고정한다(#267).
func TestReadOnlyIssueOpsReaderExecutableFormParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	worktreeBinary := installTrustedHarnessBinary(t, worker)

	command := worktreeBinary + " issueops execution status --id " + record.ID + " --json"
	req := executionRequest(record, worker, "codex", "observer-session", command)
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("#267: trusted worktree 안 absolute reader는 bare form과 같게 통과해야 한다: %+v", got)
	}
}

// TestOwnerMutationExecutableFormFailsClosedOutsideTheCheckout는 위 parity가
// 임의 절대 경로를 열지 않음을 고정한다.
func TestOwnerMutationExecutableFormFailsClosedOutsideTheCheckout(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	outside := filepath.Join(t.TempDir(), "bin", "agent-harness")
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("untrusted"), 0o755); err != nil {
		t.Fatal(err)
	}
	tail := " issueops phase --id " + record.ID + " --to implement" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"

	for _, tc := range []struct {
		name       string
		executable string
	}{
		{"checkout 밖 절대 경로", outside},
		{"checkout 안 다른 파일명", filepath.Join(worker, "bin", "agent-harness-fake")},
		{"bin이 아닌 디렉터리", filepath.Join(worker, "scripts", "agent-harness")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", tc.executable+tail)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("신뢰 경계 밖 executable은 계속 차단돼야 한다: %s -> %s", tc.executable, got.Decision)
			}
		})
	}
}

// TestActiveOwnerRecordLinkingCommandsAreClassified는 public CLI가 제공하지만
// command classifier에 등록되지 않아 active holder가 실행할 수 없던 두 기록
// 명령을 고정한다(#309 link-related, #312 feedback add).
func TestActiveOwnerRecordLinkingCommandsAreClassified(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actor := " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"

	for _, tc := range []struct {
		name    string
		command string
		issue   string
	}{
		{
			"link-related", "agent-harness issueops link-related --id " + record.ID +
				" --type duplicates --related-url https://github.com/m16khb/agent-harness/issues/305" +
				" --title 'related issue'" + actor, "#309",
		},
		{
			"feedback add", "agent-harness issueops feedback add --id " + record.ID +
				" --source brooks --body 'PASS with no findings' --classification contract_change" + actor, "#312",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", tc.command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				code := ""
				if got.Deny != nil {
					code = got.Deny.Code
				}
				t.Fatalf("%s: active current holder의 exact 기록 명령은 허용돼야 한다: decision=%s code=%s",
					tc.issue, got.Decision, code)
			}

			foreign := executionRequest(record, worker, "claude", "foreign-session", tc.command)
			foreign.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" {
				t.Fatalf("%s: foreign holder는 계속 차단돼야 한다: %s", tc.issue, got.Decision)
			}

			unknown := executionRequest(record, worker, "claude", "owner-session", tc.command+" --unknown-flag x")
			unknown.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(unknown); got.Decision != "block" {
				t.Fatalf("%s: unknown flag는 계속 fail-closed여야 한다: %s", tc.issue, got.Decision)
			}
		})
	}
}

// TestChildStartHolderClassificationIsPayloadInvariant는 free-text payload의
// 길이·반복·문자 내용이 holder 판정을 바꾸지 않음을 고정한다(#320).
func TestChildStartHolderClassificationIsPayloadInvariant(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	control := " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	base := "agent-harness issueops child start --parent " + record.ID + " --branch 320-payload"

	shortPayload := " --title child --scope '짧은 범위' --acceptance 'AC-01 하나'"
	longScope := strings.Repeat("긴 한국어 범위 설명과 이슈 참조 #303 그리고 구두점, 쉼표. ", 40)
	var payload strings.Builder
	payload.WriteString(" --title child --scope '" + longScope + "'")
	for index := 1; index <= 7; index++ {
		payload.WriteString(" --acceptance 'AC-0" + string(rune('0'+index)) + " 반복되는 한국어 수용 기준 #303 항목'")
	}
	longPayload := payload.String()

	for _, tc := range []struct {
		name    string
		command string
	}{
		{"짧은 payload", base + shortPayload + control},
		{"긴 한국어 payload와 반복 acceptance 7개", base + longPayload + control},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := executionRequest(record, worker, "claude", "owner-session", tc.command)
			req.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				code := ""
				if got.Deny != nil {
					code = got.Deny.Code
				}
				t.Fatalf("#320: payload 내용이 holder 판정을 바꾸면 안 된다: decision=%s code=%s", got.Decision, code)
			}
		})
	}
}

func installTrustedHarnessBinary(t *testing.T, worktree string) string {
	t.Helper()
	directory := filepath.Join(worktree, "bin")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(directory, "agent-harness")
	if err := os.WriteFile(binary, []byte("trusted worktree binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
