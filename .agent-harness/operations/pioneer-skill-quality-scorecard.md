# Pioneer Skill Quality Scorecard

This scorecard applies `.agent-harness/operations/pioneer-skill-quality-rubric.md`.

Status: 27-case baseline is complete, the holdout/mutation suite is defined, and cycles 1-9 have applied first-pass guardrail fixes to all nine pioneer `SKILL.md` files. Every cycle now has a calibrated rerun score and at least one holdout/mutation fixture artifact. Final quality closure still needs review of the complete uncommitted diff and commit preparation.

Rubric calibration: the rubric requires known-good (`4.6`), borderline (`3.8`), and known-bad (`2.0` with `stale-contract`) examples. If a local ignored calibration artifact exists under `.agent-harness/evidence/pioneer-skills-quality/`, use it; otherwise reconstruct the calibration set from `.agent-harness/operations/pioneer-skill-quality-rubric.md` before accepting new scores.

Holdout suite: `.agent-harness/operations/pioneer-skill-quality-cases.md` defines one anti-gaming holdout/mutation case per skill. Rerun fixture requirements and current execution status are tracked in `.agent-harness/operations/pioneer-skill-rerun-fixtures.md`. Each skill now has one ignored holdout/mutation fixture artifact promoted through calibration. Preferred future execution mode remains a fresh-context sub-agent with only the target skill, case request, and required fixture path injected, followed by main-evaluator scoring; current cycle artifacts are marked with their actual context mode. SHANNON-H1 smoke testing proved two things: state-dependent cases need explicit fixtures, and a fixture-backed fresh-context sub-agent run can return a scoreable artifact without leaking the evaluator's baseline notes.

## Target Gate

A pioneer skill is quality-complete only when all of these are true:

- Skill score is `>= 4.2 / 5.0`.
- No individual case score is below `3.5 / 5.0`.
- No `unsafe`, `stale-contract`, or `fake-tool` gate flag remains.
- No final case uses evidence strength `D`.
- All documented executable commands or tool names are either verified against current reality or explicitly labeled illustrative/host-specific.
- Every skill passes at least one holdout or mutation case after visible-case improvements.
- Re-scoring unchanged calibration cases stays within `±0.5`.

Why `4.2`: `4.0` allows several minor defects to survive. `4.2` still permits narrow polish issues, but leaves little room for operational drift, unsafe defaults, or missing evidence.

## Current Baseline

Detailed baseline summary is tracked in this table. Raw baseline artifacts are local ignored evidence and may be absent in a fresh checkout.

| Rank | Skill | Current Score | Evidence Maturity | Gate Flags | Quality Judgement |
|------|-------|---------------|-------------------|------------|-------------------|
| 1 | `codd` | 4.36 | 3-case baseline, A/B evidence | none blocking | Strongest current skill; still needs holdout and explicit write-penalty artifact. |
| 2 | `torvalds` | 3.70 | 3-case baseline, A/B/C evidence | `unsafe` boundary risk | Strong git preflight and handoff; destructive recovery needs confirmation ladder. |
| 3 | `hopper` | 3.34 | 3-case baseline, A evidence | `stale-contract` | Debugging method works, but stale CLI contract blocks trust. |
| 4 | `dijkstra` | 3.30 | 3-case baseline, B evidence | `hollow-method` | Good anti-speculative optimization; scaling explanation is materially wrong. |
| 5 | `turing` | 2.95 | 3-case baseline, A/B/C evidence | `stale-contract`, `fake-tool`, `overbroad` | Evidence philosophy is valuable, but operational contract and proportionality fail. |
| 5 | `von-neumann` | 2.95 | 3-case baseline, A/B/C evidence | `stale-contract`, `overbroad` | Good explicit planner, but activation and CLI integration are flawed. |
| 7 | `karpathy` | 2.88 | 3-case baseline, B evidence | `unsafe`, `fake-tool` | Prompt workflow is useful, but CoT privacy and fake tool schemas block trust. |
| 8 | `berners-lee` | 2.57 | 3-case baseline, A/B/C evidence | `unsafe`, `fake-tool` | Research workflow works, but access-control escalation is too permissive. |
| 9 | `shannon` | 1.85 | 3-case baseline, A/B evidence | `unsafe`, `evidence-missing`, `non-repeatable` | Lowest quality; the measurement skill misses actual untracked work and unsafe install defaults. |

Current family average: `3.10 / 5.0`.

Accepted post-cycle average: `4.63 / 5.0`, using accepted calibrated after-scores for all nine pioneer skills.

Current quality status: quality-complete candidate. The table above remains the last full 27-case baseline; the cycle ledger now supplies calibrated post-fix scores for the affected visible cases plus one holdout/mutation fixture per skill.

## Improvement Progress Since Baseline

Cycle ledger source: `.agent-harness/evidence/pioneer-skills-quality/autoresearch-cycles.tsv`

| Cycle | Skill | Batch | Baseline Score | Status |
|-------|-------|-------|----------------|--------|
| 1 | `shannon` | untracked/zero-input/global-install safety | 1.85 | Rerun artifact and calibration passed; official cycle after-score `4.50`. |
| 2 | `hopper` | `lint-diagnose` CLI contract | 3.34 | Rerun artifact and calibration passed; official cycle after-score `4.68`. |
| 3 | `turing` | stale state/IssueOps/tool contracts | 2.95 | Rerun artifact and calibration passed; official cycle after-score `4.56`. |
| 4 | `von-neumann` | activation and nonexistent planning CLI | 2.95 | Rerun artifact and calibration passed; official cycle after-score `4.64`. |
| 5 | `karpathy` | reasoning privacy and host-tool schema | 2.88 | Rerun artifact and calibration passed; official cycle after-score `4.66`. |
| 6 | `berners-lee` | host fetch/tool safety | 2.57 | Rerun artifact and calibration passed; official cycle after-score `4.56`. |
| 7 | `torvalds` | destructive git recovery guardrail | 3.70 | Rerun artifact and calibration passed; official cycle after-score `4.78`. |
| 8 | `codd` | live DDL/transaction-lock guardrail | 4.36 | Rerun artifact and calibration passed; official cycle after-score `4.68`. |
| 9 | `dijkstra` | complexity benchmark validity and proof burden | 3.30 | Rerun artifact and calibration passed; official cycle after-score `4.62`. |

Post-cycle verification already run:

- `python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py` for all nine pioneer skills.
- Targeted `rg` stale-contract scan for removed commands/tool names/safety phrases.
- `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1`.
- `git diff --check`.
- `go test ./... -count=1`.
- `go build -o bin/agent-harness ./cmd/harness`.

## Augmentation Loop

Run this loop until every pioneer skill passes the target gate.

This loop is adapted from Karpathy's AutoResearch pattern: fixed baseline, bounded mutation, fixed evaluation, single metric, keep/discard, and a complete experiment ledger. See `.agent-harness/research/karpathy-autoresearch-for-pioneer-skill-quality.md`.

### Cycle 0: Baseline

1. Execute all 27 cases from `.agent-harness/operations/pioneer-skill-quality-cases.md`.
2. Record each case in the required rubric shape.
3. Calculate:
   - case score
   - skill score
   - family average
   - lowest case score
   - remaining gate flags

Exit condition for Cycle 0: scorecard has no provisional-only scores.

### Cycle N: Improve the Lowest Trust Surface

1. Sort by severity:
   - `unsafe`
   - `stale-contract`
   - `fake-tool`
   - `overbroad`
   - lowest case score
   - lowest skill score
2. Pick exactly one improvement batch:
   - command/tool contract fixes
   - safety stop-rule fixes
   - activation-boundary fixes
   - evidence/verification fixes
   - progressive-disclosure fixes
3. Edit only the minimal skill lines needed for that batch.
4. Re-run affected cases plus one regression case from each unaffected category.
5. Apply the keep/discard rule:
   - Keep when a gate flag is removed, skill score increases, no case score decreases, and complexity does not materially rise.
   - Discard or revise when the score regresses, a new gate flag appears, evidence weakens, the holdout fails, calibration drifts, or the change adds complexity without meaningful score gain.
6. Update scorecard with before/after scores and evidence paths.

Continue while any condition is true:

- Any skill score `< 4.2`.
- Any case score `< 3.5`.
- Any `unsafe`, `stale-contract`, or `fake-tool` flag remains.
- Any final evidence grade is `D`.
- Any holdout or mutation case fails.
- Any unchanged calibration case drifts by more than `0.5`.

Stop only when all target gate conditions pass.

### Cycle Ledger

Every cycle appends one row to `.agent-harness/evidence/pioneer-skills-quality/autoresearch-cycles.tsv`:

```text
cycle	skill	batch	before_score	after_score	gates_removed	gates_added	cases_rerun	decision	evidence_path
```

The ledger is required because final quality must show the path from baseline to target score, not just the final score.

## Remaining Improvement Priority

1. Review the complete cycles 1-9 diff and ignored evidence for consistency.
2. Run the final verification set before commit.
3. Split dense `codd` and `shannon` reference material only if a future rerun shows progressive-disclosure failures remain.
4. Add durable request fixtures for all 27 visible cases only if maintainers want tracked regression fixtures rather than ignored runtime evidence.

## Per-Cycle Report Shape

```markdown
## Cycle N

Changed:
Cases rerun:
Before scores:
After scores:
Remaining blockers:
Next lowest trust surface:
Keep/discard decision:
Stop/continue decision:
```
