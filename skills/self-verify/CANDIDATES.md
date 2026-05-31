# Self-verification candidate catalog

Date: 2026-05-27
Scope: reliability, observability, security, regression detection, and native integration coverage for `harness self-verify`.

This file is the backlog for improving the self-verification loop itself. It is not the implementation backlog for self-augmentation. Even when the current loop passes the 95-point gate, this catalog records gaps that future runs could miss.

## Current evidence

A redirected baseline run completed successfully after roughly 52 seconds. The run reported `minimum_goal_score=100` with every goal at `100/100`. The slowest steps were risk-QA tier checks, around 2.4 to 2.6 seconds each. Existing `./bin/agent-harness self-augment --json` candidates were already satisfied and no candidate was selected.

Conclusion: the run was not hung, but redirected execution gave too little progress feedback. Candidate priority therefore emphasizes progress observability, blind-spot detection, and reproducibility rather than pass/fail alone.

## Candidate generation method

The catalog uses these `GENIUS_THINK.md` patterns:

- Problem reframing: change “Did self-verify pass?” into “Is the pass result observable, repeatable, secure, and stable under polluted or parallel environments?”
- Innovative solution generation: score impact, feasibility, novelty, risk, verification cost, and user value together.
- Thought evolution: convert real operational friction, such as interrupted long runs, into the next candidate.
- Complexity resolution: split verification trust into observability, security, state, performance, contract, and reproducibility concerns.

Priority score formula:

```text
priority = impact*0.25 + feasibility*0.20 + novelty*0.15 + (100-risk)*0.15 + (100-verification_cost)*0.10 + user_value*0.15
```

## Candidate list

2026-05-31 cycle update: LangChain harness-engineering analysis reopened one completion evidence gap. Current open self-verification candidate: `completion-evidence-audit`.

| Priority | Candidate ID | Category | Score | Why it mattered | Verification |
| --- | --- | --- | ---: | --- | --- |
| 1 | `self-verify-progress-heartbeat` | observability | 81 | Long redirected runs looked hung without progress events. | `self-verify --progress=jsonl`, heartbeat tests, unchanged `--json` stdout contract. |
| 2 | `self-verify-secret-redaction-audit` | security | 79 | Secret-like output in command logs, state, goldens, or MCP responses would make the verifier a leak path. | Synthetic secret fixtures and redaction scans. |
| 3 | `self-verify-coverage-gap-report` | coverage | 78 | Invariant-to-step ownership needed to be machine-visible. | Coverage matrix fixtures and unowned-claim failures. |
| 4 | `completion-evidence-audit` | completion evidence | 80 | Completion reports can still omit structured `verify-work`/guard evidence even though the CLI exists. | `verify-work --json` fixture, candidate export, and workflow doc completion-report check. |
| 5 | `self-verify-failure-rerun-recipe` | reproducibility | 78 | Failed steps needed copy-paste rerun commands with seed/env context. | Failing fixture and `summary.rerun_commands`. |
| 6 | `self-verify-candidate-export` | curriculum | 77 | Future self-verify improvements needed a dedicated export after self-augment candidates were satisfied. | `self-verify candidates --json`, MCP golden, state export. |
| 7 | `self-verify-step-budget-baseline` | performance | 76 | Gradual slowdowns needed p95 label budgets, not only top-5 slowest steps. | `summary.step_duration_stats` and compare regressions. |
| 8 | `self-verify-install-dry-run-smoke` | native integration | 76 | Dry-run install needed explicit no-write verification evidence. | Temp HOME/CODEX_HOME/HARNESS_ROOT dry-run smoke. |
| 9 | `self-verify-policy-path-fuzz-plus` | policy/security | 76 | Path policy needed symlink, `~/`, URL, git ref, and Unicode cases. | Seeded policy fuzz table and negative fixtures. |
| 10 | `self-verify-json-schema-contract` | contract | 76 | Expanding summaries needed schema/hash drift detection beyond manual golden review. | Contract hash and required-field tests. |
| 11 | `self-verify-flake-classifier` | reliability | 75 | Intermittent seed failures needed deterministic vs flaky classification. | Synthetic intermittent step and failure clustering. |
| 12 | `self-verify-output-size-budget` | operations | 73 | Large failing stdout/stderr could bloat JSON/state output. | Oversized output fixture, truncation metadata, state-size checks. |
| 13 | `self-verify-history-retention-budget` | state operations | 71 | Unbounded history could slow state and mix stale baselines. | Retention dry-run/confirm tests. |
| 14 | `self-verify-parallel-temp-isolation` | concurrency | 70 | Concurrent runs needed isolated temp state and build artifacts. | Parallel seeded smoke and race tier. |
| 15 | `self-verify-duplicate-mcp-warning` | native integration | 70 | Duplicate Claude/Codex MCP registrations can pass smoke checks but harm UX. | Mocked duplicate-scope fixture and warning classification. |
| 16 | `self-verify-daemon-restart-resilience` | daemon | 68 | Daemon-backed MCP needed stale-lock/socket restart recovery checks. | Temp socket fixture, restart smoke, stale-lock test. |

## Completion criteria

This catalog is complete when it contains at least 10 candidates, each candidate directly improves the self-verification loop, each has a reason and verification method, and the latest baseline evidence is reflected. Future cycles should first inspect `self-verify candidates --json`; open candidates should be selected by score and priority.
