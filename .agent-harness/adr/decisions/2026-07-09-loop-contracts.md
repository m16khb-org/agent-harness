# 2026-07-09 — Loop contracts

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: loop/article gaps 1-2-3 plan, distilled from external article notes on agent loops, representative-task validation, and fail-closed verification contracts
- Summary: Add a durable `agent-harness loop` state machine, keeping the harness as a state/gate recorder rather than a scheduler or verifier.
- Context: The plan originally considered broader loop automation after article-inspired gap analysis. Brooks-style trim reduced the implementation to the smallest public contract that enforces observable completion discipline: four loop tools, repo+name identity, and strict readiness consumption. The user explicitly directed including the consumer readiness gate rather than leaving a spike-only state package.
- Decision:
  - Add `loop start/record-attempt/status/stop` CLI and matching MCP tools. `start` records `verify_argv` but never runs it; `record-attempt` requires evidence; `stop --success` requires the latest attempt to be `pass`.
  - Key loops by normalized repo+name. Active loops resume, terminal loops require a fresh name, and same-repo `active`/`exhausted` loops block strict PR readiness as `loop_incomplete:<loop-id>`.
  - Keep partial-verification policy normative in `.agent-harness/TESTING.md`; skills only point to that source.
- Consequences: Golden contracts include the loop command/tool list. Loop state lives in user-state `loop/` and is diagnosed by `agent-harness doctor`.
- Evidence:
  - `internal/core/looprun` state machine and tests
  - `internal/core/issueops_facade.go` strict readiness `loop_incomplete:` gate
  - `cmd/harness/testdata/{usage.golden.txt,mcp_tools.golden.json,response_contracts.golden.json}` regenerated after CLI/MCP wiring
- Alternatives / rejected options:
  - Scheduler or automatic verify-command execution — rejected because host approvals, tool policy, and command side effects must remain with the active agent; the harness records evidence and gates only.
  - Hook-enforced loop execution — rejected because hooks must stay fast observe/relay/block surfaces and cannot own long-running verification.
  - Token or cost telemetry enforcement — rejected as speculative; attempt evidence records observed attempts/time without inventing a cost model.
  - `state write` convention instead of a loop state machine — rejected because arbitrary checkpoints cannot enforce evidence-required attempts, max-attempt exhaustion, terminal restart refusal, or strict readiness gating.
  - Public `loop list` surface — rejected to keep the first contract minimal; doctor and PR readiness are the consumers.
  - `success_criteria` field — rejected because the loop goal plus evidence list is enough for the first state machine and avoids duplicating IssueOps intent contracts.
  - Goal-hash identity — rejected because repo+name is debuggable and supports intentional resume.
