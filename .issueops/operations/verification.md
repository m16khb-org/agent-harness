---
name: verification.md
description: self-verify, self-augment, API documentation gates, and operational smoke checks.
---

# Verification Operations

## Self-Verify

Quick mode is the default:

```bash
issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
issueops self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-latest --json
```

One deterministic pass can collect every step instead of failing fast:

```bash
issueops self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

`--progress=jsonl` keeps final JSON on stdout and writes step heartbeat lines to stderr so long runs are not mistaken for hangs. Repeated full/iteration modes were removed because they reran the same expensive evidence pass without adding independent evidence.

The deterministic project gate pins `--llm-eval=false`. An ambient `ISSUEOPS_SELF_VERIFY_LLM_EVAL=gate` is valid, but the current opt-in path only renders a read-only evaluator prompt, sends no Z.AI request, and cannot pass gate mode without an ingested external verdict. Record the explicit override and restart from the first gate after an interrupted or prompt-only attempt.

### Orca execution focused and native hook smokes

```bash
go test ./internal/domain/issueops/... ./internal/adapter/orca ./internal/domain/commandparse ./internal/adapter/skillcontract ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookinput ./cmd/issueops/issueopscli ./cmd/issueops/issueopsapp -count=1
```

The hook-input package is `./cmd/issueops/hookcli/hookinput`; there is no domain-level hook-input package. It only reads the repo/cwd the context hooks need; Codex, Claude, and Omo fixtures go through the same reader and the same catalog shape.

Checkpoint commands:

```bash
issueops self-verify candidates --json
issueops self-verify candidates --save-state --state-key self-verify-candidates-latest --json
issueops self-verify history --prefix self-verify --json
issueops self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
issueops self-verify history --prefix self-verify --retention-limit 20 --prune-retention --confirm --json
issueops self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
issueops self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
```

`--save-state` stores a compact summary snapshot, not full run logs. `history`, `compare`, and `promote` operate on those snapshots. Retention pruning is dry-run unless `--confirm` is present.

## Self-Augment

```bash
issueops self-augment --cycles=1 --target-score=95 --json
issueops self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
issueops self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
```

## API Documentation Gate

`issueops api-doc review` is a framework-agnostic host-agent review gate for endpoint/DTO/OpenAPI documentation drift. By default it scopes to staged API candidate files only. Without `--result`, it renders the prompt/schema the host agent must run. With `--result`, it records the host agent's JSON verdict as evidence.

```bash
issueops api-doc review --json
issueops api-doc review --result review.json --json
issueops api-doc check --result review.json --json
issueops api-doc review -- src/users/users.controller.ts internal/api/user_handler.go openapi.yaml
issueops api-doc review --prompt-file docs/api-doc-rules.md
```

Project-specific strictness belongs in `--prompt-file`, not in harness core.

Target Node/Nest repos may use:

```json
{
  "scripts": {
    "swagger:check": "issueops api-doc check --json"
  }
}
```

Business-logic-aware rule: API docs review must inspect directly related service/usecase/domain error mapping, not only decorators/comments. If changed code can return public errors such as 404, 403, 409, validation 400, or auth 401, OpenAPI responses must document them.

Endpoint/controller/DTO/schema/OpenAPI changes use `.issueops/OPEN_API_SPEC.md` as the project-specific prompt source. `issueops api-doc review` includes it automatically when no explicit `--prompt-file` is provided.

## General Verification

`issueops self-verify compare` reports `slow_step:*` regressions when label-level `slowest_steps` deltas exceed the configured regression threshold. Keep this marker documented because self-augmentation uses it as evidence that performance baseline regression support is present.

```bash
issueops inspect --json
issueops docs --json
issueops daemon status --json
issueops policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
scripts/release-repro-smoke.sh
scripts/release-build-matrix.sh
git diff --check
go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
go test ./... -count=1
```

Use `.issueops/TESTING.md` for change-specific test selection and reporting requirements.
