---
name: workflows
description: Codex-only dynamic workflow execution with batched subagents. Use when the user says workflows, dynamic workflows, run a workflow, use a workflow, fan out agents, launch subagents, coordinate parallel agents, perform a broad audit/migration/research sweep, or asks for Claude Code dynamic workflows behavior in Codex.
---

# Workflows

## Scope

`workflows` is the Codex orchestration primitive. It runs an explicit workflow for the current task.

## Evidence-backed model

Claude Code dynamic workflows move the orchestration plan into a generated script that coordinates subagents. Codex cannot reuse that proprietary runtime, so Codex emulates the behavior with a controller-led protocol:

- the main Codex agent is the workflow controller;
- subagents do bounded independent work;
- the controller keeps the phase graph, ledger, verification gates, and final synthesis.

Scale facts from Claude Code docs: up to 16 concurrent agents and 1,000 total agents per run. In Codex, use those as inspiration, not a promise: start smaller, batch aggressively, and only scale when the user explicitly wants broad fan-out.

## Controller protocol

1. **Declare the workflow**
   - State goal, acceptance criteria, phases, stop condition, `max_concurrent`, and `max_total`.
   - Defaults: `max_concurrent=4-8`, `max_total<=32`.
   - For explicit large sweeps, use up to `max_concurrent=16` and split about 100 agents across batches/phases.
2. **Partition the work**
   - Split by directory, subsystem, hypothesis, file glob, test class, or evidence source.
   - Every partition must have a non-overlapping scope and expected output schema.
3. **Spawn bounded batches**
   - Use Codex subagent tooling such as `multi_agent_v1.spawn_agent` when available and authorized by the user's workflow/subagent request.
   - Keep prompts self-contained. Ask for evidence paths/lines, confidence, risks, and verification commands.
   - Wait at phase barriers, not after every single spawn.
4. **Maintain a ledger**
   - Track: phase, partition, agent id, prompt, status, result summary, evidence, confidence, and follow-up.
   - Summarize completed agents into the ledger before starting dependent phases.
5. **Verify and reduce**
   - Deduplicate findings.
   - Launch verifier/critic/reviewer agents for high-impact claims or broad edits.
   - Run deterministic local checks whenever possible.
6. **Synthesize**
   - Report final answer with evidence, unresolved risks, changed files, commands run, and skipped checks.

## Workflow types

- **Research workflow**: split by source, question, hypothesis, or competing interpretation.
- **Audit workflow**: split by subsystem/path and use verifier passes for high-risk findings.
- **Implementation workflow**: split write scopes so agents do not edit the same files; main controller integrates.
- **Verification workflow**: split by test layer, contract, docs, runtime behavior, and regression risk.

## Safety rules

- Do not spawn agents merely for style or vague thoroughness.
- Do not run a 100-agent sweep unless the task is broad enough and the user explicitly wants that scale.
- Keep secret-bearing files and destructive commands out of subagent prompts unless specifically required and safe.
- Prefer read-only exploration before parallel writes.

## Reference

Read `references/claude-code-workflows-research.md` when exact Claude Code behavior, limits, permissions, or effort-mode relationship matters.
