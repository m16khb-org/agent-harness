---
name: verification.md
description: self-verify, self-augment, API documentation gates, and operational smoke checks.
---

# Verification Operations

## Self-Verify

Quick mode is the default:

```bash
agent-harness self-verify --seed=100 --target-score=95 --json
agent-harness self-verify --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
```

Full mode is explicit:

```bash
agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
```

`--progress=jsonl` keeps final JSON on stdout and writes iteration/step heartbeat lines to stderr so long runs are not mistaken for hangs.

Checkpoint commands:

```bash
agent-harness self-verify candidates --json
agent-harness self-verify candidates --save-state --state-key self-verify-candidates-latest --json
agent-harness self-verify history --prefix self-verify --json
agent-harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
agent-harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --confirm --json
agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
```

`--save-state` stores a compact summary snapshot, not full run logs. `history`, `compare`, and `promote` operate on those snapshots. Retention pruning is dry-run unless `--confirm` is present.

## Self-Augment

```bash
agent-harness self-augment --cycles=1 --target-score=95 --json
agent-harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
agent-harness self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
```

## API Documentation Gate

`agent-harness api-doc review` is a framework-agnostic AI gate for endpoint/DTO/OpenAPI documentation drift. By default it reviews staged API candidate files only.

```bash
agent-harness api-doc review
agent-harness api-doc check --json
agent-harness api-doc review -- src/users/users.controller.ts internal/api/user_handler.go openapi.yaml
agent-harness api-doc review --prompt-file docs/api-doc-rules.md
```

The reviewer runs Codex non-interactively with `gpt-5.5` and `model_reasoning_effort=medium` by default, in read-only/no-approval mode. Project-specific strictness belongs in `--prompt-file`, not in harness core.

Target Node/Nest repos may use:

```json
{
  "scripts": {
    "swagger:check": "agent-harness api-doc check --json"
  }
}
```

Business-logic-aware rule: API docs review must inspect directly related service/usecase/domain error mapping, not only decorators/comments. If changed code can return public errors such as 404, 403, 409, validation 400, or auth 401, OpenAPI responses must document them.

Endpoint/controller/DTO/schema/OpenAPI changes use `.agent-harness/OPEN_API_SPEC.md` as the project-specific prompt source. `agent-harness api-doc review` includes it automatically when no explicit `--prompt-file` is provided.

## General Verification

`agent-harness self-verify compare` reports `slow_step:*` regressions when label-level `slowest_steps` deltas exceed the configured regression threshold. Keep this marker documented because self-augmentation uses it as evidence that performance baseline regression support is present.

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
git diff --check
go test ./cmd/harness -run Golden -count=1
go test ./... -count=1
```

Use `.agent-harness/TESTING.md` for change-specific test selection and reporting requirements.
