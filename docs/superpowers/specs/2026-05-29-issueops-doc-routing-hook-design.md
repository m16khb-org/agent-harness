# Agent Harness Document-Routing Hook Design

## Summary

`issueops hook user-prompt` should make `.issueops` project documents reliably visible at the moment an agent receives a task. Its primary job is not to run external companion tools. Its primary job is to inject a short, deterministic routing contract that tells the agent which repo-local operating documents are likely required and why.

External tools such as agentmemory, LLM Wiki, and CodeGraph remain useful, but they are secondary. They should be suggested only when the prompt clearly benefits from them. The hook must not reimplement or auto-run their core behavior.

## Goals

- Ensure `.issueops` documents are considered before work starts when the task implies project rules, architecture, operations, tests, API docs, commits, or durable decisions.
- Keep hook output short enough to inject on every prompt without overwhelming the agent.
- Preserve the existing safety model: the hook performs static prompt analysis only, does not execute work, does not read large files, and does not use the network.
- Maintain host-neutral behavior across Codex and Claude Code by keeping routing logic in the shared Go core behind `issueops hook user-prompt`.
- Make external companion tools optional hints, not the center of the design.

## Non-goals

- Do not inject full `.issueops` document contents on every prompt.
- Do not auto-call MCP tools from the hook.
- Do not query agentmemory, LLM Wiki, or CodeGraph from the hook.
- Do not duplicate upstream tool functionality inside issueops.
- Do not block user work if routing cannot decide a perfect document set.

## Architecture

The hook should be a deterministic document router with three layers:

1. **Always-on routing contract**
   - Inject a brief reminder that `.issueops` documents are the project operating source of truth.
   - Tell the agent to route/read/update/record only when the current task needs repo evidence, policy, validation, or durable records.
   - Preserve the existing instruction that MCP is not required for simple reasoning.

2. **Signal-to-document hints**
   - Map prompt signals to likely `must_read` documents and concise reasons.
   - Examples:
     - architecture, design, alternative, structural decision → `ARCHITECTURE.md`, `ADR.md`
     - hook, install, local run, daemon, MCP, operations → `OPERATIONS.md`, `CONVENTIONS.md`, `TECH_STACK.md`
     - test, verify, CI, golden, race, QA → `TESTING.md`, `AGENT_WORKFLOW.md`
     - recurring failure, false case, caution → `CAUTIONS.md`
     - endpoint, DTO, Swagger, OpenAPI, API docs → `OPEN_API_SPEC.md`
     - commit, push, PR, release note → `COMMIT_POLICY.md`
     - project rules, AGENTS, bootstrap, conventions → `CONSTITUTION.md`, `CONVENTIONS.md`

3. **Secondary companion-tool hints**
   - Suggest CodeGraph only for repo-local symbol, call graph, impact, or trace questions.
   - Suggest LLM Wiki only for explicit wiki/research/knowledge-base workflows.
   - Suggest agentmemory only for previous-session memory, repeated work, or “did we already solve this?” style prompts.
   - These hints should never outrank `.issueops` routing.

## Data Flow

1. Host calls `issueops hook user-prompt` on `UserPromptSubmit`.
2. CLI parses the prompt from flags, args, or hook stdin JSON.
3. Core prompt analyzer builds a `HookUserPromptResult` with:
   - `should_inject`
   - `additional_context`
   - structured hint entries
4. Renderer emits host-compatible `hookSpecificOutput.additionalContext`.
5. The agent sees a short routing contract and decides whether to use project docs MCP, read files directly, or proceed without extra tools.

## Output Shape

The injected context should stay English and concise. It should include:

- A title such as `issueops project-doc routing hint:`.
- A mandatory routing sentence: decide whether `.issueops` docs are necessary before acting.
- Zero or more document hints formatted as `- DOC: reason`.
- Optional MCP tool hints only when useful, such as `project_docs_route`, `project_docs_read`, `project_docs_revise`, or `project_docs_append`.
- A safety footer: writable tools must preserve user consensus and current file evidence.

The output should not paste entire documents, command output, or upstream tool results.

## Error Handling and Safety

- Empty prompt returns `should_inject=false`.
- Unknown prompt returns either no injection or only the minimal always-on routing contract, depending on implementation preference after tests confirm noise level.
- JSON parsing failures should fall back to treating stdin as raw prompt text.
- Hook failures should not block the user task.
- The hook must remain fast and deterministic; no network, no long file scans, no daemon dependency, and no companion-tool invocation.

## Testing

Add or update tests around `BuildUserPromptMCPHints` or its successor to prove:

- Architecture/design prompts include `ARCHITECTURE.md` and `ADR.md`.
- Hook/install/operation prompts include `OPERATIONS.md` and relevant convention/tech documents.
- Test/verification prompts include `TESTING.md` and `AGENT_WORKFLOW.md`.
- API prompts still include `OPEN_API_SPEC.md` and API doc MCP hints.
- Companion tool prompts produce secondary hints without replacing document routing.
- Injected context remains English-only.
- Empty prompts do not inject.

Verification commands should include targeted Go tests for hook routing and the existing broader Go test suite when code changes follow this spec.

## Trade-offs

This design favors reliable project-rule recall over maximal automation. It may require the agent to make one extra explicit read or MCP call, but it avoids noisy context injection, host-specific behavior, and accidental dependency on external plugin availability.

## Implementation Boundary

The first implementation should stay surgical:

- Modify only the hook prompt analysis/rendering path and related tests unless evidence requires otherwise.
- Keep existing API doc routing behavior intact.
- Do not change install/bootstrap behavior except if hook output naming or docs require template updates.
- Do not introduce new dependencies.

## Acceptance Criteria

- The hook design makes `.issueops` document routing the first-class responsibility.
- External companion tools are documented as secondary hints only.
- Safety constraints remain explicit and testable.
- A future implementation plan can be derived without additional architectural decisions.
