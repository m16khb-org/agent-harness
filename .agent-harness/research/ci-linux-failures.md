# CI Linux Failures

## 2026-07-02: native install adapter golden path drift

- Run: GitHub Actions `CI` push run `28561360348` on `quality-optimization-2026-07-02` at `1b214bb`.
- Failure: `go test ./... -count=1` failed only in `agent-harness/internal/adapter`, `TestNativeInstallAdapterContractMatrix`.
- Cause: the native install adapter contract golden recorded the macOS Reasonix user MCP config path as `$HOME/Library/Application Support/reasonix/config.toml`. Linux uses `$HOME/.config/reasonix/config.toml`, so the golden was platform-sensitive even though the contract intent is host behavior, not OS config-dir spelling.
- Fix: normalize both Reasonix user config directories to `$REASONIX_CONFIG` before golden comparison and refresh `native_install_contract_matrix.golden.json`.
- Verification: `go test ./internal/adapter -count=1`; `go test ./... -count=1`; rerun GitHub Actions CI on the updated branch.
