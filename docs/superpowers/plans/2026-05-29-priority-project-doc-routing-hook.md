# Priority Project-Doc Routing Hook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `agent-harness hook user-prompt` from flat document candidates to priority-based `.agent-harness` routing with Required, Consider, Route, and Secondary companion sections.

**Architecture:** Keep routing deterministic and local in `internal/core/hook_prompt.go`. Extend `HookUserPromptHint` with a backward-compatible `priority` JSON field, classify hints at creation time, and render grouped sections. Update `internal/core/hook_prompt_test.go` to lock priority ordering, false-positive reduction, and existing API/tool behavior.

**Tech Stack:** Go standard library, existing `go test` suite, existing `agent-harness` CLI.

---

## File Structure

- Modify `internal/core/hook_prompt.go`: add hint priority constants, priority-aware add helper, stronger phrase matching for noisy short terms, and grouped rendering.
- Modify `internal/core/hook_prompt_test.go`: add tests for Required/Consider/Route grouping, ambiguous prompts, false-positive prevention, and secondary companion placement.
- Create this plan file under `docs/superpowers/plans/`.

---

### Task 1: Add failing priority-routing tests

**Files:**
- Modify: `internal/core/hook_prompt_test.go`

- [ ] **Step 1: Add tests that describe the priority sections**

Append tests that assert:

```go
func TestBuildUserPromptMCPHintsUsesPrioritySections(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 테스트 검증을 설계해줘"})
	for _, want := range []string{"Required project docs:", "Consider project docs:", "Route if ambiguous:", "ARCHITECTURE.md", "OPERATIONS.md", "TESTING.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("priority section missing %q:\n%s", want, got.AdditionalContext)
		}
	}
	if strings.Index(got.AdditionalContext, "Required project docs:") > strings.Index(got.AdditionalContext, "Consider project docs:") {
		t.Fatalf("required docs should render before consider docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsAmbiguousPromptEmphasizesRoute(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이거 좀 개선해줘"})
	if !strings.Contains(got.AdditionalContext, "Route if ambiguous:") || strings.Contains(got.AdditionalContext, "Required project docs:") {
		t.Fatalf("ambiguous prompt should emphasize route without required docs:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsDoesNotTreatPRSubstringAsCommit(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "print output formatting을 확인해줘"})
	if strings.Contains(got.AdditionalContext, "COMMIT_POLICY.md") {
		t.Fatalf("substring pr should not trigger commit policy:\n%s", got.AdditionalContext)
	}
}
```

- [ ] **Step 2: Run targeted tests to confirm failure**

Run:

```bash
go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1
```

Expected: FAIL because current output has flat `Project document candidates` and no `priority` model.

---

### Task 2: Implement priority-aware hint model and routing

**Files:**
- Modify: `internal/core/hook_prompt.go`

- [ ] **Step 1: Add priority field and constants**

Update `HookUserPromptHint`:

```go
type HookUserPromptHint struct {
	Tool     string `json:"tool"`
	Reason   string `json:"reason"`
	Priority string `json:"priority,omitempty"`
}
```

Add constants:

```go
const (
	hintPriorityRequired  = "required"
	hintPriorityConsider  = "consider"
	hintPriorityRoute     = "route"
	hintPriorityAction    = "action"
	hintPrioritySecondary = "secondary"
)
```

- [ ] **Step 2: Replace `add` with `addPriority`**

Use:

```go
addPriority := func(tool, reason, priority string) {
	for _, h := range result.Hints {
		if h.Tool == tool && h.Reason == reason && h.Priority == priority {
			return
		}
	}
	result.Hints = append(result.Hints, HookUserPromptHint{Tool: tool, Reason: reason, Priority: priority})
}
```

Add `add := func(tool, reason string) { addPriority(tool, reason, hintPriorityAction) }` only for existing action-style tool hints if needed.

- [ ] **Step 3: Classify document hints by confidence**

Rules:

- Always add `project_docs_route` as `route`.
- Architecture/design prompts:
  - `ARCHITECTURE.md` as `required`
  - `ADR.md` as `consider`
  - `project_docs_record` as `action`
- Hook/install/daemon/MCP prompts:
  - `OPERATIONS.md` and `CONVENTIONS.md` as `required`
  - `TECH_STACK.md` as `consider`
- Test/verification prompts:
  - `TESTING.md` as `required`
  - `AGENT_WORKFLOW.md` as `consider`
- API prompts:
  - `OPEN_API_SPEC.md` as `required`
  - API tools as `action`
- Commit/push/pull request prompts:
  - `COMMIT_POLICY.md` as `required`
- General docs/conventions prompts:
  - `CONSTITUTION.md` and `CONVENTIONS.md` as `consider`
- Companion tools:
  - `CodeGraph`, `LLM Wiki`, `agentmemory` as `secondary`

- [ ] **Step 4: Reduce noisy substring matches**

Do not match `pr` as a bare substring. Keep `pull request`, `commit`, `push`, `release note`, Korean commit/push tokens. Keep `run` only inside the operations group because it is already balanced by other hook/install/local terms in common prompts; if a test exposes noise later, split phrase matching further.

---

### Task 3: Render priority sections

**Files:**
- Modify: `internal/core/hook_prompt.go`

- [ ] **Step 1: Update grouping**

Group hints by priority:

- `Required project docs:` for `.md` + `required`
- `Consider project docs:` for `.md` + `consider`
- `Route if ambiguous:` for `route`
- `MCP/action candidates:` for `action`
- `Secondary companion-tool hints:` for `secondary`

- [ ] **Step 2: Preserve safety and host output**

Keep the same `HookUserPromptResult` and `hookSpecificOutput.additionalContext` shape. Do not change `cmd/harness/hook_user_prompt.go`.

- [ ] **Step 3: Run targeted tests**

Run:

```bash
go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1
```

Expected: PASS.

---

### Task 4: Verify CLI and full suite

**Files:**
- No new source files.

- [ ] **Step 1: Build CLI**

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
```

Expected: PASS.

- [ ] **Step 2: Smoke test hook output**

Run:

```bash
printf '{"prompt":"hook 구조와 테스트 검증을 설계해줘"}' | ./bin/agent-harness hook user-prompt
```

Expected: output contains `Required project docs:`, `Consider project docs:`, `Route if ambiguous:`, `ARCHITECTURE.md`, `OPERATIONS.md`, and `TESTING.md`.

- [ ] **Step 3: Run full test suite**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

---

### Task 5: Commit

**Files:**
- Modify: `internal/core/hook_prompt.go`
- Modify: `internal/core/hook_prompt_test.go`
- Create: `docs/superpowers/plans/2026-05-29-priority-project-doc-routing-hook.md`

- [ ] **Step 1: Review diff**

Run:

```bash
git diff -- internal/core/hook_prompt.go internal/core/hook_prompt_test.go docs/superpowers/plans/2026-05-29-priority-project-doc-routing-hook.md
```

Expected: diff only implements priority routing and tests; no companion-tool execution.

- [ ] **Step 2: Commit with Lore body**

Run:

```bash
git add internal/core/hook_prompt.go internal/core/hook_prompt_test.go docs/superpowers/plans/2026-05-29-priority-project-doc-routing-hook.md
git commit -m "feat(hook): rank project-doc routing hints" -m "Make prompt-hook document guidance distinguish required reads from optional considerations and route-only ambiguity handling.\n\nConstraint: Hook routing must remain deterministic static analysis without MCP, network, or companion-tool execution.\nRejected: Flat document candidate lists | They blur mandatory project rules with optional context and encourage over-reading.\nConfidence: high\nScope-risk: narrow\nDirective: Add future document mappings with an explicit priority tier and a regression test.\nTested: go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1; go build -o bin/agent-harness ./cmd/harness; hook JSON smoke; go test ./... -count=1\nNot-tested: Live host hook invocation in Codex and Claude Code."
```

Expected: commit succeeds.

---

## Self-Review

- Spec coverage: The plan extends the approved document-routing hook into priority tiers while keeping external tools secondary.
- Placeholder scan: No placeholders or vague steps remain.
- Type consistency: `Priority` is backward-compatible JSON because it is `omitempty`, and existing `Tool`/`Reason` tests remain valid.
