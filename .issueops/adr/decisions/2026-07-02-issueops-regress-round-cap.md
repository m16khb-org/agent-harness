# 2026-07-02 — IssueOps regress rounds are capped with a human-decision escalation

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: article-insights improvement plan T2
- Summary: RegressIssueOpsForReplan now appends an IssueOpsRegressEvent audit entry per successful regression and refuses fail-closed once a cycle reaches 3 stop-to-re-plan rounds, demanding a human decision instead of another automatic re-plan.
- Context: The 2026-07-02 article-insight plan (LINE multi-agent: repeated-failure loops waste tokens and need escalation to a differently-scoped reviewer) cross-checked with the repo showed the stop→reflect→regress→re-plan loop had no round counter or upper bound. Brooks review trimmed the original proposal (A→B→A findings-signature oscillation detection) to a count cap only, since each round already requires a fresh stop verdict plus remote reflection and no signature-oscillation occurrence has been observed.
- Decision: Add RegressEvents ([]{reason, from_phase, at}, omitempty) to IssueOpsRecord as the audit trail; cap successful regressions at issueOpsRegressCap=3 per cycle. At the cap the regress is refused before any mutation (phase, ledger, and events untouched) with an error stating the plan is thrashing and a human decision is required. Phase judgement stays a pure reducer; the cap lives in the regress wrapper action alongside the existing stop/reflect preconditions.
- Consequences: Cycles that hit the cap require deliberate human action (e.g. force-release, re-scope, or a fresh cycle). RegressEvents is omitempty so existing records and response goldens are unaffected until a regression occurs.
- Evidence:
  - internal/core/issueops/issueops_regress.go (cap + event append)
  - internal/core/issueops/model/types.go (IssueOpsRegressEvent, RegressEvents)
  - internal/core/issueops/issueops_regress_cap_test.go (below-cap allowed, at-cap refused without mutation)
- Alternatives / rejected options:
  - A→B→A findings-signature oscillation detection — deferred: needs normalized-hash infrastructure with no observed occurrence evidence
  - Unbounded regressions (status quo) — rejected: stop→regress rounds can thrash indefinitely, burning tokens without converging
  - Auto-closing the cycle at the cap — rejected: destructive; the cycle stays intact so a human can waive, re-scope, or abandon deliberately
