---
title: "Claude/Codex Stop hook: continue vs decision semantics"
source: "agent-harness investigation 2026-06-04"
target_wiki: "dev-fundamentals"
target_type: "notes"
summary: "A Stop hook must return decision:block + reason (with continue left true) to make the agent recover in-turn; continue:false is a hard stop that takes precedence over decision and halts the agent. Verified against Claude 2.1.162 and Codex 0.137.0 binaries."
suggester: "manual"
model: "claude-opus-4-8"
---

# Claude/Codex Stop Hook Output Semantics

When a Stop hook wants the agent to **recover and keep working in the same turn**
(e.g. re-emit missing numbered choices), the output field choice is decisive.

## Verified host contract

Both hosts share the same Stop hook output schema (verified by inspecting the
installed binaries, not docs guesses):

- **Claude `2.1.162`** — embedded hook docs:
  - `continue` — "Set to `false` to block/stop (default: true)"
  - `stopReason` — "Message shown when `continue` is false"
  - `decision` — "block" for Stop hooks, with `reason` as the explanation fed to the model
- **Codex `0.137.0`** — `stop.command.output` JSON schema:
  - `continue` (boolean, default `true`)
  - `decision` → `BlockDecisionWire` enum `["block"]`, default null
  - `reason` (string) — note: "Claude requires `reason` when `decision` is `block`; we enforce that semantic rule during output parsing"

## The rule

| Goal | Output |
|------|--------|
| Let the agent continue in-turn and act on guidance | `{"decision":"block","reason":"...","continue":true}` (or omit `continue`) |
| Hard stop and show a message to the user | `{"continue":false,"stopReason":"..."}` |

`continue:false` **takes precedence over `decision`**. Sending both
`continue:false` and `decision:block` halts the agent and surfaces the reason
to the user — the agent never gets to act on it.

## Anti-loop guard

Hosts set `stop_hook_active: true` on a Stop that is itself a continuation of a
prior stop-hook block. A `decision:block` Stop hook must allow the stop (no-op
`{}` output) while that flag is true, otherwise a non-complying agent loops
forever.

## Other Stop-hook constraints

Stop hooks accept only the stop-control schema
(`continue`/`decision`/`reason`/`stopReason`/`systemMessage`/`suppressOutput`).
Injecting `hookSpecificOutput.additionalContext` on a Stop event makes Codex
report "invalid stop hook JSON output"; use a no-op `{}` payload when not
blocking.

## Provenance

Observed 2026-06-04 in agent-harness: the `--enforce-numbered-next-actions`
Stop hook block branch sent `continue:false`, so the agent halted instead of
presenting the required `선택지:` choices. Fixed in
`cmd/harness/hook_user_prompt.go` by switching the block branch to
`continue:true` (matching the auto-proceed branch) and guarding with
`stop_hook_active`.
