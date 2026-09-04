# 2026-07-01 — IssueOps devil's-advocate is a fail-closed loop, not just skill prose

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: issueops devil's-advocate loop (spec/plan 2026-07-01)
- Summary: The design-review devil's advocate becomes a machine-enforced gate — implement entry requires a recorded verdict (pass, or a stop/revise explicitly waived), and a `stop`'s findings must be reflected into the remote issue before `regress` rewinds to grill.
- Context: design-review was only skill-mandated (SKILL prose "MUST run"); the state machine had no invariant that it ran, and a `stop`'s findings stayed local (regress recorded a scope decision and never touched the remote issue). The loop was advisory, not enforced.
- Decision: (1) Add a first-class `IssueOpsDevilsAdvocateReview` record (verdict pass|revise|stop, findings, waiver, IssueReflectedAt) mirroring the design/compatibility review pattern. (2) `IssueOpsImplementationReadiness` fails closed on a missing review or an unwaived stop/revise (missing key `devils_advocate_review`). (3) A new `UpdateIssueBodySection` provider method (github/gitlab) idempotently splices findings into a delimited issue-body section; `issueops remote reflect-devils-advocate` writes it and stamps `IssueReflectedAt`. (4) `regress` requires a recorded stop whose findings were reflected, and clears the review so the re-planned cycle earns a fresh verdict.
- Consequences: The new record field, two CLI subcommands (`devils-advocate review`, `remote reflect-devils-advocate`), two MCP tools (`issueops_record_devils_advocate_review`, `issueops_remote_reflect_devils_advocate`), and the provider interface method are public contract; mcp_tools/response goldens were regenerated. The enforced loop is stop → reflect → regress → re-plan → fresh verdict.
- Alternatives / rejected options:
  - Keep design-review skill-only — rejected: machine enforcement is the point (the loop-engineering ask).
  - Extend `ValidateArtifactURL` for issue verification — rejected as dead code; reflection uses `UpdateIssueBodySection`, not the pr/mr verify chain.
  - Have `regress` write the issue itself — rejected: keeping regress local/pure and ordering the steps (record → reflect → regress) is a cleaner fail-closed sequence.
