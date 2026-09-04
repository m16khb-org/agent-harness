# 2026-07-02 — External LLM stays Z.AI-only until a second provider is real

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: quality optimization Track 1 L1 cleanup
- Summary: Delete the unused `internal/port.ExternalLLM` interface and keep external LLM invocation as the concrete Z.AI wrapper in `internal/core/externalllm`.
- Context: `internal/port/externalllm.go` defined `ExternalLLM`, but no implementation or caller used it. Real call sites (`commitsuggest`, `draftwiki`, `issueops` scoring/benchmark, `lintdiagnose`, `nextaction`, self-verify LLM eval) call `internal/core/externalllm.RunExternalLLMPrint`, which already hard-rejects non-Z.AI providers. Keeping the orphan interface made the audit appear solved by an abstraction that did not exist in the runtime path.
- Decision: Remove `port.ExternalLLM`. Do not add a replacement provider abstraction in this pass. Treat Z.AI-only as an explicit YAGNI decision until a second provider is required by a real caller and can be tested through the same CLI/MCP contracts.
- Consequences: There is no CLI/MCP public contract change. Provider extensibility is intentionally deferred; future provider work must introduce the abstraction together with a concrete second implementation, tests, and an ADR update.
- Alternatives / rejected options:
  - Wire existing call sites through `port.ExternalLLM` now — rejected because there is only one provider and no observed variation point.
  - Keep the unused interface as a placeholder — rejected because it falsely signals decoupling and is not exercised by tests or runtime.
