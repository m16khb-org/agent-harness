# Main Agent Heuristic Gate Delegation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the next-action hook a factual trigger and evidence relay only, while the main agent performs all safety, reversibility, alignment, and proceed-or-ask judgement from full task context.

**Architecture:** Keep the external LLM Stop-hook gate disconnected. `UserPromptSubmit` injects the next-action policy up front. The Stop hook only detects that the final assistant message reached a review point, extracts inspectable facts such as the explicit `선택지:` section and recommended option text, and re-enters the main agent with those facts. It must not assign scores, thresholds, safety verdicts, or execution eligibility.

**Tech Stack:** Go, existing `cmd/issueops` hook CLI, `internal/core` lifecycle helpers, project docs in `.issueops`.

---

## Phase 0: Documentation Discovery

Allowed APIs and source contracts:

- `internal/core/lifecycle_state.go:1474`: `NextActionAutoProceedResult` currently contains judgement-shaped fields (`Threshold`, `TopScore`, `AgentJudgementRequired`) that should be replaced or compatibility-wrapped by a factual trigger result.
- `internal/core/lifecycle_state.go:1489`: `EvaluateNextActionAutoProceed` currently scores choices and decides eligibility; this is the behavior to remove from the hook path.
- `cmd/issueops/hook_user_prompt.go:565`: `runHookStop` already says final proceed/ask judgement belongs to the main agent, but it still passes score/threshold language in the reason. That must be removed.
- `internal/core/hook_prompt_test.go:10`: UserPromptSubmit policy injection already frames the main agent as the actor that prepares turn-ending choices.
- `.issueops/ADR.md:279`: the active ADR index says the external LLM gate is disconnected because Stop-hook latency was too high. Refine this decision: not "static heuristic plus policy" as a new judge, but "hook trigger plus evidence relay to main agent."
- `.issueops/CAUTIONS.md:215`: Stop hook must not claim candidate status by itself; it should tell the main agent to re-check its recommended choice and decide from context.

Anti-patterns to avoid:

- Do not reconnect `EvaluateNextActionAutoProceedLLM` to `runHookStop`.
- Do not make the Stop hook execute work directly.
- Do not have the hook calculate numeric scores, thresholds, confidence, safety verdicts, reversibility verdicts, destructive verdicts, or eligibility verdicts.
- Do not use `continue:false` for recoverable Stop-hook feedback.
- Do not parse explanatory numbered lists as next-action choices; only parse inside an explicit `선택지:` / `Options:` / `Next actions:` section.
- Do not lower the 95-point self-verify gate to make this pass.

## File Structure

- Modify `internal/core/lifecycle_state.go`: replace the hook-facing scoring/eligibility helper with a factual trigger helper that parses only explicit next-action sections and returns observed facts.
- Modify `internal/core/lifecycle_state_test.go`: replace score/threshold expectations with factual trigger expectations: no choices, no recommendation, exactly one recommendation, multiple recommendations, and explanatory numbered-list false case.
- Modify `cmd/issueops/hook_user_prompt.go`: update flag help, JSON field labels, and Stop-hook reasons so they say "this review point was reached; here are the observed facts; main agent must judge."
- Modify `cmd/issueops/hook_user_prompt_test.go`: assert the Stop hook re-enters the agent with facts only and never emits score, threshold, candidate, eligibility, safety, or direct auto-execution wording.
- Modify `.issueops/ADR.md` and `.issueops/CAUTIONS.md`: record the sharper policy: hook triggers and relays evidence; main agent judges everything.
- Modify `skills/issueops/SKILL.md` only if its auto-proceed language still says a heuristic scores or decides; preserve IssueOps workflow semantics.

## Task 1: Replace Scoring With A Factual Trigger

**Files:**
- Modify: `internal/core/lifecycle_state.go`
- Test: `internal/core/lifecycle_state_test.go`

- [ ] **Step 1: Write the factual trigger tests**

Add tests that describe the hook contract without scoring:

```go
func TestBuildNextActionJudgementTriggerReportsRecommendedChoiceFacts(t *testing.T) {
	message := strings.Join([]string{
		"구현을 마쳤습니다.",
		"선택지:",
		"1. 진행: 다음 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 현재 상태로 멈춥니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent {
		t.Fatalf("expected factual trigger to re-enter main agent, got %+v", result)
	}
	if result.RecommendedCount != 1 || result.RecommendedText == "" {
		t.Fatalf("expected exactly one recommended choice fact, got %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.Score != 0 || candidate.Destructive {
			t.Fatalf("trigger must not score choices or emit destructive verdicts, got %+v", result)
		}
	}
}
```

Also cover:

```go
func TestBuildNextActionJudgementTriggerDoesNotParseExplanatoryNumberedText(t *testing.T) {
	message := "1. 설명입니다.\n2. 추천이라는 단어가 있습니다.\n3. 끝입니다."
	result := BuildNextActionJudgementTrigger(message)
	if result.ShouldReenterAgent {
		t.Fatalf("numbered explanation without 선택지 header must not trigger, got %+v", result)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/core -run 'BuildNextActionJudgementTrigger' -count=1
```

Expected before implementation: FAIL because `BuildNextActionJudgementTrigger` does not exist.

- [ ] **Step 3: Add the minimal factual DTO and parser**

Add a result type that contains facts only:

```go
type NextActionJudgementTriggerResult struct {
	OK                  bool                  `json:"ok"`
	ShouldReenterAgent  bool                  `json:"should_reenter_agent"`
	ChoicesFound        bool                  `json:"choices_found"`
	ChoiceCount         int                   `json:"choice_count"`
	RecommendedCount    int                   `json:"recommended_count"`
	RecommendedIndex    int                   `json:"recommended_index,omitempty"`
	RecommendedText     string                `json:"recommended_text,omitempty"`
	Reason              string                `json:"reason"`
	Evidence            []string              `json:"evidence"`
	Candidates          []NextActionCandidate `json:"candidates"`
}
```

Implementation rule: it may parse section structure and `(추천)` markers, but must not call `scoreNextActionCandidate`, must not populate score/threshold verdicts, and must not label a choice safe, reversible, destructive, or eligible.

- [ ] **Step 4: Re-run targeted tests**

Run:

```bash
go test ./internal/core -run 'BuildNextActionJudgementTrigger|BuildUserPromptMCPHintsInjectsNextActionPolicy' -count=1
```

Expected: PASS.

## Task 2: Make Stop-Hook Output Relay Evidence Only

**Files:**
- Modify: `cmd/issueops/hook_user_prompt.go`
- Test: `cmd/issueops/hook_user_prompt_test.go`

- [ ] **Step 1: Strengthen hook tests**

Keep or add assertions equivalent to:

```go
if obj["continue"] != true || obj["decision"] != "block" {
	t.Fatalf("expected Stop hook to re-enter main agent with observed facts, got %+v", obj)
}
reason, _ := obj["reason"].(string)
if !strings.Contains(reason, "판단 지점") || !strings.Contains(reason, "근거") || !strings.Contains(reason, "메인 에이전트") {
	t.Fatalf("expected factual trigger directive, got %q", reason)
}
for _, banned := range []string{"점수", "임계값", "안전", "되돌릴 수", "destructive", "eligible", "candidate"} {
	if strings.Contains(reason, banned) {
		t.Fatalf("Stop hook reason must not include hook judgement wording %q: %q", banned, reason)
	}
}
```

- [ ] **Step 2: Update hook help/comments**

Add `--relay-next-action-judgement` as the primary flag for "re-enter the main agent when a final response contains inspectable next-action facts." Keep `--auto-proceed-next-actions` only as a deprecated compatibility alias so existing user hook configs continue to parse.

- [ ] **Step 3: Keep Stop output schema host-safe**

Eligible choice output must stay:

```json
{"continue": true, "decision": "block", "reason": "...observed facts... main agent must judge..."}
```

No-op output must stay:

```json
{}
```

- [ ] **Step 4: Run hook tests**

Run:

```bash
go test ./cmd/issueops -run 'RunHookStop|HookStop|Golden' -count=1
```

Expected: PASS. If usage golden fails because help text changed, update only the relevant golden fixture.

## Task 3: Align Project Docs And Skill Text

**Files:**
- Modify: `.issueops/ADR.md`
- Modify: `.issueops/CAUTIONS.md`
- Maybe modify: `skills/issueops/SKILL.md`

- [ ] **Step 1: Update the ADR wording**

Record this decision in the active ADR area or archive index:

```text
The next-action Stop hook is not a judge, scorer, classifier, or safety gate. It only detects that a review point was reached, extracts inspectable evidence, and re-enters the main agent so the main agent can judge safety, reversibility, alignment, and whether to proceed or ask the user.
```

- [ ] **Step 2: Update caution language**

Ensure `.issueops/CAUTIONS.md` says the Stop hook must not claim "자동진행 후보", score, threshold, destructive verdict, safety verdict, reversibility verdict, or equivalent direct execution authority; it must instruct the main agent to judge from the relayed facts.

- [ ] **Step 3: Search for stale wording**

Run:

```bash
rg -n "자동진행 후보|auto-proceed candidate|directly auto-proceed|auto-continue the recommended|heuristic.*verdict|hook.*decides|score|threshold|eligible|destructive_action" .issueops cmd internal skills docs --glob '!bin/**'
```

Expected: remaining matches are either tests that prohibit stale wording, archived historical notes, or explicitly marked deprecated code.

## Task 4: Contract Verification

**Files:**
- Verify only; no new files expected.

- [ ] **Step 1: Run targeted tests**

Run:

```bash
go test ./internal/core -run 'BuildNextActionJudgementTrigger|BuildUserPromptMCPHints' -count=1
go test ./cmd/issueops -run 'RunHookStop|Golden' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full core/CLI package tests**

Run:

```bash
go test ./internal/core ./cmd/issueops -count=1
```

Expected: PASS.

- [ ] **Step 3: Build the binary**

Run:

```bash
go build -o bin/issueops ./cmd/issueops
```

Expected: exit code 0.

- [ ] **Step 4: Run a hook smoke**

Run:

```bash
printf '{"cwd":"%s","last_assistant_message":"선택지:\n1. 진행: 테스트를 추가하고 구현을 계속합니다. (추천)\n2. 축소 진행: 일부만 검증합니다.\n3. 보류: 멈춥니다."}' "$PWD" | ./bin/issueops hook stop --relay-next-action-judgement
```

Expected: JSON with `continue:true`, `decision:"block"`, and a Korean reason that lists observed facts and tells the main agent to judge from current context. The output must not include numeric score or threshold text.

- [ ] **Step 5: Run the self-verify smoke**

Run:

```bash
./bin/issueops self-verify --seed=100 --target-score=95 --json
```

Expected: `summary.termination_eligible == true` and `summary.minimum_goal_score > 95`.

## Completion Criteria

- The Stop hook path does not calculate scores, thresholds, confidence, destructive verdicts, safety verdicts, reversibility verdicts, or eligibility verdicts.
- A final response with an explicit next-action section and exactly one recommendation re-enters the main agent through Stop-hook `decision:"block"` with observed facts.
- Missing, malformed, no-recommendation, or multi-recommendation choices are reported as facts; the hook does not decide what they mean beyond whether the message reached a review point.
- UserPromptSubmit remains the place where main-agent policy is injected.
- External LLM gate code remains unused and marked deprecated.
- Docs and tests agree that all judgement belongs to the main agent, not the hook.
