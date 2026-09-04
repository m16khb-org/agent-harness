# Omo native as a first-party host

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-12
- Status: accepted

## Context

Agent-harness installed shared skills, MCP registration, and lifecycle hooks for
Codex and Claude Code, while Omo could only discover whichever skills happened
to exist under its independent user roots. The observed result was partial:
9 of 12 pioneer skills were auto-discovered even though all 12 parsed when
loaded explicitly.

Treating that as sufficient would make install/update results host-dependent
and leave Omo MCP and project-doc lifecycle activation outside strict readback.

## Decision

Omo native is the third first-party host adapter.

- User skills are symlinked from the canonical `skills/` tree into
  `~/.omo/skills`.
- User MCP registration is merged into `~/.omo/mcp.json` as
  `issueops`, preserving unrelated servers and removing the obsolete
  hyphenated alias.
- `~/.omo/extensions/issueops.js` maps Omo `session_start` and accepted
  `session_compact` events to the shared raw lifecycle hook commands and injects
  hidden project-doc context without triggering a new turn.
- IssueOps binds Omo mutations to `PI_SESSION_ID` plus a live process receipt.
  Omo 5.x RPC mode detaches the `omo` launcher, so its persistent `senpi`
  runtime receipt is accepted as the Omo process identity only for
  `host=omo`; a session ID alone never authorizes a lease.
- Explicit project-local install may write `.omo/skills/*` and
  `.omo/mcp.json`; default install writes no target-repository Omo files.
- Activation commits only after exact MCP semantics and lifecycle extension
  bytes are read back with filesystem identity evidence.
- Agent-harness does not install Omo itself or make the external runtime a
  readiness gate.

## Consequences

`install`, `update`, dry-run output, adapter contract goldens, native
integration verification, and operating docs cover Codex, Claude Code, and Omo
in a deterministic order. New shared skills are available to all three hosts
unless an explicit `install.json` host filter excludes one.
