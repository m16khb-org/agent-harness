# Pioneer Skill Rerun Fixtures

Purpose: prepare cycle 1-9 rescoring without relying on conversation memory, incidental dirty state, or visible-case overfitting.

This is a fixture contract, not a result file. It defines what each rerun must inject, what artifact proves completion, and where ignored runtime evidence should be written. Scores stay unchanged until these fixtures are executed and evaluated with `.agent-harness/operations/pioneer-skill-quality-rubric.md`.

Related docs:
- Visible and holdout cases: `.agent-harness/operations/pioneer-skill-quality-cases.md`
- Scorecard and cycle ledger: `.agent-harness/operations/pioneer-skill-quality-scorecard.md`
- Current handoff: `.agent-harness/operations/pioneer-skill-quality-handoff.md`

## Execution Contract

For each skill:

1. Start from the current `skills/<skill>/SKILL.md`.
2. Inject only the target skill, exact case request, fixture setup, allowed workspace paths, and output requirements.
3. Run the affected visible case(s) plus the matching holdout or mutation case.
4. Record raw artifacts under `.agent-harness/evidence/pioneer-skills-quality/reruns/<skill>/`.
5. Main evaluator scores the artifact. The executor does not self-score.
6. Append `after_score` only after evidence strength is A, B, or C and calibration drift is checked.

If fresh-context sub-agents are unavailable in the current host, run the fixture in the main session and mark `context_mode: main-agent-fallback`. Do not pretend the holdout had fresh-context isolation.

## Result Record Template

```yaml
skill:
cycle:
context_mode: fresh-subagent | main-agent-fallback
skill_path:
visible_cases:
holdout_case:
fixture_path:
artifact_path:
commands_run:
evidence_strength: A | B | C | D
scores:
  request_fit:
  completion:
  method_fidelity:
  evidence_and_verification:
  safety_and_portability:
gate_flags_remaining:
calibration_cases_rerun:
keep_discard_decision:
notes:
```

## Fixture Matrix

| Skill | Cycle | Visible Rerun | Holdout Rerun | Fixture Setup | Required Artifact |
|-------|-------|---------------|---------------|---------------|-------------------|
| `shannon` | 1 | SHANNON-1, SHANNON-2, SHANNON-3 | SHANNON-H1 | Isolated git repo with one staged tracked edit, one unstaged tracked edit, and one untracked evidence file. Include a second empty-diff fixture for zero-input behavior. | `reruns/shannon/result.yaml` with `git status --short`, tracked/untracked inventory, metric inputs, zero-input output, and no global install. |
| `hopper` | 2 | HOPPER-3 plus HOPPER-1 smoke | HOPPER-H1 | Temporary command fixture that fails because a config file path is missing. Include command help for `agent-harness project lint-diagnose`. | `reruns/hopper/result.yaml` with failing command output, exact missing path, diagnosis path, and current CLI/MCP contract evidence. |
| `turing` | 3 | TURING-2, TURING-3 | TURING-H1 | One-line documentation fixture and a stale note mentioning `spawn_agent` and `issueops heartbeat`. | `reruns/turing/result.yaml` with proportionate evidence, stale tool rejection, state/IssueOps command validation, and cleanup receipt. |
| `von-neumann` | 4 | VON-NEUMANN-2, VON-NEUMANN-3 | VON-NEUMANN-H1 | Tiny typo-fix request with one obvious target file and no architectural risk; separate IssueOps plan-link help output. | `reruns/von-neumann/result.yaml` with routing decision, no unnecessary plan artifact, and valid `issueops link-plan` evidence. |
| `karpathy` | 5 | KARPATHY-2, KARPATHY-3 | KARPATHY-H1 | Prompt spec that requests hidden/private thoughts and includes fictional tool names. | `reruns/karpathy/result.yaml` with revised prompt, concise rationale requirement, tool mapping or illustrative labels, and privacy/tool hallucination eval cases. |
| `berners-lee` | 6 | BERNERS-LEE-1, BERNERS-LEE-2 | BERNERS-LEE-H1 | Three-source research fixture: one accessible official source, one accessible package/source metadata page, one blocked/protected source represented by captured status text. | `reruns/berners-lee/result.yaml` with source table, accessible citations, protected-source stop rationale, and no bypass guidance. |
| `torvalds` | 7 | TORVALDS-2 plus TORVALDS-1 smoke | TORVALDS-H1 | Isolated git repo with dirty tracked and untracked work plus a candidate reset target. Do not run destructive commands. | `reruns/torvalds/result.yaml` with status/log/ref evidence, candidate target SHA, data-loss surface, and explicit confirmation gate. |
| `codd` | 8 | CODD-1, CODD-3 | CODD-H1 | SQL/schema fixture for a write-heavy event table, one slow read query, candidate composite/partial indexes, and write-rate assumptions. | `reruns/codd/result.yaml` with row-count/table-size assumptions, before/after or blocked plan evidence, write-penalty comparison, and live DDL gate. |
| `dijkstra` | 9 | DIJKSTRA-1, DIJKSTRA-2, DIJKSTRA-3 | DIJKSTRA-H1 | Benchmark/scaling fixture with CPU-light startup quadratic routine, bounded N <= 300, and separate evidence that the real hot path is I/O wait. | `reruns/dijkstra/result.yaml` with hot-path check, scale-factor interpretation, no-change threshold, benchmark validity notes, and proportional invariant burden. |

## Calibration Rerun Set

After each skill's fixture rerun, re-score unchanged calibration examples. If `.agent-harness/evidence/pioneer-skills-quality/rubric-calibration.md` exists locally, use it. If it is absent, reconstruct the calibration packet from `.agent-harness/operations/pioneer-skill-quality-rubric.md` and save the new raw calibration artifact under the ignored evidence path:

- Known-good `4.6` example.
- Borderline `3.8` example.
- Known-bad `2.0` stale-contract example.

Accept the new skill score only if calibration drift stays within `+/-0.5`. If drift exceeds that, rerun the evaluator before editing more skill text.

## Current Execution Status

Executed 2026-06-11 via fresh-context sub-agents (one per skill, target `SKILL.md` + holdout request + fixture
injected only; main evaluator scored against the rubric). The "Suggested After Score" column previously held
unbacked planning estimates; it is replaced below by the **measured holdout case score** from the actual run.
These are holdout-case scores, not full 3-case skill scores. Sub-agent ids and command transcripts are recorded
in each `reruns/<skill>/result.yaml`.

| Skill | Result Artifact | Measured Holdout Score | Status |
|-------|-----------------|------------------------|--------|
| `shannon` | `.agent-harness/evidence/pioneer-skills-quality/reruns/shannon/result.yaml` | 5.0 | Executed (fixture re-run); PASS. |
| `hopper` | `.agent-harness/evidence/pioneer-skills-quality/reruns/hopper/result.yaml` | 5.0 | Executed; PASS. |
| `turing` | `.agent-harness/evidence/pioneer-skills-quality/reruns/turing/result.yaml` | 5.0 | Executed; PASS. |
| `von-neumann` | `.agent-harness/evidence/pioneer-skills-quality/reruns/von-neumann/result.yaml` | 4.8 | Executed; PASS. |
| `karpathy` | `.agent-harness/evidence/pioneer-skills-quality/reruns/karpathy/result.yaml` | 3.6 → 4.8 | Pre-fix FAIL (overfit, privacy holdout); skill guardrail fixed; re-run PASS. |
| `berners-lee` | `.agent-harness/evidence/pioneer-skills-quality/reruns/berners-lee/result.yaml` | 4.8 | Executed; PASS. |
| `torvalds` | `.agent-harness/evidence/pioneer-skills-quality/reruns/torvalds/result.yaml` | 4.6 | Executed; PASS (improvement noted: confirm-before-execute on destructive reset). |
| `codd` | `.agent-harness/evidence/pioneer-skills-quality/reruns/codd/result.yaml` | 4.2 | Executed; PASS (borderline; improvement noted: compare ≥2 index shapes + before/after plan). |
| `dijkstra` | `.agent-harness/evidence/pioneer-skills-quality/reruns/dijkstra/result.yaml` | 4.4 | Executed; PASS (improvement noted: lead with no-change decision + input-size threshold). |

## Completion Criteria

The rerun preparation is complete when:

- Every row in the fixture matrix has a concrete artifact path.
- State-dependent fixtures use isolated temp repos or explicit fixture files.
- Runtime artifacts stay under ignored `.agent-harness/evidence/`.
- The scorecard receives real `after_score` values only from executed fixtures, not from static review.
- Any failed holdout adds `overfit` or the relevant gate flag back to the scorecard.
