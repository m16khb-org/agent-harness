# 2026-07-07 — Standalone harness policy, upstream wiring removed

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: codex Task 22 independence policy docs update from codex-orchestration implementation plan
- Summary: Native install, update, readiness, and self-verification paths are standalone; upstream companion tool wiring is removed from the harness contract.
- Context: Earlier architecture allowed opt-in wiring for LLM Wiki, CodeGraph, claude-mem, LazyCodex, and similar tools. That made clean-machine reproduction harder, pulled external accounts/keys/network/tool drift into core paths, and encouraged compatibility shims around third-party plugin caches. Task 18 removed the unused external LLM port abstraction, Task 19 made draft-wiki promotion a local export, Task 20 removed upstream installer wiring, and Task 21 replaced Codex process spawning with a host-agent prompt/result contract.
- Decision: `agent-harness install`, `bootstrap`, `update`, `scripts/install-native.sh`, readiness gates, and self-verification must succeed using only repo-local code, user-level harness config, and explicit local fixtures. External tools can still be used by a user outside the harness, but agent-harness consumes them only through ordinary inspectable boundaries such as local files, command output, or already-configured MCP data. The harness does not install, patch, register, or require third-party toolchains.
- Rationale: Core paths need independence, reproducibility, and no external keys/accounts. A standalone contract makes CI, clean-machine install, user support, and host parity testable without inheriting external tool lifecycle failures.
- Consequences: Future features must not add third-party installers, companion hook patchers, external MCP registration, or external-tool-required readiness checks to native install/update/self-verify. Documentation should describe external tools as optional user environment context, not dependencies.
- Evidence:
  - scripts/install-native.sh removed upstream install flags and helper paths
  - internal/adapter/install_contract_matrix_test.go asserts companion installer paths stay absent
  - internal/core/draftwiki/draft_wiki_promote.go exports approved drafts locally
  - cmd/harness/apidoc/api_doc_review_runner.go renders host-agent prompt/schema and records result files without spawning Codex
- Alternatives / rejected options:
  - Keep opt-in upstream wiring — rejected because it preserves network/account/tool-version drift in the harness support surface and revives removed flags.
  - Keep compatibility shims that patch companion plugin caches — rejected because agent-harness would own another project's lifecycle without owning its contract.
  - Reimplement external tool features in core — rejected because it expands harness scope and duplicates specialized tools instead of preserving a small host-neutral core.
