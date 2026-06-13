---
name: self-verify
description: Run or interpret the agent-harness self-verification loop. Use when the user asks to verify the harness, run the 95-point gate, inspect self-verification candidates, compare or promote verification baselines, or confirm CLI/MCP/native integration health.
---

# Self-verification loop

## Goal

Verify that the harness behaves consistently across Codex and Claude Code, and that CLI, MCP, native integration, state, policy, docs, and skills work as intended. This skill is a QA gate; it does not choose improvements by itself.

## Commands

```bash
./bin/agent-harness self-verify --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
HARNESS_SELF_VERIFY_LLM_EVAL=gate ./bin/agent-harness self-verify --seed=100 --target-score=95 --json
./bin/agent-harness self-verify candidates --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
# Confirmed promote refuses a source snapshot that did not pass the gate; --allow-failed-source is an
# explicit, deliberate override for accepted deviations only — never a pressure/demo shortcut.
```

Default `self-verify` is quick mode: one deterministic evidence pass followed by the final LLM evaluator when LLM eval is enabled. Use `--full` for the full ten-plus-iteration gate. Passing `--iterations` without `--full` is invalid.

`HARNESS_SELF_VERIFY_LLM_EVAL` defaults to off. Set it to `advisory` or `gate` to run the `agy --dangerously-skip-permissions -p` LLM evaluator after deterministic self-verification; explicit CLI flags (`--llm-eval`, `--llm-eval=false`, `--llm-eval-mode`) override the environment.

## Gate

Completion requires every concrete goal score to exceed the target score. The default target is 95. If any item scores 95 or below, the state is not complete; improve, retry, or report the blocker.

If any self-verification candidate test, compilation, or general check fails during the loop, you can execute the `lint_diagnose` MCP tool (or run `agent-harness project lint-diagnose -- <failed_command>`) to quickly diagnose the root cause and receive a targeted fix proposal via Gemini 3.5 Flash. This enables fast auto-healing during the self-verify cycle.

## Candidate catalog

The self-verification improvement catalog in this skill's `CANDIDATES.md` is the source of truth.
