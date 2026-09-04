---
name: cautions/lessons/2026-08-11-self-verify-iterations-full-modes-removed.md
description: Dated lesson — self-verify --full/--iterations multi-seed modes removed 2026-08-11; historical behavior preserved as evidence.
---

# 2026-08-11 — self-verify `--full`/`--iterations` modes removed

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: self-verify CLI refactor (2026-08-11)
- Summary: The `self-verify` `--full` + `--iterations=N` multi-seed execution modes were removed on 2026-08-11. They are no longer operational commands. The historical behavior and observed budgets are preserved here as incident-time evidence only.
- Removed statements (verbatim from the prior CAUTIONS.md §18 and stability-audit block):
  - `self-verify --iterations=N` requires `--full`; without it the CLI exits fast with "--iterations requires --full". Observed 2026-06-03: `e2e_stability_audit.py` invoked `self-verify --iterations=10` without `--full`, so the audit reported a false self-verify failure in ~150ms while a direct quick run passed 22/22.
  - Give the full 10-iteration self-verify a generous timeout (>=180s); the 10 seeded deterministic iterations exceed the quick-mode budget.
  - `self-verify --full --iterations=10`은 매 seed마다 test/race를 실제 실행한다. 현재 약 3712초가 관측됐으므로 audit timeout은 5400초다.
- Resolution: Do not treat `--iterations requires --full`, `self-verify --full --iterations=10`, the 10 seeded deterministic iterations, or the ~180s / ~3712s / 5400s budgets as current operational commands or timeouts. Current `self-verify` behavior is documented by the testing family's self-verification module (`testing/self-verification.md`). Stability-audit and self-verify invocation still apply the evergreen guidance in [audit-and-process.md](../audit-and-process.md) — suspiciously-fast reproduction, JSON success-predicate discipline, and `--llm-eval=false` for deterministic completion gates remain current.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
