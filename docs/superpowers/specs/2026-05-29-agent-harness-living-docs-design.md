# Agent Harness Living Project Docs Design

Date: 2026-05-29
Status: approved for implementation planning

## Purpose

`.agent-harness/*.md` should be living project knowledge, not stale bootstrap output. The harness should notice user decisions, agent-visible workflow outcomes, code changes, and verification signals, then help agents inject or update the right project documents at the right time.

The design must support team repositories. Per-user and per-session runtime state must not be committed to the target repository.

## Non-goals

- Do not put lifecycle runtime state in `.agent-harness/` by default.
- Do not put lifecycle schemas in the target repository.
- Do not let hooks silently rewrite shared docs.
- Do not replace specialized tools such as CodeGraph, LLM Wiki, or agentmemory.
- Do not make missing optional companion tools a hard failure.

## Key decision

Agent-harness owns lifecycle state schema, validation, and migration logic in its Go core. Target repositories contain only shared knowledge documents under `.agent-harness/`.

Default runtime state lives outside the repository and is strictly project-scoped:

```text
~/.local/state/agent-harness/projects/<repo-id>/
```

`<repo-id>` must be derived from stable local repo evidence such as canonical root path, git repository identity, and git origin metadata when available, then hashed so multiple repositories on the same machine do not share lifecycle state. The state directory must contain a small `project.json` fingerprint with the resolved root and git metadata used to create the id. Every hook invocation should re-check that fingerprint before trusting existing state. If the current repo does not match the stored fingerprint, the hook should ignore that state and `agent-harness doctor` should report a namespace mismatch rather than merging data.

There should be no single global lifecycle queue shared by all repositories. A global index may list known project ids for pruning or diagnostics, but project routing, consensus, upkeep, and hook-health data belong under one project namespace.

## Repository boundary

Allowed in target repos:

```text
.agent-harness/ARCHITECTURE.md
.agent-harness/CAUTIONS.md
.agent-harness/COMMIT_POLICY.md
.agent-harness/CONSTITUTION.md
.agent-harness/CONVENTIONS.md
.agent-harness/TECH_STACK.md
.agent-harness/TESTING.md
.agent-harness/OPEN_API_SPEC.md
.agent-harness/ADR.md
.agent-harness/OPERATIONS.md
.agent-harness/AGENT_WORKFLOW.md
```

Not created by default:

```text
.agent-harness/state/
.agent-harness/STATE.md
.agent-harness/state.schema.json
```

If repo-local runtime or schema-looking files are found, `agent-harness doctor` should warn because team member state can pollute shared history. `STATE.md` should not be universally forbidden; warn only when it appears to be runtime/schema plumbing rather than intentional project documentation.

## Lifecycle state model

Agent-harness core should define versioned DTOs for project lifecycle state. Initial persisted state files can be implementation-specific, but the conceptual model is:

### Project profile

Tracks stable repo facts used for routing, namespace validation, and diagnostics.

Fields:
- repo id
- repo root
- canonical root path used for id derivation
- git directory or worktree identity when available
- git origin fingerprint when available
- detected languages and package managers
- detected build, test, and lint commands
- known `.agent-harness` docs present in the repo
- last namespace validation result

### Doc routing state

Tracks recent document-routing behavior so prompt injection can be state-aware instead of purely keyword-based.

Fields:
- recently selected docs
- last injected docs
- prompt categories that led to each selection
- small cache metadata to avoid noisy repeated injection

### Doc upkeep queue

Append-only JSONL-style queue of possible documentation updates.

Event kinds:
- `consensus`: user made an explicit durable decision
- `code_change`: changed files may affect docs
- `verification`: test, build, lint, or quality gate changed the docs contract
- `resolved_failure`: an avoidable or repeated failure should become a caution
- `operation_change`: install, hook, MCP, daemon, state, or doctor behavior changed

Each event should include:
- event id
- timestamp
- repo id
- target doc candidates
- summary
- evidence references
- source hook or command
- status: pending, applied, dismissed, superseded

### Consensus events

A focused log for user decisions that may deserve ADR or convention updates.

Example:

```json
{
  "kind": "consensus",
  "target_docs": ["ADR.md", "CONVENTIONS.md"],
  "summary": "Team/user runtime state must not be committed to target repositories.",
  "source": "user_prompt"
}
```

### Namespace safety

Hooks and doctor must prevent cross-repo state bleed on the same machine.

Rules:
- Resolve the current repo root before reading lifecycle state.
- Read only from that repo's hashed project namespace.
- Validate the namespace fingerprint in `project.json` before trusting queued events.
- If the fingerprint mismatches, do not merge or migrate automatically; mark the namespace unhealthy and ask doctor to propose a safe fix.
- Use file locks or atomic rename for queue writes so concurrent sessions in the same repo do not corrupt JSONL state.
- Support an environment override for tests and advanced users, but doctor should show the active state root clearly.

### Hook health

Tracks hook execution health for diagnostics.

Fields:
- last successful hook run per event
- last failure per event
- timeouts
- disabled or missing hook indicators
- schema migration status

## Bootstrap behavior

`agent-harness project bootstrap` should be the normal place to initialize the project-scoped lifecycle namespace, but it must initialize it in user-state, not in the target repo.

Behavior:

- Dry-run bootstrap reports the planned user-state project namespace and fingerprint, but does not create runtime state.
- Write bootstrap creates or validates the user-state project namespace and writes `project.json` with the repo fingerprint.
- Bootstrap still writes target repo `.agent-harness/*.md` only when `--write` is explicitly set, preserving existing dry-run semantics.
- A future narrower flag such as `project bootstrap --init-state` may initialize only user-state for repos that already have project docs.
- If `project.json` already exists and its fingerprint mismatches the current repo, bootstrap must not merge state. It should return a warning and let `agent-harness doctor` propose migration or cleanup.

This makes bootstrap the first durable handshake between a target repo and the harness lifecycle system while keeping team repositories free of per-user state.

## Hook behavior

### UserPromptSubmit

Current behavior is keyword-only routing. It should become state-aware:

1. Parse the prompt.
2. Resolve the current repo id and lifecycle state.
3. Inspect pending doc-upkeep and consensus events.
4. Select only the relevant `.agent-harness` docs.
5. Inject concise additional context with required docs, optional docs, and action candidates.
6. Record routing metadata in user state.

It should stay fast and local. No network access and no long file reads in the hook path.

### PostToolUse

Records potential upkeep events when relevant files or commands change the documentation contract.

Examples:
- hook code changed -> `OPERATIONS.md`, `CONVENTIONS.md`
- MCP tool schema changed -> `OPERATIONS.md`, `ARCHITECTURE.md`, contract tests
- test/golden behavior changed -> `TESTING.md`
- `.agent-harness` doc changed -> mark related queue entries applied or superseded

PostToolUse should not rewrite docs directly.

### Stop

At turn end, inspect pending upkeep. If pending items are important, inject a concise reminder to the agent. It may recommend:

- `project_docs_append(kind=adr)` for decisions and rejected alternatives
- `project_docs_append(kind=caution)` for resolved failures and repeated mistakes
- `project_docs_read` then `project_docs_revise` for full document edits

Stop should not directly modify shared docs.

### PreCompact/PostCompact

Preserve pending lifecycle state across compaction. This avoids losing recent decisions or doc-upkeep candidates in long sessions.

## Document promotion rules

Lifecycle state is not documentation by itself. Promote only when the event is durable, reusable, and evidence-backed.

### ADR.md

Use for:
- structural decisions
- long-lived trade-offs
- rejected alternatives that future agents should not re-litigate

### CONVENTIONS.md

Use for:
- repeated implementation rules
- adapter boundaries
- package and hook conventions

### OPERATIONS.md

Use for:
- install and update behavior
- hook registration
- daemon and MCP operations
- doctor usage
- local run behavior

### TESTING.md

Use for:
- new verification commands
- golden and contract test expectations
- regression gates

### CAUTIONS.md

Use for:
- resolved false cases
- repeated failures
- operational warnings future agents should avoid

## `agent-harness doctor`

Add a top-level command:

```bash
agent-harness doctor
agent-harness doctor --json
```

This is the user-facing comprehensive diagnostic command. Existing `agent-harness state doctor` may remain as a narrower internal state-store integrity check, but docs should prefer top-level `doctor` for general troubleshooting.

### Diagnostic scope

`agent-harness doctor` should check:

1. binary version and build availability
2. user-level command shim
3. Codex native skill links
4. Claude native skill links
5. Codex hook registration
6. Claude MCP and user-scope registration where available
7. `agent_harness` MCP server registration
8. daemon status
9. user-state root existence, permissions, schema, and migration status
10. project lifecycle state existence, namespace fingerprint, and schema compatibility
11. cross-repo namespace mismatch or suspicious shared state
12. `.agent-harness` docs presence and obvious missing-doc conditions
13. pending doc-upkeep summary
14. forbidden or suspicious repo-local runtime/schema state
15. optional companion tools such as CodeGraph, LLM Wiki, and agentmemory when detectable

### Output shape

Human output should be concise and action-oriented. JSON output should include machine-readable issues and safe fix suggestions:

```json
{
  "ok": true,
  "healthy": false,
  "issues": [
    {
      "code": "repo_local_state_present",
      "severity": "warning",
      "summary": ".agent-harness/state contains runtime state in a team repo.",
      "fix": {
        "command": "agent-harness doctor --fix",
        "destructive": false,
        "description": "Migrate runtime state to the user state dir or add an ignore rule."
      }
    }
  ]
}
```

Default `doctor` is read-only. Any fixing behavior must be explicit, conservative, and separately gated with `--fix` or narrower future flags.

## Error handling and safety

- If lifecycle state cannot be read, hooks should degrade to current keyword-only routing.
- If lifecycle schema is unsupported, doctor should report migration guidance.
- If user-state is unwritable, hooks should still return useful routing context when possible.
- If multiple repos map to the same id, include enough fingerprint evidence to detect and warn, and do not reuse the mismatched state for routing.
- If repo identity changes because a directory was moved, doctor may propose a safe namespace migration only when the old and new fingerprints are explainably related.
- Shared docs require evidence-preserving updates through `project_docs_read` plus `project_docs_revise`, or append-only `project_docs_append` for ADR/caution entries.

## Testing strategy

Implementation should add or update tests for:

- repo-id resolution is stable and collision-resistant enough for multiple local repos
- lifecycle state read/write, namespace fingerprint validation, and schema version validation
- hook fallback when state is missing or corrupt
- UserPromptSubmit uses pending lifecycle state to add doc/action candidates
- PostToolUse records upkeep candidates for relevant paths
- Stop produces reminders for pending upkeep without direct writes
- top-level `agent-harness doctor --json` reports comprehensive issue records
- doctor detects repo-local runtime/schema artifacts
- CLI usage and MCP/contract golden outputs where command surfaces change

## Implementation sequence

1. Add lifecycle state DTOs and resolver in `internal/core`.
2. Add lifecycle state store helpers using the existing user-state conventions.
3. Extend `project bootstrap` dry-run/write results to plan and initialize the project-scoped user-state namespace.
4. Extend `hook user-prompt` to read lifecycle state and route docs statefully.
5. Add hook subcommands or shared handlers for PostToolUse and Stop lifecycle observations.
6. Add top-level `agent-harness doctor` with read-only diagnostics and JSON output.
7. Update generated usage, response contracts, docs, and golden tests.
8. Update `.agent-harness/OPERATIONS.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, and `TESTING.md` only where implementation evidence requires it.

## Open implementation details

These are deliberately left to implementation planning rather than product design:

- exact repo-id hash inputs and migration behavior
- exact JSON file layout under the user-state project directory
- whether doctor exposes `--fix` in the first implementation or only reports fix commands
- whether lifecycle state is also exposed through MCP resources
- whether the first implementation needs a separate `project bootstrap --init-state` flag or can rely on `--write` bootstrap
