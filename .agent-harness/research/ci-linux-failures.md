# CI Linux Failures

## 2026-07-02: native install adapter golden path drift

- Run: GitHub Actions `CI` push run `28561360348` on `quality-optimization-2026-07-02` at `1b214bb`.
- Failure: `go test ./... -count=1` failed only in `agent-harness/internal/adapter`, `TestNativeInstallAdapterContractMatrix`.
- Cause: the native install adapter contract golden recorded the macOS Reasonix user MCP config path as `$HOME/Library/Application Support/reasonix/config.toml`. Linux uses `$HOME/.config/reasonix/config.toml`, so the golden was platform-sensitive even though the contract intent is host behavior, not OS config-dir spelling.
- Fix: normalize both Reasonix user config directories to `$REASONIX_CONFIG` before golden comparison and refresh `native_install_contract_matrix.golden.json`.
- Verification: `go test ./internal/adapter -count=1`; `go test ./... -count=1`; rerun GitHub Actions CI on the updated branch.

## 2026-07-02: response contract docs-count drift after CI note

- Run: GitHub Actions `CI` push run `28561532136` on `quality-optimization-2026-07-02` at `94aec77`.
- Failure: `go test ./... -count=1` failed in `agent-harness/cmd/harness/harnessapp`, `TestResponseContractsGolden`.
- Cause: adding this research note changed the docs-index projection from 67 to 68 indexed docs. The relaxed response contract intentionally keeps `docs_count` and the required docs list golden-pinned while avoiding full docs body snapshots.
- Fix: refresh `response_contracts.golden.json` so the docs projection includes this research note.
- Verification: `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`; `go test ./... -count=1`; rerun GitHub Actions CI on the updated branch.

## 2026-07-02: response contract CI auth and float drift

- Run: GitHub Actions `CI` push run `28561705733` on `quality-optimization-2026-07-02` at `9451496`.
- Failure: `go test ./... -count=1` failed in `agent-harness/cmd/harness/harnessapp`, `TestResponseContractsGolden`.
- Cause: the response contract still compared machine-dependent details: Go JSON output exposed equivalent two-decimal quality scores with different floating-point tails, and the `gh issue view` authentication error differed between local unauthenticated shells and GitHub Actions without `GH_TOKEN`.
- Fix: normalize `score` fields to two decimal places and normalize `IssueOps remote reflect-devils-advocate` GitHub auth failures to `$GH_AUTH_ERROR`; refresh `response_contracts.golden.json`.
- Verification: `go test ./cmd/harness/harnessapp/responsecontract -count=1`; `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`; `go test ./cmd/harness/harnessapp ./internal/adapter -count=1`; rerun GitHub Actions CI on the updated branch.

## 2026-07-02: self-verify native integration on fresh runner

- Run: GitHub Actions `CI` push run `28561931246` on `quality-optimization-2026-07-02` at `cc67970`.
- Failure: `go test ./... -count=1` passed, then `./bin/agent-harness self-verify --seed=100 --target-score=95 --json` failed at the `native_integration` goal.
- Cause: the self-verify native integration step checks actual user-home Codex/Claude skill, MCP, and hook wiring. A fresh GitHub runner has no agent-harness native install unless the workflow performs one first.
- Fix: run the self-verify gate inside a temporary CI HOME, install only agent-harness native integrations there with `./scripts/install-native.sh --skip-build --path-mode=skip`, then run `self-verify` with the same `HOME`/`CODEX_HOME`. This verifies native integration without adding runner-global home state to later contract tests.
- Verification: temp-HOME reproduction of `install-native` followed by `self-verify`; rerun GitHub Actions CI on the updated branch.

## 2026-07-02: self-verify after runner-global install

- Run: GitHub Actions `CI` push run `28562108646` on `quality-optimization-2026-07-02` at `1eedaf1`.
- Failure: the standalone `go test ./... -count=1` step passed, the runner-global native install step passed, then the self-verify gate failed inside its own `go test ./... -count=1`.
- Cause: installing user-level native integrations into `/home/runner` before self-verify changed the process-wide home integration state observed by the nested test run.
- Fix: replace the runner-global install step with the temp-HOME self-verify wrapper described above.
- Verification: temp-HOME reproduction of `install-native` followed by `self-verify`; rerun GitHub Actions CI on the updated branch.
