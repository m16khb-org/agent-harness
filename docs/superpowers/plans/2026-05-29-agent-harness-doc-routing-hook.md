# Agent Harness Document-Routing Hook Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `agent-harness hook user-prompt` prioritize concise `.agent-harness` document routing hints while keeping external companion tools secondary.

**Architecture:** Extend the existing shared Go hook analyzer in `internal/core/hook_prompt.go` rather than adding host-specific logic. Tests in `internal/core/hook_prompt_test.go` lock the document-first routing behavior and preserve existing API-doc behavior. The CLI wrapper in `cmd/harness/hook_user_prompt.go` remains unchanged because it already delegates to the shared core and emits host-compatible hook JSON.

**Tech Stack:** Go standard library, existing `go test` suite, existing `agent-harness` CLI.

---

## File Structure

- Modify `internal/core/hook_prompt.go`: add document-first routing hints, render them with a project-doc title, and keep companion-tool hints secondary.
- Modify `internal/core/hook_prompt_test.go`: add behavior tests for document routing, companion secondary routing, empty prompt behavior, and existing English-only output.
- No changes to install/bootstrap templates are needed because command shape and hook registration stay unchanged.

---

### Task 1: Add failing tests for document-first hook routing

**Files:**
- Modify: `internal/core/hook_prompt_test.go`

- [ ] **Step 1: Add tests for architecture, operations, testing, and companion-tool prompts**

Append these tests to `internal/core/hook_prompt_test.go`:

```go
func TestBuildUserPromptMCPHintsRoutesArchitectureDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "hook 구조와 대안을 설계해줘"})
	for _, want := range []string{"agent_harness project-doc routing hint", "ARCHITECTURE.md", "ADR.md", "project_docs_route"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("architecture doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesOperationsDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "install hook와 daemon 운영 경로를 고쳐줘"})
	for _, want := range []string{"OPERATIONS.md", "CONVENTIONS.md", "TECH_STACK.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("operations doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsRoutesTestingDocs(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "golden test와 verification을 추가해줘"})
	for _, want := range []string{"TESTING.md", "AGENT_WORKFLOW.md"} {
		if !strings.Contains(got.AdditionalContext, want) {
			t.Fatalf("testing doc hint missing %q:\n%s", want, got.AdditionalContext)
		}
	}
}

func TestBuildUserPromptMCPHintsCompanionToolsStaySecondary(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "이 symbol의 call graph와 impact를 codegraph로 확인해줘"})
	projectIndex := strings.Index(got.AdditionalContext, "agent_harness project-doc routing hint")
	companionIndex := strings.Index(got.AdditionalContext, "Secondary companion-tool hints")
	if projectIndex < 0 || companionIndex < 0 || projectIndex > companionIndex {
		t.Fatalf("expected project-doc routing before companion hints:\n%s", got.AdditionalContext)
	}
	if !strings.Contains(got.AdditionalContext, "CodeGraph") {
		t.Fatalf("expected CodeGraph secondary hint:\n%s", got.AdditionalContext)
	}
}

func TestBuildUserPromptMCPHintsEmptyPromptDoesNotInject(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "   "})
	if got.ShouldInject || got.AdditionalContext != "" || len(got.Hints) != 0 {
		t.Fatalf("expected no injection for empty prompt: %+v", got)
	}
}
```

- [ ] **Step 2: Run targeted test and verify it fails before implementation**

Run:

```bash
go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1
```

Expected: FAIL because the current hook renderer still uses `agent_harness MCP routing hint` and does not emit the new document-specific or companion-tool sections.

---

### Task 2: Implement document-first routing in the hook analyzer

**Files:**
- Modify: `internal/core/hook_prompt.go`

- [ ] **Step 1: Replace the hint builder with document-first categories**

Update `BuildUserPromptMCPHints` so it:

- Keeps `project_docs_route` as the baseline hint for non-empty prompts.
- Adds document hint entries where `Tool` is a document filename and `Reason` explains when to read it.
- Keeps API doc MCP hints for endpoint/OpenAPI prompts.
- Adds companion hints only after project-doc hints.

Concrete behavior to implement in `internal/core/hook_prompt.go`:

```go
add("project_docs_route", "Before acting, decide which AGENTS.md/.agent-harness documents are required for this task.")

if containsAny(lower, "architecture", "architect", "refactor", "design", "decision", "alternative", "structure") || containsAny(prompt, "아키텍처", "리팩터", "결정", "대안", "구조", "설계") {
	add("ARCHITECTURE.md", "Read when architecture, boundaries, host-neutral design, or component responsibilities shape the work.")
	add("ADR.md", "Read or update when a structural decision, trade-off, or rejected alternative matters long-term.")
	add("project_docs_record", "When a structural decision or rejected alternative matters long-term, consider kind=adr for ADR.md.")
}

if containsAny(lower, "hook", "install", "bootstrap", "update", "daemon", "mcp", "operation", "local", "run") || containsAny(prompt, "훅", "설치", "부트스트랩", "데몬", "운영", "로컬 실행") {
	add("OPERATIONS.md", "Read for install, update, hook registration, daemon, MCP, and local-run behavior.")
	add("CONVENTIONS.md", "Read for package boundaries, adapter conventions, and hook implementation rules.")
	add("TECH_STACK.md", "Read when toolchain, companion tools, or runtime choices affect the work.")
}

if containsAny(lower, "test", "spec", "verify", "verification", "lint", "typecheck", "ci", "golden", "race", "qa") || containsAny(prompt, "테스트", "검증", "골든") {
	add("TESTING.md", "Read for verification commands, gates, and expected test scope.")
	add("AGENT_WORKFLOW.md", "Read for agent workflow, routing, and validation expectations.")
}
```

Also add smaller mappings for `CAUTIONS.md`, `OPEN_API_SPEC.md`, `COMMIT_POLICY.md`, and `CONSTITUTION.md` using the spec text as the source of truth.

- [ ] **Step 2: Add companion-tool secondary hints**

Add hints only for explicit prompt signals:

```go
if containsAny(lower, "codegraph", "symbol", "call graph", "impact", "trace", "caller", "callee") {
	add("CodeGraph", "Secondary hint: consider CodeGraph for repo-local symbol, call graph, impact, or trace questions.")
}
if containsAny(lower, "llm-wiki", "wiki", "knowledge base", "research", "compile") {
	add("LLM Wiki", "Secondary hint: consider upstream LLM Wiki for explicit wiki, research, knowledge-base, query, or compile workflows.")
}
if containsAny(lower, "claude-mem", "memory", "previous session", "last time", "already solve", "already solved") || containsAny(prompt, "전에", "지난번", "이미 해결") {
	add("claude-mem", "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.")
}
```

- [ ] **Step 3: Update renderer to group project-doc hints before companion hints**

Change `renderHookMCPHintContext` to:

- Start with `agent_harness project-doc routing hint:`.
- Print the baseline instruction.
- Print `Project document candidates:` for hints ending in `.md`.
- Print `MCP/action candidates:` for `project_docs_*` and API review tools.
- Print `Secondary companion-tool hints:` for `CodeGraph`, `LLM Wiki`, and `claude-mem`.
- End with the existing writable-tool safety footer.

Do not change public JSON field names.

- [ ] **Step 4: Run targeted test and verify it passes**

Run:

```bash
go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1
```

Expected: PASS.

---

### Task 3: Verify CLI hook output and full Go suite

**Files:**
- No source changes expected unless verification finds a regression.

- [ ] **Step 1: Build the CLI**

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
```

Expected: command exits 0.

- [ ] **Step 2: Smoke test hook JSON output**

Run:

```bash
printf '{"prompt":"hook 구조와 테스트 검증을 설계해줘"}' | ./bin/agent-harness hook user-prompt
```

Expected: JSON contains `hookSpecificOutput`, `agent_harness project-doc routing hint`, `ARCHITECTURE.md`, `ADR.md`, `TESTING.md`, and `AGENT_WORKFLOW.md`.

- [ ] **Step 3: Run standard Go tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Check worktree**

Run:

```bash
git status --short
```

Expected: only intentional edits to `internal/core/hook_prompt.go`, `internal/core/hook_prompt_test.go`, and this plan file unless code generation or golden fixtures are proven necessary.

---

### Task 4: Commit implementation

**Files:**
- Modify: `internal/core/hook_prompt.go`
- Modify: `internal/core/hook_prompt_test.go`
- Create: `docs/superpowers/plans/2026-05-29-agent-harness-doc-routing-hook.md`

- [ ] **Step 1: Review diff**

Run:

```bash
git diff -- internal/core/hook_prompt.go internal/core/hook_prompt_test.go docs/superpowers/plans/2026-05-29-agent-harness-doc-routing-hook.md
```

Expected: diff matches this plan and does not add new dependencies or auto-execution behavior.

- [ ] **Step 2: Commit with Lore body**

Run:

```bash
git add internal/core/hook_prompt.go internal/core/hook_prompt_test.go docs/superpowers/plans/2026-05-29-agent-harness-doc-routing-hook.md
git commit -m "feat(hook): prioritize project-doc routing hints" -m "Make the prompt hook steer agents toward .agent-harness source-of-truth documents before optional companion tools.\n\nConstraint: Hook execution must stay deterministic, short, host-neutral, and free of network or companion-tool calls.\nRejected: Auto-querying claude-mem, LLM Wiki, or CodeGraph from UserPromptSubmit | It would duplicate upstream behavior and make every prompt slower and noisier.\nConfidence: high\nScope-risk: narrow\nDirective: Keep future hook additions as concise routing hints unless a separate ADR approves safe automation.\nTested: go test ./internal/core -run 'TestBuildUserPromptMCPHints' -count=1; go build -o bin/agent-harness ./cmd/harness; hook JSON smoke; go test ./... -count=1\nNot-tested: Cross-host live Codex/Claude hook invocation."
```

Expected: commit succeeds.

---

## Self-Review

- Spec coverage: The plan implements document-first routing, keeps companion tools secondary, preserves no-network/no-execution hook constraints, and includes tests for prompt categories from the design spec.
- Placeholder scan: No placeholders remain; all commands and expected outcomes are concrete.
- Type consistency: The plan uses existing `HookUserPromptRequest`, `HookUserPromptResult`, and `HookUserPromptHint` types and does not introduce incompatible JSON fields.
