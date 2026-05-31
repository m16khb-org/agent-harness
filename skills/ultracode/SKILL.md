---
name: ultracode
description: Codex-only ultracode execution. Emulate Claude Code /effort ultracode by using xhigh-style controller reasoning and automatically applying the workflows orchestration pattern to substantive tasks. Use when the user says ultracode, $ultracode, xhigh workflow orchestration, maximum thoroughness, broad autonomous workflow execution, or wants Claude Code ultracode behavior in Codex.
---

# Ultracode

## Relationship to workflows

`ultracode` uses `workflows`.

`workflows` is the explicit orchestration primitive. `ultracode` is the automatic higher-level mode: for each substantive task, decide whether to run one or more workflows without waiting for the user to say “workflow” again.

## Meaning

In Claude Code, `/effort ultracode` means xhigh effort plus automatic dynamic-workflow orchestration. In Codex, ultracode means:

- reason locally at maximum practical depth before acting;
- classify whether the task needs workflow orchestration;
- if yes, apply the `$workflows` controller protocol: phase graph, batched subagents, ledger, verification, reduce, synthesize;
- if no, execute normally and explain why workflow overhead is not justified.

## Ultracode protocol

1. **Classify**
   - Use workflows for broad investigation, multi-file implementation, migrations, audits, hard debugging, high-stakes verification, or explicit fan-out requests.
   - Avoid workflows for trivial edits, narrow factual answers, or tasks whose next step is tightly coupled and blocking.
2. **Plan the workflow chain**
   - Engineering default: **understand → change → verify**.
   - Research default: **source sweep → contradiction check → synthesis**.
   - Debugging default: **hypothesis fan-out → reproduction/evidence → fix verification**.
3. **Set budget**
   - Default: `max_concurrent=4-8`, `max_total<=32` per workflow chain.
   - Explicit large sweep: up to `max_concurrent=16`; about 100 agents is a large practical Codex sweep, not the default.
4. **Run workflows**
   - Follow `skills/workflows/SKILL.md` for partitioning, spawning, ledger, verifier passes, and synthesis.
5. **Report**
   - Include workflow decision, phases run, agents used, evidence, verification commands, risks, and skipped checks.

## Hard boundaries

- Do not claim Claude Code parity beyond current Codex subagent tooling.
- Do not create `.claude/workflows/` or Claude project config; Claude already has native ultracode/workflows.
- Keep this skill Codex-only via `install.json`.
