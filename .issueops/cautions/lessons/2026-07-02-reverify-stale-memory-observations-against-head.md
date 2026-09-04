---
name: cautions/lessons/2026-07-02-reverify-stale-memory-observations-against-head.md
description: Dated lesson — re-verify stale memory observations against HEAD before treating them as plan gaps.
---

# 2026-07-02 — Re-verify stale memory observations against HEAD before planning gaps

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: article-insights improvement plan design-review review
- Summary: A claude-mem observation (Jun 11, #14024) claiming self-verify promote skips snapshot validation was used as a plan gap, but commit 09fcb7c had already implemented the fail-closed gate; the stale claim survived into plan v1 and was only caught by adversarial review.
- Context: 2026-07-02 article-insights improvement plan v1 proposed P0-3 "promote snapshot validation" based on a remembered observation. Brooks devil's-advocate review falsified it: cmd/issueops/selfworkflow/stateio/self_verify_promote_core.go:35-44 already checks snapshot.OK && Summary.TerminationEligible and refuses promotion without --allow-failed-source (commit 09fcb7c).
- Resolution: Before promoting any memory/observation-sourced claim into a plan gap or work item, re-verify it against HEAD with direct evidence (git log on the file, read the current implementation). Treat dated observations as hypotheses, not facts; record the verification command alongside the gap.
- Evidence:
  - git log --oneline -- cmd/issueops/selfworkflow/stateio/self_verify_promote_core.go → 09fcb7c
  - cmd/issueops/selfworkflow/stateio/self_verify_promote_core.go:35-44

> Incident-time command, field, and state references are historical evidence, not current execution directives.
