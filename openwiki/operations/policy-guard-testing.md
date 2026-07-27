---
type: Operations
title: Policy, Guard, and Testing
description: Command execution policy catalog with tier-based classification, code anti-pattern guard with block/warn/review severities, and the testing conventions including golden contracts and cross-host tool conformance.
tags: [policy, guard, testing, golden-contracts, conformance]
---

# Policy, Guard, and Testing

agent-harness separates three quality enforcement layers: **command policy** (runtime command classification), **guard** (code-level anti-pattern detection), and **testing conventions** (verification standards including golden contracts).

## Command Policy

The command policy is a **policy-check-only system**, not a real shell runner. It evaluates whether a command would be allowed before execution. The fake runner returns policy results and audit IDs without executing anything.

### Policy Tiers

The policy derives a named privilege tier from capability flags:

```
read_only → workspace_write → network_access → shell_exception
```

YOLO/auto-escalation tiers are deliberately excluded. Source: [`internal/core/policy/policy_types.go`](/internal/core/policy/policy_types.go).

### Classification Catalog

The built-in catalog ([`internal/core/policy/policy_catalog.go`](/internal/core/policy/policy_catalog.go)) classifies commands into six maps:

| Category | Examples |
|----------|---------|
| Shell interpreters | `sh`, `bash`, `zsh`, `fish`, `dash`, `ksh` |
| Network commands | `curl`, `wget`, `ssh`, `npm`, `pip`, package managers |
| Network subcommands | `git fetch`, `git pull`, `git push`, `git clone` |
| Write commands | `rm`, `mv`, `cp`, `tee`, `go build`, `go test` |
| Write subcommands | `git add`, `git commit`, `git reset` |
| Read-only commands | `ls`, `cat`, `grep`, `find`, `git status`, `git diff`, `git log` |

Policy overrides from `.agent-harness/policy.json` are **additive only** — they can add commands, never remove from the built-in catalog.

### Evaluation

`EvaluateCommandPolicy` ([`internal/core/policy/policy_evaluate.go`](/internal/core/policy/policy_evaluate.go)) validates workspace root, CWD containment, argv, timeout (≤15m), env allowlist, and secret-like arguments, then applies classification rules:

- `cwd` outside `workspace_root` → deny
- Path-like argv pointing outside workspace → deny
- Shell interpreter without reason → deny
- Network command when `network_allowed=false` → deny
- Write command when `write_allowed=false` → deny
- Read-only command outside allowlist → deny
- Secret-like path/argument → deny

### Policy Surfaces

| Surface | Commands |
|---------|----------|
| CLI | `policy check`, `policy fake-run`, `policy audit` |
| MCP | `command_policy_check`, `command_fake_run`, `command_policy_audit` |
| Resource | `harness://command-policy` |

`policy audit` appends a redacted JSONL audit record. It does not execute the command.

## Guard

The guard scans code files for anti-patterns that cause flaky tests, security issues, or maintainability problems. It operates on staged files, all files, or explicit file lists.

### Guard Severities

| Severity | Behavior |
|----------|----------|
| `block` | Prevents commit (exit code 3) |
| `warn` | Reports but allows |
| `review` | Flags for human review |
| `info` | Informational |

### Guard Rules

| Rule | Severity | Trigger |
|------|----------|---------|
| `sleep-in-test` | **block** | Wall-clock sleep (`time.Sleep`, `Thread.sleep`, `setTimeout`) in test code |
| `real-external-service-in-test` | **block** | External URL in test (not an allowlisted fixture) |
| `secret-like-path` | **block** | `.env`, `id_rsa`, `*.pem`, `*.key`, `*credentials*`, `*secret*` in staged files |
| `nondeterministic-context-serialization` | warn | `time.Now()`, `rand.*`, `uuid.New` in immutable-prefix file without `volatile-ok` |
| `ambiguous-test-name` | warn | Generic test names (`TestWorks`, `TestBasic`) |
| `localhost-in-test` | warn | Localhost dependency |
| `snapshot-test-review` | warn | `toMatchSnapshot`, golden assertions |
| `prod-change-without-test` | warn | Production source changed, no test changed |
| `contract-surface-without-golden` | warn | CLI/MCP/adapter contract surface changed, no golden update |
| `large-test-fixture` | warn | Test file > 200KB |

Source: [`internal/core/guard/findings.go`](/internal/core/guard/findings.go), [`internal/core/guard/pattern/patterns.go`](/internal/core/guard/pattern/patterns.go).

### Markers

- **`harness:immutable-prefix`**: Marks a file region where non-deterministic code is forbidden.
- **`volatile-ok`**: Explicitly permits non-deterministic code in an immutable-prefix region.

### Guard Surfaces

| Surface | Command |
|---------|---------|
| CLI | `guard check --staged`, `guard check --all`, `guard check --files ...` |
| Exit code | 3 on `GuardBlockedError` |

## Testing Conventions

### Basic Verification

```bash
# Full test suite
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```

For small changes, run targeted tests first, then the full suite before completion.

Source: [`.agent-harness/TESTING.md`](/.agent-harness/TESTING.md).

### Core Package Tests

```bash
go test ./internal/core -count=1
go test ./cmd/harness -count=1
```

### Golden Contract Updates

Golden files are updated **only** when the schema is deliberately changed:

```bash
# CLI/MCP response golden
go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1

# Host adapter install contract golden
go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract -count=1
```

Source: [`cmd/harness/testdata/`](/cmd/harness/testdata/).

### Test Quality Standards

**Well-structured tests:**
- Validate changed public behavior/contract directly
- Deterministic — no test order, wall-clock sleep, real network, or local machine state dependency
- Regression tests include the reproducing input, false cases, and expected result
- Reuse existing helpers and style
- Use `internal/testsupport` capture helpers for stdout/stderr pipe tests

**Poorly-structured tests:**
- Fixate on internal structure unrelated to requirements
- Rely on `time.Sleep`, real network, or execution order
- Use web search for local symbol discovery

### Cross-Host Tool Contract Conformance

The tool contract conformance system validates that MCP tools receive correct arguments across hosts:

```bash
# Deterministic baseline (no network/auth)
./bin/agent-harness contract conformance baseline --json

# Live measurement (opt-in, external cost)
HARNESS_TOOL_CONFORMANCE_LIVE=1 agent-harness contract conformance live \
  --hosts codex,claude --profile clean --target-completed 1 --json

# Replay a fixture
agent-harness contract conformance replay --fixture PATH --json
```

- **Baseline**: 3 representative schemas × 10 preregistered cases (valid, unknown_key, coercible_type_drift, noncoercible_type_drift). Deterministic, no external dependencies.
- **Live**: Clean-context `3 hosts × 3 fixtures = 9 completed episodes`. Environment/transport/no-call attempts are excluded from the denominator. Max 3 retries per case.
- **Promotion**: An invalid raw call must be reproduced with the same diagnostic signature **twice or more** before becoming a regression fixture. One-time observations are never promoted.

Evidence is stored in mode 0600/0700 under `.agent-harness/evidence/tool-conformance/` and never added to git.

Source: [`.agent-harness/TESTING.md`](/.agent-harness/TESTING.md) §2, [`internal/core/toolconformance/`](/internal/core/toolconformance/).

### Self-Verify Quality Gate

The harness quality gate runs a **25-step deterministic verification pipeline** per iteration:

```bash
# Quick check (single iteration, default)
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json

# Full 10-iteration gate
agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

The 25 steps run in order: `harness invariants` → `go test` → `contract golden` (cached) → `risk QA tier` → `go build` → `binary drift` → `inspect smoke` → `docs index` → `candidate export` → `step budget` → `install dry-run` → `command policy smoke` → `command audit smoke` → `contract check` → `tool contract conformance` → `worker lifecycle` → `MCP smoke` → `state roundtrip` → `parallel isolation` → `daemon resilience` → `preflight fuzz` → `web fetch` → `native integration` → `redaction audit` → `QA gate`.

**Fail-fast by default.** `CollectAllSteps: true` mode continues after a step failure to surface all failing gates — it never weakens the gate. The golden test step short-circuits if `go test` already passed.

**Target score**: default 95. Every concrete goal must score **above** 95 (exclusive). Score ≤ 95 = not complete.

**LLM eval**: defaults to **off**. Setting `HARNESS_SELF_VERIFY_LLM_EVAL=gate` only renders the evaluator prompt — no external API call is made. Explicit `--llm-eval=false` overrides the environment.

**Run modes**: `quick` (single deterministic pass) or `full` (10 iterations). Passing `--iterations` without `--full` is invalid.

Source: [`cmd/harness/selfworkflow/verifyloop/loop.go`](/cmd/harness/selfworkflow/verifyloop/loop.go), [`cmd/harness/selfworkflow/steps/self_verify_steps.go`](/cmd/harness/selfworkflow/steps/self_verify_steps.go).

### Self-Augment Loop

Self-augment identifies and implements the next valuable improvement, then verifies it:

```bash
agent-harness self-augment --cycles=1 --target-score=95 --json
```

**Exit criteria** (all must be met): all goals exceed target score (95); ≥10 candidates scored by impact, feasibility, novelty, risk, verification cost, user value; at least one actual code/docs/skill diff (cosmetic-only doesn't count); `self-verify` passes; decisions/lessons captured via `self-augment lesson`.

**Reflexion lessons**: `self-augment lesson --candidate X --lesson "..." --next-action "..." --state-key Y` stores reusable lessons for future cycles.

**Distinction**: self-verify = "does it work?" (QA gate). Self-augment = "implement the next valuable improvement, then verify it."

Source: [`cmd/harness/selfworkflow/augmentcmd/augment.go`](/cmd/harness/selfworkflow/augmentcmd/augment.go), [`skills/self-augment/SKILL.md`](/skills/self-augment/SKILL.md).

### IssueOps Benchmark

IssueOps benchmark fixtures are repo-agnostic, scoring portable workflow evidence:

```bash
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
```

A passing deterministic benchmark requires `average_score == 100`, `minimum_score == 100`, and `critical_failure_count == 0`.

### Operational Health and Stability Audit

- Pure classifier tests pin the 15-minute heartbeat boundary, invocation-only preserves, duplicate/incomplete inventory failure, and exact resource ownership.
- External vocabulary enumerations cite the upstream definition, not observed samples.
- Stability audit builds the binary, then delegates operational judgement to `doctor`.
- Final live reconciliation runs only after the external recovery manifest/journal is sealed.

Source: [`.agent-harness/TESTING.md`](/.agent-harness/TESTING.md) §2 "Operational-health and stability delegation".
