package lifecycle

import (
	"strings"
	"testing"
)

// owner가 lease를 쥔 채로 자기 사이클의 준비 상태를 점검할 수 있어야 한다.
// pr-readiness --strict는 PR 생성 직전 게이트를 확인하는 유일한 표면인데,
// 정작 그 시점에 owner는 lease를 쥐고 있다(#135).
func TestExecutionReadOnlyDiagnosticsAreObservations(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"list":            "agent-harness issueops list --json",
		"list by repo":    "agent-harness issueops list --repo " + worker + " --json",
		"cleanup status":  "agent-harness issueops cleanup status --id " + active.ID + " --json",
		"cleanup merged":  "agent-harness issueops cleanup status --id " + active.ID + " --merged --json",
		"pr readiness":    "agent-harness issueops pr-readiness --id " + active.ID + " --json",
		"pr readiness -s": "agent-harness issueops pr-readiness --id " + active.ID + " --strict --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(active, worker, "claude", "owner-session", "")
			req.AgentID, req.Tool, req.Command = "owner-agent", "Bash", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("a read-only diagnostic must be observable while the lease is held: %+v", got)
			}
		})
	}
}

// cleanup 하위는 status 하나만 읽기다. 나머지 넷은 파괴 명령이며 경로 문자열이
// prefix를 공유하므로, 관찰을 넓힐 때 함께 열리기 가장 쉬운 자리다.
func TestExecutionCleanupMutationsStayOutsideObservation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"finish":        "agent-harness issueops cleanup finish --id " + active.ID + " --preview --json",
		"abandon":       "agent-harness issueops cleanup abandon --id " + active.ID + " --reason 포기 --preview --json",
		"remote-branch": "agent-harness issueops cleanup remote-branch --id " + active.ID + " --preview --json",
		"close-children": "agent-harness issueops cleanup close-children --id " + active.ID +
			" --merged --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(active, worker, "claude", "non-holder-session", "")
			req.AgentID, req.Tool, req.Command = "other-agent", "Bash", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision == "allow" {
				t.Fatalf("a cleanup mutation must not gain observation standing: %+v", got)
			}
		})
	}
}

// 관찰 자격은 정확한 형태에만 붙는다. 미지의 플래그나 셸 확장이 섞이면
// 종전대로 거부한다 — 그래야 allowlist가 fail-closed로 남는다.
func TestExecutionReadOnlyDiagnosticsRejectInexactForms(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	for name, command := range map[string]string{
		"unknown flag":   "agent-harness issueops pr-readiness --id " + active.ID + " --deep --json",
		"substitution":   "agent-harness issueops pr-readiness --id $(cat id.txt) --json",
		"piped":          "agent-harness issueops list --json | jq .",
		"missing id":     "agent-harness issueops cleanup status --json",
		"chained mutate": "agent-harness issueops list --json && rm -rf " + worker,
	} {
		t.Run(name, func(t *testing.T) {
			req := executionRequest(active, worker, "claude", "non-holder-session", "")
			req.AgentID, req.Tool, req.Command = "other-agent", "Bash", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision == "allow" {
				t.Fatalf("an inexact form must not be observed: %+v", got)
			}
		})
	}
}

// spec 등록은 플래그 파싱 계약일 뿐이며 쓰기 권한을 주지 않는다.
// 두 allowlist가 별개임을 고정한다.
func TestPRReadinessIsNotAnOwnerMutation(t *testing.T) {
	if exactIssueOpsOwnerMutation("agent-harness issueops pr-readiness --id io-1 --host claude --session-id s --cwd /tmp --json") {
		t.Fatal("registering a command in the flag spec must not grant it mutation standing")
	}
	for _, command := range []string{
		"agent-harness issueops list --json",
		"agent-harness issueops cleanup status --id io-1 --json",
	} {
		if exactIssueOpsOwnerMutation(command) {
			t.Fatalf("a read-only diagnostic must not be an owner mutation: %s", command)
		}
	}
}

// pr-readiness가 IssueOpsCommandSpec에 등록되어 정확한 플래그 집합을 갖는다.
func TestPRReadinessObservationCoversItsFlags(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, active, worker := executionActiveLifecycleRecord(t)

	req := executionRequest(active, worker, "claude", "owner-session", "")
	req.AgentID, req.Tool = "owner-agent", "Bash"
	req.Command = "agent-harness issueops pr-readiness --id " + active.ID + " --strict --json"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "allow" {
		t.Fatalf("pr-readiness with its documented flags must parse and observe: %+v", got)
	}
	if strings.TrimSpace(got.Reason) != "" {
		t.Fatalf("an observed command must carry no block reason: %q", got.Reason)
	}
}
