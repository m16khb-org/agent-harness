# 2026-07-02 — Self-augment planner consumes Reflexion lessons as score penalty

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: article-insights improvement plan T1
- Summary: The self-augment planner now loads self-augment-lesson-* state snapshots and demotes candidates with repeated severe failure lessons via a score penalty, instead of leaving lessons write-only.
- Context: 2026-07-02 article-insight plan (LINE multi-agent oscillation/warning-label mechanisms, Introspection trace-to-pattern loop) cross-checked against the repo showed self_augment_lesson snapshots are recorded but never consumed by augmentplan; only success-side satisfaction rules feed back into the curriculum. Brooks devil's-advocate review confirmed this gap is real (plan.go loads no lesson snapshot) and trimmed the scope to score-penalty demotion only.
- Decision: Consume lessons in the planner as an advisory score penalty: candidates accumulating severe lessons lose curriculum rank but are never auto-failed or removed (LINE "advisory signal, not judgement" principle). No new lesson DTO fields (Category/RootCauses/SeenCount/Confidence deferred until a second consumer exists). This differs from the KILLed tool-error context injection (2026-07-01): that was an in-session PostToolUse static nudge duplicating host inline display; this is cross-run Reflexion memory feeding planner candidate scoring — different consumer, loop, and layer, so the KILL rationale does not apply.
- Consequences: Planner reads lesson state at plan time; repeated failed attempts on one candidate naturally rotate the curriculum to the next best candidate. Score penalty parameters live in augmentcatalog. Golden contracts unchanged unless plan DTO shape changes.
- Evidence:
  - cmd/issueops/selfworkflow/augmentplan/plan.go (no lesson consumption before this change)
  - cmd/issueops/selfworkflow/augmentlesson/self_augment_lesson_state.go: self-augment-lesson-<slug>-<ts> state keys
  - .omc/plans/2026-07-02-article-insights-harness-improvement-plan.md (v2, design-review-reviewed)
- Alternatives / rejected options:
  - Warning-label schema with new DTO fields (Category/RootCauses/SeenCount/Confidence) rendered into plan output — rejected as second-system: 2 of 3 intended consumers (pattern report, pattern-to-judge) do not exist yet
  - Auto-fail/remove repeatedly failing candidates — rejected: violates the advisory-signal safeguard; past patterns may prompt extra scrutiny but must not auto-fail current work
  - A→B→A findings-signature oscillation detection — deferred: requires normalized-hash infrastructure with no observed occurrence evidence
