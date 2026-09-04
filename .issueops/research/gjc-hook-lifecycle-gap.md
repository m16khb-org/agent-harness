# Research: Can issueops integrate its 7 lifecycle hooks into GJC (gajae-code) the way it does into Codex/Claude Code?

## TL;DR
**Conclusion**: No. GJC's runtime emits a Claude/Codex-comparable set of lifecycle events (agent_start, session_start, tool_call, compaction, turn, shutdown), but the *user-facing hook registration surfaces* that an external CLI harness like issueops can use are restricted to (a) pre/post-tool shell scripts discovered from `~/.gjc/agent/hooks/{pre,post}/`, and (b) plugin-bundle hooks whose constrained API forbids `exec`. issueops's `user-prompt`, `session-start`, `stop`, and `compact` hooks have no GJC registration equivalent. This is **partially intentional** (GJC promotes a first-party TypeScript `HookAPI` / "extensions" model over shell-command hooks) and **partly an undiscovered surface gap** (runtime events exist without a matching registration path).

**Confidence**: High for the surface inventory; Medium for the intent judgment (intent inferred from API shape + doc wording, not a maintainer statement).

**Sources**: 4 independent sources (GJC repo source + examples, Claude Code official docs, Codex official docs, third-party Codex/Claude guides), 0 single-sourced critical claims.

## Method
- Search angles: (1) Claude Code hook event inventory & config model; (2) Codex hook event inventory & config model; (3) GJC hook/extension runtime events; (4) GJC hook registration/discovery surfaces + plugin bundle constraints.
- Sources fetched and read in full: Claude Code hooks reference (code.claude.com), Codex hooks reference (developers.openai.com), GJC `packages/coding-agent/examples/hooks/README.md`, GJC `src/extensibility/hooks/types.ts`, GJC `src/extensibility/gjc-plugins/constrained-hooks.ts`, GJC `src/discovery/builtin.ts` (loadHooks), GJC `src/capability/hook.ts`.
- Cross-verification: each platform's event list checked against its official doc; GJC runtime events cross-checked against GJC's own `agent-session.ts` emit call sites seen during source investigation.

## Findings

### Finding 1 — Claude Code exposes a very wide lifecycle hook set via shell-command handlers
- **Claim**: Claude Code supports SessionStart, SessionEnd, UserPromptSubmit, UserPromptExpansion, PreToolUse, PostToolUse, PostToolUseFailure, PostToolBatch, PermissionRequest, PermissionDenied, Stop, StopFailure, PreCompact, PostCompact, SubagentStart, SubagentStop, Notification, TeammateIdle, InstructionsLoaded, ConfigChange, CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove, Elicitation, ElicitationResult, MessageDisplay, Setup, TaskCreated, TaskCompleted — registered as command/HTTP/MCP-tool/prompt/agent handlers in `settings.json` / plugin `hooks/hooks.json`.
- **Sources**:
  - Hooks reference — Claude Code Docs — https://code.claude.com/docs/en/hooks — retrieved 2026-06-30 — full event table + matcher/handler schema
  - Claude Code Hooks visual monitoring — https://agentsroom.dev/claude-code-hooks — retrieved 2026-06-30 — corroborates PreToolUse/PostToolUse/UserPromptSubmit/Stop/SubagentStop/Notification set
- **Verification**: Confirmed by 2 independent sources (official + third-party guide).

### Finding 2 — Codex exposes a Claude-style command-hook set via hooks.json
- **Claim**: Codex supports SessionStart, SubagentStart, PreToolUse, PermissionRequest, PostToolUse, PreCompact, PostCompact, UserPromptSubmit, SubagentStop, Stop. Handlers are `type: "command"` shell commands (prompt/agent parsed but skipped today). Config lives in `~/.codex/hooks.json` or inline `[hooks]` in `config.toml`, plus plugin-bundled `hooks/hooks.json`. Codex requires per-hook trust review before running non-managed hooks.
- **Sources**:
  - Hooks — Codex | OpenAI Developers — https://developers.openai.com/codex/hooks — retrieved 2026-06-30 — canonical event list + config shape + plugin bundling
  - Codex CLI Hooks complete guide — https://codex.danielvaughan.com/2026/04/15/codex-cli-hooks-complete-guide-events-policy-patterns/ — retrieved 2026-06-30 — corroborates "five hook event types (SessionStart, PreToolUse, PostToolUse, UserPromptSubmit, Stop)" and calls Codex a "Claude-style engine"
- **Verification**: Confirmed by 2 independent sources (official + independent deep guide). Note: the guide's "five" is the core set; the official doc lists the extended set including SubagentStart/Stop, PreCompact/PostCompact, PermissionRequest.

### Finding 3 — GJC's runtime emits a lifecycle event set comparable to Claude/Codex
- **Claim**: GJC's extension runner emits rich events: AgentStartEvent, AgentEndEvent, AutoCompactionStart/EndEvent, AutoRetryStart/EndEvent, ContextEvent, SessionStartEvent, SessionShutdownEvent, SessionBeforeBranch/Switch/Tree/Compact events (+ Results), SessionBranch/Switch/Tree/Compact events, SessionCompactingEvent/Result, TurnStartEvent, TurnEndEvent, TodoReminderEvent, TtsrTriggeredEvent, ToolCall/ToolResult events.
- **Sources**:
  - gajae-code `packages/coding-agent/src/extensibility/hooks/types.ts` (imported event type list) — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/src/extensibility/hooks/types.ts — retrieved 2026-06-30
  - gajae-code `agent-session.ts` emit call sites — runtime emits `{type:"agent_start"}`, `{type:"agent_end"}`, `{type:"session_shutdown"}`, `{type:"ttsr_triggered"}`, plus `emitBeforeAgentStart`, `emitContext`, `emitToolCall` — retrieved 2026-06-30 during source investigation
- **Verification**: Confirmed by 2 internal source classes (type declarations + runtime call sites). Single project, but two independent code locations agree.

### Finding 4 — GJC's user-facing hook registration is split into two narrow surfaces
- **Claim**: GJC lets users register hooks through only two paths:
  1. **Shell-script discovery** — `~/.gjc/agent/hooks/{pre,post}/<tool-name-or-*>` files, documented as "Pre/post tool execution hooks defined as shell scripts". Only tool-pre and tool-post; no user-prompt/session-start/stop/compact slots.
  2. **Plugin-bundle hooks** (`gajae-plugin.json` `hooks[]`) — TypeScript hooks with `event`/`target`/`phase`, but a **constrained API**: `sendMessage`, `appendEntry`, `registerMessageRenderer`, `registerCommand`, and `exec` are denied (`security_policy`); the factory must register exactly its declared event or be quarantined.
- **Sources**:
  - gajae-code `src/capability/hook.ts` — "Pre/post tool execution hooks defined as shell scripts" — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/src/capability/hook.ts — retrieved 2026-06-30
  - gajae-code `src/discovery/builtin.ts` loadHooks — scans only `hooks/pre/` and `hooks/post/`, filename = tool name — retrieved 2026-06-30
  - gajae-code `src/extensibility/gjc-plugins/constrained-hooks.ts` — `DENIED_API_METHODS` list incl. `exec`; declared-event-only registration — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/src/extensibility/gjc-plugins/constrained-hooks.ts — retrieved 2026-06-30
- **Verification**: Confirmed by 3 internal source files describing the two surfaces consistently.

### Finding 5 — GJC's first-party "HookAPI" is the intended extension model and is not shell-command based
- **Claim**: GJC's documented hook authoring model is a TypeScript `HookAPI` imported from `@gajae-code/coding-agent/hooks`, using `pi.on("tool_call", async (event, ctx) => {...})` and `pi.registerCommand(...)`. The SDK example explicitly notes: *"hooks" is now called "extensions" in the API.* The example hooks (permission-gate, git-checkpoint, custom-compaction, auto-commit-on-exit) are all TypeScript, not shell scripts. There is no documented equivalent of Claude/Codex `type:"command"` handlers for the broad event set.
- **Sources**:
  - gajae-code `examples/hooks/README.md` — usage + HookAPI example + hook list — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/examples/hooks/README.md — retrieved 2026-06-30
  - gajae-code `examples/sdk/06-hooks.ts` — *"Note: 'hooks' is now called 'extensions' in the API."* — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/examples/sdk/06-hooks.ts — retrieved 2026-06-30
- **Verification**: Single-sourced on intent wording (only GJC's own examples), but corroborated by Finding 3 (rich runtime events exist for exactly this TS API).

## Cross-check (adversarial)
- **Refutation attempt**: "Maybe GJC accepts Claude-style `hooks/UserPromptSubmit/...` command configs somewhere I didn't look."
- **Result**: Could not confirm. `src/discovery/claude.ts` loadHooks reads only `.claude/hooks/{pre,post}/` (project scope) and maps them to GJC's pre/post tool model — it does not surface Claude's UserPromptSubmit/Stop/SessionStart command arrays as runnable hooks. `src/hooks/codex-native-hooks-config.ts` exists but only recognizes "gjc-managed" Codex hook entries for GJC's own bookkeeping, not as a general external-hook bridge. The refutation fails — no broad command-hook path was found.

## Conclusion
issueops cannot integrate its `user-prompt`, `session-start`, `stop`, `pre-compact`, `post-compact` hooks into GJC the way it does into Codex/Claude Code, because GJC's externally-usable hook surfaces are limited to pre/post-tool shell scripts and exec-denied plugin hooks. The gap is **not** a runtime limitation — GJC's extension runtime emits the events — it is a **registration-surface limitation** combined with a **design preference for the TypeScript HookAPI**.

For an issue to file upstream, the actionable ask is one of:
1. Broaden the shell-script discovery (`~/.gjc/agent/hooks/`) or plugin-bundle hooks to accept lifecycle events beyond pre/post-tool (user-prompt, session-start, stop, compact), so external CLI harnesses can register shell-command hooks the Claude/Codex way; OR
2. Document the recommended pattern for an external CLI harness to participate in GJC lifecycle events (e.g., a thin TypeScript shim plugin that shells out, if policy permits), so issueops does not have to reverse-engineer the boundary.

## Access boundary
All sources are public (official docs + public GitHub repo). No login/paywall/CAPTCHA encountered. GitHub source files fetched via the `gh` API and `code.claude.com`/`developers.openai.com` fetched via reader.

---

## Addendum (2026-06-30) — CORRECTION: full lifecycle integration IS possible via the first-party TS HookAPI

The original report concluded external CLI harnesses cannot integrate the broad lifecycle set. **That conclusion was scoped to the *shell-script discovery surface* (`capability/hook.ts` pre/post files) and the *constrained plugin-bundle hooks*. A third, broader surface exists and was under-weighted: the first-party TypeScript `HookAPI`.**

### What changed
GJC's first-party hook authoring API (`@gajae-code/coding-agent/hooks`, default-export factory `(pi: HookAPI) => {...}`) exposes `pi.on(event, handler)` for the **full** event set, including every event issueops needs:

- `pi.on("context", ...)` → Claude/Codex `UserPromptSubmit`
- `pi.on("session_start", ...)` → `SessionStart`
- `pi.on("session_shutdown", ...)` / `pi.on("turn_end", ...)` / `pi.on("agent_end", ...)` → `Stop` / `SessionEnd`
- `pi.on("session_compact", ...)` / `pi.on("auto_compaction_start"|"auto_compaction_end", ...)` → `PreCompact` / `PostCompact`
- `pi.on("tool_call", ...)` → `PreToolUse`/`PostToolUse` (pre/post phases via the event payload)

The loader (`extensibility/hooks/loader.ts`) loads these as TypeScript modules via native Bun import from `~/.gjc/agent/hooks/` (and `.pi`/settings-configured paths), discovered through `discoverAndLoadHooks`. Unlike the **plugin-bundle** constrained hooks (`DENIED_API_METHODS` incl. `exec`), the **first-party** `HookAPI` does not deny `exec`/`registerCommand`/`sendMessage` — `HookContext` imports `ExecOptions`/`ExecResult`, so a first-party hook can spawn external processes.

### New conclusion
issueops **can** integrate all 7 lifecycle hooks into GJC by shipping one TypeScript shim hook that subscribes to the GJC events above and spawns `issueops hook <event>` in each handler. This is **not** a Claude/Codex-style direct shell-command hook registration — it requires a thin TS adapter file installed into `~/.gjc/agent/hooks/` — but functionally it achieves the same lifecycle coverage.

### Sources for the correction
- gajae-code `src/extensibility/hooks/types.ts` lines 470-560 — `HookAPI.on(...)` full event overload set — https://github.com/Yeachan-Heo/gajae-code/blob/main/packages/coding-agent/src/extensibility/hooks/types.ts — retrieved 2026-06-30
- gajae-code `src/extensibility/hooks/loader.ts` — `loadHooks(paths, cwd)` via native Bun import; `discoverAndLoadHooks(configuredPaths, cwd)` discovers from `.gjc/.pi hooks/` + settings — retrieved 2026-06-30
- gajae-code `examples/hooks/README.md` — `pi.on("tool_call", ...)` + `pi.registerCommand(...)` pattern, `gjc --hook <file.ts>` or `~/.gjc/agent/hooks/` install — retrieved 2026-06-30

### Impact on the upstream issue
The originally drafted issue (requesting broadened shell discovery / exec-enabled plugin hooks) is **partially moot**: the integration path already exists via the first-party HookAPI. A revised ask would instead request **documented guidance for external CLI harnesses to ship a first-party TS hook shim**, or optionally a convention so installers can drop a TS hook into `~/.gjc/agent/hooks/` without hand-authoring TypeScript.
