# 2026-07-03 — Codex PreToolUse ask fallback

← [ADR index](../../ADR.md)

Decision: Codex PreToolUse "ask" outcomes are emitted as a normal `decision="block"` response, while Claude Code keeps `hookSpecificOutput.permissionDecision="ask"`.

Rationale:

- Codex CLI 0.142.5 rejects `hookSpecificOutput.permissionDecision="ask"` with `unsupported permissionDecision:ask`, so emitting native ask breaks the hook before the user sees the gate reason.
- The core lifecycle decision still records `decision="ask"` in JSON analysis, preserving the domain meaning and hook metrics.
- A block fallback is fail-closed and host-compatible for live-access gates such as `kubectl exec` and `kubectl port-forward`.

Rejected alternative: allow Codex ask-style gates by emitting unsupported `permissionDecision="ask"` and relying on the host to recover. This was rejected because it turns a deliberate safety gate into a hook runtime failure.
