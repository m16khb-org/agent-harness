# Pioneer Skill Quality Scorecard

This scorecard applies `.agent-harness/operations/pioneer-skill-quality-rubric.md`.

Status: 27-case baseline is complete and the holdout/mutation suite is defined. Quality is not complete because holdout/mutation cases have not yet been executed after improvements.

Rubric calibration: `.agent-harness/evidence/pioneer-skills-quality/rubric-calibration.md` separates known-good (`4.6`), borderline (`3.8`), and known-bad (`2.0` with `stale-contract`) examples. This means the strengthened rubric is fit for the next 27-case baseline pass, as long as every case records pre-score critical checks and evidence strength.

Holdout suite: `.agent-harness/evidence/pioneer-skills-quality/holdout-mutation-suite.md` defines one anti-gaming holdout/mutation case per skill. These cases are designed but not yet executed as final gates. Preferred execution mode is a fresh-context sub-agent with only the target skill, case request, and required fixture path injected, followed by main-evaluator scoring. SHANNON-H1 smoke testing proved two things: state-dependent cases need explicit fixtures, and a fixture-backed fresh-context sub-agent run can return a scoreable artifact without leaking the evaluator's baseline notes.

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

Detailed results: `.agent-harness/evidence/pioneer-skills-quality/baseline-27-case-results.md`

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

Current quality status: not quality-complete. Only `codd` exceeds the numeric skill target, and even it still lacks holdout/mutation proof. The family still has `unsafe`, `stale-contract`, `fake-tool`, `overbroad`, and `non-repeatable` blockers. The evaluation criteria are now stronger because each skill has a named anti-gaming holdout, but those holdouts must still be executed after fixes.

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

## Improvement Priority From Current Baseline

1. `shannon`: fix untracked/staged/unstaged measurement, zero-input guard, and global install default.
2. `hopper`: replace stale `--command-argv` CLI form with `project lint-diagnose --json -- <command...>`.
3. `turing`: remove or replace `issueops heartbeat`, `remove-ai-slops`, stale `state write <key> <content>`, and unavailable host tool names.
4. `von-neumann`: remove nonexistent `agent-harness von-neumann plan` claim and narrow activation boundary.
5. `karpathy`: remove user-visible chain-of-thought guidance and label tool schemas as illustrative unless mapped to the current host.
6. `berners-lee`: gate fetch escalation and replace host-specific `web_fetch` assumptions with current-host translation.
7. `torvalds`: add explicit destructive recovery confirmation ladder and non-interactive rebase fallback.
8. `codd`: split dense reference material and add explicit write-penalty output.
9. `dijkstra`: fix scaling-test explanation and add no-change response template.

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
