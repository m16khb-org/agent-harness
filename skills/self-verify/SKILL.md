---
name: self-verify
description: Run or interpret the agent-harness self-verification loop. Use when the user asks to verify the harness, run the 95-point gate, inspect self-verification candidates, compare or promote verification baselines, or confirm CLI/MCP/native integration health.
---

# Self-verification loop

## Goal

First-party hosts are exactly Codex and Claude Code. Verify that the harness behaves consistently across both hosts and that CLI, MCP, native integration, state, policy, docs, and skills work as intended. This skill is a QA gate; it does not choose improvements by itself.

## Commands

```bash
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
HARNESS_SELF_VERIFY_LLM_EVAL=gate ./bin/agent-harness self-verify --seed=100 --target-score=95 --json
./bin/agent-harness self-verify candidates --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
# Confirmed promote refuses a source snapshot that did not pass the gate; --allow-failed-source is an
# explicit, deliberate override for accepted deviations only — never a pressure/demo shortcut.
```

Default `self-verify` is quick mode: one deterministic evidence pass. Use `--full` for the full ten-plus-iteration gate. Passing `--iterations` without `--full` is invalid.
The quick and full modes both run the deterministic 10-case `contract conformance baseline`; they never launch Codex, Claude, or a live model. The normative live/reproduction and evidence rules are in `.agent-harness/TESTING.md`.

`HARNESS_SELF_VERIFY_LLM_EVAL` defaults to off. In the current implementation, setting it to `advisory` or `gate` only renders the read-only evaluator prompt after deterministic self-verification. No Z.AI request is sent. The `advisory` result exposes that prompt without changing deterministic success, while `gate` therefore returns a non-passing `llm_eval` result because no external verdict is ingested.

For the repository's deterministic completion gate, pass explicit `--llm-eval=false` when the environment intentionally exports `HARNESS_SELF_VERIFY_LLM_EVAL=gate`; explicit CLI flags override the environment. Record that override and restart the verification sequence from its first gate after any interrupted or prompt-only run. Do not report a prompt-only result as an external LLM judgment.

## Gate

Completion requires every concrete goal score to exceed the target score. The default target is 95. If any item scores 95 or below, the state is not complete; improve, retry, or report the blocker.

If any self-verification candidate test, compilation, or general check fails during the loop, you can execute the `lint_diagnose` MCP tool (or run `agent-harness project lint-diagnose -- <failed_command>`) to quickly diagnose the root cause and receive a targeted fix proposal via Gemini 3.5 Flash. This enables fast auto-healing during the self-verify cycle.

다단계 검증에서 한 단계라도 실패하면 1단계부터 재실행하며 부분 통과 evidence를 재사용하지 않는다 (규범 출처: `.agent-harness/TESTING.md` 부분 검증 상태 금지 절).

## Candidate catalog

The self-verification improvement catalog in this skill's `CANDIDATES.md` is the source of truth.
