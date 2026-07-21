package hookcli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/handoff"
)

// incompleteEngelbartCanvasContent is a meeting Canvas missing the required
// follow-up/appendix/transcript blocks; a create_canvas carrying it must block.
const incompleteEngelbartCanvasContent = "::: {.callout}\n회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요\n:::\n\n## 메타데이터\n|Field|Value|\n|---|---|\n|Date|2026-06-26|\n\n## TL;DR\n- 요약\n\n## 결정사항\n- **결정**\n\n## 액션 보드\n- [ ] 담당: 작업\n\n## 주제별 논의\n### 주제\n- 정리\n\n## 리스크 / 열린 질문\n- 리스크"

// completeEngelbartCanvasContent carries every required template block in order.
func completeEngelbartCanvasContent() string {
	return joinLinesForTest(
		"::: {.callout}",
		"회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요",
		":::",
		"",
		"## 메타데이터",
		"|Field|Value|",
		"|---|---|",
		"|Date|2026-06-26|",
		"",
		"## TL;DR",
		"- 요약",
		"",
		"## 결정사항",
		"- **결정**",
		"",
		"## 액션 보드",
		"- [ ] 담당: 작업",
		"",
		"## 주제별 논의",
		"### 주제",
		"- 정리",
		"",
		"## 후속 확인",
		"- 확인",
		"",
		"## 리스크/열린 질문",
		"- **질문**",
		"",
		"---",
		"",
		"## 보정 및 원문 부록",
		"",
		"### 용어 보정",
		"- `오인식` -> `정확한 표현`. 근거: 사용자 보정. 신뢰도: 높음. 확인 방법: source.",
		"",
		"### 불확실 단어/문장 보정",
		"- 없음.",
		"",
		"### 참석자/화자 보정",
		"- `참석자 1` -> `김현호`. 근거: 사용자 제공. 신뢰도: 높음. 확인 방법: source.",
		"",
		"### 원문 전사본 전문",
		"```text",
		"참석자 1 00:00",
		"원문",
		"```",
	)
}

func joinLinesForTest(lines ...string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}

func writeCanvasToolLine(t *testing.T, tool, content string) map[string]any {
	t.Helper()
	return map[string]any{
		"type":           "tool_use",
		"name":           tool,
		"recipient_name": tool,
		"input": map[string]any{
			"title":   "2026-06-26 [배포] TC NCP 마이그레이션 회의",
			"content": content,
		},
	}
}

func writeTranscriptForTest(t *testing.T, lines ...map[string]any) string {
	t.Helper()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := []byte{}
	for _, line := range lines {
		b, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, b...)
		body = append(body, '\n')
	}
	if err := os.WriteFile(transcript, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return transcript
}

func seedPendingOwnershipCleanupForStop(t *testing.T) (string, string, []byte) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	worker := filepath.Join(repo, "worker")
	if err := os.MkdirAll(worker, 0o700); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(repo, "cleanup-stop-plan.md")
	if err := os.WriteFile(planPath, []byte("# Cleanup Stop plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := "2026-07-21T00:00:00Z"
	baseHead := strings.Repeat("b", 40)
	orca := &issueops.IssueOpsOrcaIdentity{
		RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/cleanup-stop-reentry",
		WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worker,
		WorkerPTYID: "pty-1", WorkerTerminalHandle: "term-1", WorkerMailboxHandle: "term-1",
		TaskID: "task-1", DispatchID: "dispatch-1",
	}
	record := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            issueops.NewIssueOpsID(repo, "cleanup-stop-reentry"),
		Repo:          repo,
		Branch:        "cleanup-stop-reentry",
		Phase:         issueops.IssueOpsPhaseDone,
		IssueURL:      "https://github.com/example/agent-harness/issues/1",
		PlanPath:      planPath,
		WorktreePath:  worker,
		ExecutionHandoff: &issueops.IssueOpsExecutionHandoff{
			State: handoff.StateCleanupPendingHumanDecision, Attempt: 1,
			OwnershipEpoch: "ownership-epoch-1", WorkspaceEpoch: "workspace-epoch-1",
			WorkspaceSHA256: strings.Repeat("c", 64), AttemptBaseHead: baseHead,
			ContextVersion: handoff.ContextVersion, Driver: "orca", Agent: "claude", DeliveryMode: "inject",
			CoordinatorRoot: repo, CoordinatorMailboxHandle: "term-coordinator", WorkerRoot: worker, Orca: orca,
			CoordinatorSession: &issueops.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	workspaceOrca := *orca
	workspaceOrca.WorkerPTYID, workspaceOrca.WorkerTerminalHandle, workspaceOrca.WorkerMailboxHandle = "", "", ""
	workspaceOrca.TaskID, workspaceOrca.DispatchID = "", ""
	record.ExecutionWorkspace = &issueops.IssueOpsExecutionWorkspace{
		State: "ready", WorkspaceEpoch: "workspace-epoch-1", Driver: "orca", Agent: "claude",
		CoordinatorRoot: repo, WorkerRoot: worker, BaseHead: baseHead, Orca: &workspaceOrca,
		PreparationSession: &issueops.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "coordinator-session", AgentID: "coordinator-agent"},
	}
	packet, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatalf("build cleanup handoff context: %v", err)
	}
	record.ExecutionHandoff.ContextSHA256 = packet.SHA256
	record.ExecutionHandoff.ContextSourceSHA256 = packet.SourceSHA256
	record.ExecutionHandoff.OwnerSession = &issueops.IssueOpsHostSessionIdentity{Host: "claude", SessionID: "owner-session", AgentID: "owner-agent"}
	record.ExecutionHandoff.Orientation = &issueops.IssueOpsOwnershipOrientation{
		IssueURL: record.IssueURL, PlanSHA256: packet.PlanSHA256, Understanding: "understood",
		ScopeConfirmation: "worker root only", RecordedAt: now,
	}
	record.ExecutionHandoff.Completion = &issueops.IssueOpsOwnershipCompletion{
		FinalHead: strings.Repeat("f", 40), TuringReport: "reports/cleanup-stop.md",
		Verification: []string{"go test ./..."}, CompletedAt: now,
	}
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("write pending cleanup fixture: %v", err)
	}
	stored, err := issueops.ReadIssueOpsExisting(issueops.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatalf("read pending cleanup fixture: %v", err)
	}
	before, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record.ID, before
}

func TestRunHookStopBoundsOwnershipCleanupRelay(t *testing.T) {
	repo, id, before := seedPendingOwnershipCleanupForStop(t)
	freshInput := `{"cwd":"` + repo + `","session_id":"source-session","last_assistant_message":"작업을 마쳤습니다."}`

	firstNoAuto := runHookCapture(t, `{"cwd":"`+repo+`","session_id":"source-session","last_assistant_message":"자동진행하지 않음 — 사용자 종료 판단을 따릅니다."}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(firstNoAuto) != 0 {
		t.Fatalf("an explicit no-auto-proceed judgement must outrank pending cleanup even on first contact, got %+v", firstNoAuto)
	}

	first := runHookCapture(t, freshInput, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if first["decision"] != "block" {
		t.Fatalf("fresh cleanup Stop must present the human decision once, got %+v", first)
	}
	if reason, _ := first["reason"].(string); !strings.Contains(reason, id) || !strings.Contains(reason, "three human choices") {
		t.Fatalf("cleanup block lost the exact human-choice reason: %q", reason)
	}

	choices := `선택지:\n1. 모든 자원을 유지합니다. (추천)\n2. owner만 닫고 워크트리는 유지합니다.\n3. 모든 로컬 작업 자원을 제거합니다.`
	continuation := runHookCapture(t, `{"cwd":"`+repo+`","session_id":"source-session","stop_hook_active":true,"last_assistant_message":"`+choices+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if len(continuation) != 0 {
		t.Fatalf("cleanup continuation must no-op before the generic next-action relay, got %+v", continuation)
	}

	fresh := runHookCapture(t, freshInput, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if fresh["decision"] != "block" {
		t.Fatalf("a later fresh episode may remind once again, got %+v", fresh)
	}

	afterRecord, err := issueops.ReadIssueOpsExisting(issueops.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(afterRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("Stop hook mutated the IssueOps record\nbefore=%s\nafter=%s", before, after)
	}
}

// TestRunHookStopDoesNotReBlockEngelbartCanvasWhenStopHookActive proves the
// Engelbart gate honors stop_hook_active. Without the guard an incomplete
// create_canvas blocks every Stop, including the continuation Stop the prior
// block triggered, looping forever even after the response recovered.
// Fails before the guard fix (decision=block), passes after (no-op).
func TestRunHookStopDoesNotReBlockEngelbartCanvasWhenStopHookActive(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t, writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", incompleteEngelbartCanvasContent))
	msg := "새 회의록 Canvas를 만들었습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress the Engelbart canvas block to avoid loops, got %+v", obj)
	}
}

// TestRunHookStopClearsEngelbartGateWhenCanvasFixedViaUpdate proves a canvas
// corrected through slack_update_canvas clears the gate. An incomplete
// create_canvas blocks; a following slack_update_canvas that supplies the full
// template is the most recent canvas write and must be accepted as clearing
// evidence. Fails before the update-acceptance + most-recent-scoping fix
// (the stale incomplete create still blocks), passes after (no-op).
func TestRunHookStopClearsEngelbartGateWhenCanvasFixedViaUpdate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t,
		writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", incompleteEngelbartCanvasContent),
		writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_update_canvas", completeEngelbartCanvasContent()),
	)
	msg := "회의록 Canvas를 업데이트했습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("a slack_update_canvas fix with the required sections must clear the Engelbart gate, got %+v", obj)
	}
}
