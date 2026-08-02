package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/lifecycle"
)

const hookChoiceQualityEvidenceEscaped = `\n\n## 선택지 품질 증거\n- context 확인: git status, 테스트 결과, 사용자 요청 범위를 확인했습니다.\n- 추천 근거: safe=상태 변경 없음, reversible=되돌릴 작업 없음, aligned=사용자 요청 범위와 일치합니다.\n- 사용자 승인 경계: 원격 push/delete/destructive 작업은 추천하지 않았습니다.`

func TestRunHookStopEmitsCodexCompatibleNoopJSON(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	out := captureStdoutForTest(t, func() {
		if err := runHookStop([]string{"--repo", repo}); err != nil {
			t.Fatalf("runHookStop: %v", err)
		}
	})
	var obj map[string]any
	if err := json.Unmarshal([]byte(out), &obj); err != nil {
		t.Fatalf("stop hook output is not JSON: %q: %v", out, err)
	}
	if len(obj) != 0 {
		t.Fatalf("Stop hook host output must be a no-op object, got %s", out)
	}
	if strings.Contains(out, "hookSpecificOutput") || strings.Contains(out, "additionalContext") {
		t.Fatalf("Stop hook output contains unsupported injection fields: %s", out)
	}
}

func TestRunHookStopBlocksMissingNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	// continue must stay true so the agent continues in-turn and presents the
	// choices; continue:false would hard-stop and surface the reason to the user.
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopBlocksEngelbartCanvasMissingRequiredBlocks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	canvasContent := "::: {.callout}\n회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요\n:::\n\n## 메타데이터\n|Field|Value|\n|---|---|\n|Date|2026-06-26|\n\n## TL;DR\n- 요약\n\n## 결정사항\n- **결정**\n\n## 액션 보드\n- [ ] 담당: 작업\n\n## 주제별 논의\n### 주제\n- 정리\n\n## 리스크 / 열린 질문\n- 리스크"
	line := map[string]any{
		"type":           "tool_use",
		"name":           "mcp__codex_apps__slack._slack_create_canvas",
		"recipient_name": "mcp__codex_apps__slack._slack_create_canvas",
		"input": map[string]any{
			"title":   "2026-06-26 [배포] TC NCP 마이그레이션 회의",
			"content": canvasContent,
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	msg := "새 회의록 Canvas를 만들었습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected missing Engelbart blocks to block Stop, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	for _, want := range []string{"## 후속 확인", "## 보정 및 원문 부록", "### 원문 전사본 전문", "원문 text 코드블록"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("expected reason to name missing block %q, got %q", want, reason)
		}
	}
}

func TestRunHookStopAllowsEngelbartCanvasWithRequiredBlocks(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	canvasContent := strings.Join([]string{
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
	}, "\n")
	line := map[string]any{
		"type":           "tool_use",
		"name":           "mcp__codex_apps__slack._slack_create_canvas",
		"recipient_name": "mcp__codex_apps__slack._slack_create_canvas",
		"input": map[string]any{
			"title":   "2026-06-26 [배포] TC NCP 마이그레이션 회의",
			"content": canvasContent,
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	msg := "새 회의록 Canvas를 만들었습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("expected complete Engelbart Canvas blocks to allow Stop, got %+v", obj)
	}
}

func TestRunHookStopAllowsEngelbartCanvasWithWebAPISafeStatusQuote(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	calloutHeader := strings.Join([]string{
		"::: {.callout}",
		"회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요",
		":::",
	}, "\n")
	quoteHeader := "> 회의일 2026-06-26 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요"
	canvasContent := strings.Replace(completeEngelbartCanvasContent(), calloutHeader, quoteHeader, 1)
	transcript := writeTranscriptForTest(t, writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", canvasContent))
	msg := "새 회의록 Canvas를 만들었습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("expected Web API-safe top status quote to satisfy Engelbart Canvas blocks, got %+v", obj)
	}
}

func TestRunHookStopAllowsEngelbartDiscussionWithoutCanvasWrite(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "회의록 Canvas를 생성하기 전에 회의 내용을 정리했습니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("discussion without a Slack Canvas write must allow Stop, got %+v", obj)
	}
}

func TestRunHookStopAllowsEngelbartDiscussionWithUnreadableTranscript(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "missing-transcript.jsonl")
	msg := "새 회의록 Canvas를 만들었습니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("an unreadable transcript without a Slack Canvas write must allow Stop, got %+v", obj)
	}
}

func TestRunHookStopBlocksIncompleteEngelbartCreateDespiteCompleteAssistantProse(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t, writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", incompleteEngelbartCanvasContent))
	msg := "새 회의록 Canvas를 만들었습니다.\\n" + strings.ReplaceAll(completeEngelbartCanvasContent(), "\n", "\\n")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("an incomplete Slack Canvas create must block despite complete assistant prose, got %+v", obj)
	}
}

func TestRunHookStopBlocksEmptyEngelbartCreateWithoutMeetingKeywords(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := writeTranscriptForTest(t, writeCanvasToolLine(t, "mcp__codex_apps__slack._slack_create_canvas", ""))
	msg := "Canvas created."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("an empty Slack Canvas create must not bypass the required-section gate, got %+v", obj)
	}
}

func TestRunHookStopRelaysOncePerTurnAndRefreshesRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	choicesA := `선택지:\n1. 계획 A로 진행합니다. (추천)\n2. 검증만 합니다.\n3. 보류합니다.`
	choicesB := `선택지:\n1. 계획 B로 진행합니다. (추천)\n2. 리뷰만 합니다.\n3. 보류합니다.`

	first := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+choicesA+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	if first["decision"] != "block" {
		t.Fatalf("first stop with fresh choices must relay the judgement, got %+v", first)
	}

	second := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"`+choicesB+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	if len(second) != 0 {
		t.Fatalf("continuation stop with changed choices must not re-relay, got %+v", second)
	}

	record, found := lifecycle.ReadStopNextActionRelay(repo)
	if !found {
		t.Fatalf("relay record must survive the turn for choice-reply expansion")
	}
	if len(record.Candidates) != 3 || !strings.Contains(record.Candidates[0].Text, "계획 B") {
		t.Fatalf("relay record must hold the latest displayed choices, got %+v", record)
	}
}

func TestRunHookStopRelayNamesOrchestrationMissingKeys(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, parent := seedStopRelayOrchestrationFixture(t)
	choices := `선택지:\n1. 검증 후 PR 단계로 진행합니다. (추천)\n2. child 상태만 확인합니다.\n3. child 실행 증거를 확인합니다.` + hookChoiceQualityEvidenceEscaped

	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+choices+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to relay the judgement, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	for _, want := range []string{
		"child_incomplete:" + issueops.NewIssueOpsID(repo, "relay-child-active"),
		"child_unvalidated:" + issueops.NewIssueOpsID(repo, "relay-child-done"),
		parent.ID,
	} {
		if !strings.Contains(reason, want) {
			t.Fatalf("expected Stop relay reason to name %q, got %q", want, reason)
		}
	}
	if strings.Contains(reason, "pool_incomplete:") {
		t.Fatalf("Stop relay must not surface stale pool state, got %q", reason)
	}
}

func TestRunHookUserPromptConsumesRelayAfterExpansion(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	choices := `선택지:\n1. 계획 A로 진행합니다. (추천)\n2. 검증만 합니다.\n3. 보류합니다.`
	runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+choices+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})

	out := runHookCapture(t, `{"cwd":"`+repo+`","prompt":"1"}`, func() error {
		return runHookUserPrompt([]string{"--host", "claude"})
	})
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), "계획 A") {
		t.Fatalf("bare choice reply must expand to the recorded option text, got %s", b)
	}
	if _, found := lifecycle.ReadStopNextActionRelay(repo); found {
		t.Fatalf("a real user prompt must consume the relay record after expansion")
	}
}

func seedStopRelayOrchestrationFixture(t *testing.T) (string, issueopscontract.IssueOpsRecord) {
	t.Helper()
	repo := t.TempDir()
	now := "2026-07-07T00:00:00Z"
	parentID := issueops.NewIssueOpsID(repo, "relay-parent")
	childDoneID := issueops.NewIssueOpsID(repo, "relay-child-done")
	childActiveID := issueops.NewIssueOpsID(repo, "relay-child-active")
	parent := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            parentID,
		Repo:          repo,
		Branch:        "relay-parent",
		Phase:         issueops.IssueOpsPhaseImplement,
		WorktreePath:  repo,
		Execution: &issueopscontract.Execution{
			Mode: issueopscontract.ExecutionModeDirect,
			Workspace: issueopscontract.Workspace{
				SourceRoot: repo,
				Root:       filepath.Join(repo, "relay-parent-worktree"),
				Branch:     "relay-parent",
				BaseHead:   strings.Repeat("a", 40),
				Driver:     "git",
				LinkedAt:   now,
			},
			Lease: issueopscontract.WriteLease{
				Generation:       1,
				Status:           issueopscontract.LeaseStatusClaimable,
				ClaimTokenSHA256: strings.Repeat("b", 64),
			},
		},
		ChildCycles: []issueopscontract.IssueOpsChildCycleRef{
			{CycleID: childDoneID, Branch: "relay-child-done", CreatedAt: now},
			{CycleID: childActiveID, Branch: "relay-child-active", CreatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	writeStopRelayIssueOpsRecord(t, parent)
	writeStopRelayIssueOpsRecord(t, issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childDoneID,
		Repo:          repo,
		Branch:        "relay-child-done",
		Phase:         issueops.IssueOpsPhaseDone,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	writeStopRelayIssueOpsRecord(t, issueopscontract.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsCurrentSchemaVersion,
		ID:            childActiveID,
		Repo:          repo,
		Branch:        "relay-child-active",
		Phase:         issueops.IssueOpsPhaseImplement,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	seedStopRelayStalePoolEntry(t, parent.ID, "wp-relay001")
	return repo, parent
}

func seedStopRelayStalePoolEntry(t *testing.T, parentID, id string) {
	t.Helper()
	root := filepath.Join(os.Getenv("HARNESS_STATE_DIR"), strings.Join([]string{"work", "pool"}, ""))
	if err := os.MkdirAll(filepath.Join(root, id), 0o700); err != nil {
		t.Fatalf("mkdir legacy fixture: %v", err)
	}
	manifest := `{"id":"` + id + `","name":"legacy","parent_cycle_id":"` + parentID + `","status":"active"}`
	if err := os.WriteFile(filepath.Join(root, id+".json"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write legacy manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, id, "task-stale.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}
}

func writeStopRelayIssueOpsRecord(t *testing.T, record issueopscontract.IssueOpsRecord) {
	t.Helper()
	if _, err := issueops.WriteIssueOps(issueops.IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("write issueops record %s: %v", record.ID, err)
	}
}

func TestRunHookStopDoesNotRequireFullEngelbartTemplateForCanvasUpdates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	line := map[string]any{
		"type":           "tool_use",
		"name":           "mcp__codex_apps__slack._slack_update_canvas",
		"recipient_name": "mcp__codex_apps__slack._slack_update_canvas",
		"input": map[string]any{
			"canvas_id": "F0TEST",
			"action":    "append",
			"content":   "### 원문 전사본 전문\n```text\n부분 업데이트\n```",
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	msg := "회의록 Canvas를 업데이트했습니다: https://bubbletap.slack.com/docs/T048JBUDF9U/F0TEST"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-engelbart-canvas-sections"})
	})
	if len(obj) != 0 {
		t.Fatalf("partial slack_update_canvas payloads must not be forced to contain the full template, got %+v", obj)
	}
}

func TestRunHookStopAllowsStopWhenStopHookActiveMissingChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","stop_hook_active":true,"last_assistant_message":"작업을 진행했습니다."}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("stop_hook_active must suppress the next-action block to avoid loops, got %+v", obj)
	}
}

func TestRunHookStopAllowsNoAutoProceedJudgementWithoutChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	message := strings.Join([]string{
		"자동진행하지 않겠습니다.",
		"",
		"판단 근거: 추천안인 --no-verify는 저장소가 의도적으로 설치한 pre-commit 게이트를 우회하는 행위입니다.",
		"사용자의 명시적 승인 없이 같은 작업을 자동으로 재개하지 않고 멈춥니다.",
	}, "\\n")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+message+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	if len(obj) != 0 {
		t.Fatalf("no-auto-proceed Stop relay response without choices must be allowed to stop, got %+v", obj)
	}
}

func TestRunHookStopBlockReasonTellsAgentToPresentContextSpecificChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	message := "완료했습니다.\n\nCommit: 18172349f3 fix(dmm): use persona tags for ranking registration\nIssueOps phase: pr, strict readiness 통과"
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+strings.ReplaceAll(message, "\n", "\\n")+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices with an in-turn continuation, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "차단 원인") || !strings.Contains(reason, "한국어로 작성한 `선택지:`") || !strings.Contains(reason, "no-auto-proceed") {
		t.Fatalf("expected Stop hook reason to tell the agent why it blocked and to create context-specific choices, got %q", reason)
	}
}

func TestRunHookStopAllowsNumberedNextActionsWhenExpected(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"선택지:\n1. 진행: 검증합니다. (추천)\n2. 축소 진행: 일부만 합니다.\n3. 보류: 멈춥니다.`+hookChoiceQualityEvidenceEscaped+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("expected Stop hook to allow numbered choices with no-op output, got %+v", obj)
	}
}

func TestRunHookStopBlocksNumberedNextActionsWithoutOneRecommendation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, message := range []string{
		"선택지:\\n1. 진행: 검증합니다.\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다.",
		"선택지:\\n1. 진행: 검증합니다. (추천)\\n2. 축소 진행: 일부만 합니다. (추천)\\n3. 보류: 멈춥니다.",
	} {
		obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+message+`"}`, func() error {
			return runHookStop([]string{"--enforce-numbered-next-actions"})
		})
		if obj["continue"] != true || obj["decision"] != "block" {
			t.Fatalf("expected malformed recommendations to block, got %+v", obj)
		}
	}
}

func TestRunHookStopDoesNotTreatNumberedExplanationAsAutoProceedChoices(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := strings.Join([]string{
		"설명입니다.",
		"1. Stop hook이 먼저 정적 휴리스틱으로 자동진행 후보를 판정합니다.",
		"2. 메인 에이전트는 실행 여부만 판단합니다.",
		"3. `agent-harness`가 추천 선택지를 분석해서 자동진행 후보라고 판단합니다.",
	}, "\\n")
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to block missing choices, got %+v", obj)
	}
	if !strings.Contains(reason, "올바른 numbered next actions") {
		t.Fatalf("expected missing-next-actions block, got %+v", obj)
	}
	if strings.Contains(reason, "자동진행 후보") {
		t.Fatalf("numbered explanatory text must not be treated as auto-proceed choices, got %q", reason)
	}
}

func TestRunHookStopDoesNotAutoProceedDestructiveCleanup(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	msg := "선택지:\\n1. 정리 진행: merged worktree와 branch를 삭제합니다. (추천)\\n2. 보류: 유지합니다.\\n3. 확장 정리: 전체를 점검합니다."
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--relay-next-action-judgement"})
	})
	reason, _ := obj["reason"].(string)
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected facts relay block for destructive-looking text, got %+v", obj)
	}
	if !strings.Contains(reason, "판단 지점") || !strings.Contains(reason, "추천 선택지") {
		t.Fatalf("expected factual trigger reason, got %+v", obj)
	}
	for _, banned := range []string{"destructive", "irreversible", "점수", "임계값", "자동 진행 미적용"} {
		if strings.Contains(reason, banned) {
			t.Fatalf("Stop hook must not judge destructive-looking text with %q: %+v", banned, obj)
		}
	}
}

func TestRunHookStopDoesNotRelayNextActionJudgementWithoutFlag(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// The relay flag is the on/off switch. Even with a valid recommended choice
	// present, omitting it must not re-enter the main agent.
	repo := t.TempDir()
	msg := "선택지:\\n1. 진행: 구현을 계속합니다. (추천)\\n2. 축소 진행: 일부만 합니다.\\n3. 보류: 멈춥니다." + hookChoiceQualityEvidenceEscaped
	obj := runHookCapture(t, `{"cwd":"`+repo+`","last_assistant_message":"`+msg+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if len(obj) != 0 {
		t.Fatalf("next-action judgement relay must require its flag; got %+v", obj)
	}
}

func TestRunHookStopReadsLastAssistantMessageFromTranscript(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"작업했습니다."}]}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions"})
	})
	if obj["continue"] != true || obj["decision"] != "block" {
		t.Fatalf("expected Stop hook to inspect transcript and block with an in-turn continuation, got %+v", obj)
	}
}

func TestRunHookStopIgnoresSystemTranscriptObjectWithAssistantText(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	body := `{"type":"system","text":"assistant reminder without a final assistant response"}` + "\r\n"
	if err := os.WriteFile(transcript, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, `{"cwd":"`+repo+`","transcript_path":"`+transcript+`"}`, func() error {
		return runHookStop([]string{"--enforce-numbered-next-actions", "--json"})
	})
	next, _ := obj["numbered_next_actions"].(map[string]any)
	if next["decision"] != "allow" || next["reason"] != "no assistant message available to inspect" {
		t.Fatalf("expected system transcript object to be ignored, got %+v", obj)
	}
}
