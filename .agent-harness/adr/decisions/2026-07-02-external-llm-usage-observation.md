# 2026-07-02 — External LLM calls emit per-call usage observation records

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: article-insights improvement plan T3
- Summary: Every successful RunExternalLLMPrint call now parses the provider usage block and appends an external-llm-usage-* state record (provider, model, tokens, duration) via a recorder hook wired in core root; recording is best-effort and can never block the call.
- Context: The 2026-07-02 article-insight plan T3 (Introspection: trace every run with turns/tokens/cost; Sonnet 5 tokenizer change showed model-side cost shifts are invisible without usage accounting) found the harness had zero usage observability. Brooks review trimmed scope to append-only observation records, deferring compare regression rules until baselines accumulate.
- Decision: Parse the OpenAI-compatible usage block into ExternalLLMPrintResult (Model, DurationMS, Usage) in the externalllm package, which stays state-free; a package-level usage recorder hook defaults to no-op and core root wires it to a StateWrite-backed writer in init(). This observes every production caller (leaf packages call externalllm directly, but every shipped binary links core root) while leaf-package unit tests record nothing. Keys use nanosecond suffixes (lesson key-collision lesson applied). All recorder failure paths return silently — observation must never fail the observed call. Z.AI-only scope per the existing YAGNI ADR.
- Consequences: Usage history accumulates as state records for future pattern aggregation (P1-2) and long-loop stability metrics (P2-3). Tests exercising the LLM path through core root must isolate HARNESS_STATE_DIR (llmeval helper updated).
- Evidence:
  - internal/core/externalllm/usage.go, print.go (usage parse + recorder hook)
  - internal/core/external_llm_usage.go (init wiring + best-effort writer)
  - internal/core/external_llm_usage_test.go (record written; broken state dir does not block the call)
- Alternatives / rejected options:
  - Recording inside the core facade only — rejected: most production callers (benchmark judge, remote judge, lintdiagnose, draftwiki, commitsuggest, nextaction) call externalllm directly and would be unobserved
  - Recording unconditionally inside externalllm — rejected: externalllm would depend on state, and every leaf-package unit test would write into the developer's real ~/.local/state dir
  - token_usage compare regression rules now — deferred until real baselines accumulate (brooks trim)
