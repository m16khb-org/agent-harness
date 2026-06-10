# Pioneer Skill Quality Handoff

Date: 2026-06-10

Purpose: preserve the current stopping point so the pioneer skill quality work can resume without relying on conversation memory.

## Current State

The work paused after strengthening the evaluation criteria, not after editing any pioneer skill bodies.

Committed scope should remain documentation and evidence summaries only:

- No `skills/<name>/SKILL.md` body fixes have been applied.
- No Go source changes have been made.
- The current work defines a stronger qualitative rubric, 27 visible baseline cases, one holdout/mutation case per skill, and a fresh-context sub-agent evaluation protocol.
- The first fresh-context smoke was run for `shannon` only.

## Baseline Scores

The calibrated 27-case visible baseline produced these current scores:

| Rank | Skill | Score | Main Blocker |
|------|-------|-------|--------------|
| 1 | `codd` | 4.36 | Needs holdout proof and clearer write-penalty artifact. |
| 2 | `torvalds` | 3.70 | Destructive recovery path needs an explicit confirmation ladder. |
| 3 | `hopper` | 3.34 | Stale `--command-argv` lint-diagnose contract. |
| 4 | `dijkstra` | 3.30 | Scaling explanation and no-change template need correction. |
| 5 | `turing` | 2.95 | Stale commands/tools and over-heavy verification path. |
| 5 | `von-neumann` | 2.95 | Overbroad activation and nonexistent planning CLI claim. |
| 7 | `karpathy` | 2.88 | Hidden reasoning privacy issue and illustrative fake tool names. |
| 8 | `berners-lee` | 2.57 | Fetch escalation safety and host-specific tool assumptions. |
| 9 | `shannon` | 1.85 | Misses untracked work, zero-input guard, and unsafe install default. |

Family average: `3.10 / 5.0`.

Quality status: not complete.

Target gate:

- Every skill score `>= 4.2 / 5.0`.
- No individual case below `3.5 / 5.0`.
- No `unsafe`, `stale-contract`, or `fake-tool` gate.
- No final evidence grade `D`.
- Every skill passes at least one holdout/mutation case after visible-case fixes.
- Calibration drift stays within `+/-0.5`.

## Fresh-Context Sub-Agent Protocol

The key improvement to the evaluator is this protocol:

1. Main evaluator selects one target skill and one holdout case.
2. Main evaluator prepares any required fixture first.
3. Main evaluator starts a sub-agent with `fork_context: false`.
4. Main evaluator injects only the target `SKILL.md`, exact request, allowed workspace/fixture path, and output requirements.
5. Sub-agent returns artifact, evidence, commands, files touched, and blockers.
6. Sub-agent does not self-score.
7. Main evaluator scores the artifact using the rubric.
8. Result record must include sub-agent id, context-fork status, injected skill path, fixture setup, observed artifact, evidence, evaluator score, and keep/discard decision.

This prevents the score from depending on hidden main-context knowledge.

## Fresh-Context Smoke Results

Two smoke runs were completed for `SHANNON-H1`.

First smoke:

- Sub-agent id: `019eb084-4ba0-7321-a812-2ca9b5bf4167`
- Context forked: false
- Fixture: none; current repo state was used.
- Result: incomplete, but useful. The sub-agent correctly reported that the current repo did not contain the requested staged plus unstaged fixture.
- Evaluation consequence: state-dependent holdouts must use explicit fixtures.

Second smoke:

- Sub-agent id: `019eb085-8cb1-7751-b95b-ef28dca1d1e2`
- Context forked: false
- Fixture: `/tmp/pioneer-shannon-holdout.PsWc6x`
- Fixture state: isolated git repo with `MM tracked.md` and `?? evidence.md`.
- Result: pass as protocol smoke. The sub-agent separated staged tracked change, unstaged tracked change, and untracked evidence file, and returned scoreable evidence without self-scoring.
- Evaluation consequence: keep the fresh-context sub-agent protocol and fixture requirement.

This was not a final `shannon` quality pass because the fixture used simple text files, not realistic code for entropy/redundancy metrics, and no skill body fix has been applied.

## Files Added Or Updated

Tracked documentation intended for commit:

- `.agent-harness/operations/pioneer-skill-live-invocation-summary.md`
- `.agent-harness/operations/pioneer-skill-quality-cases.md`
- `.agent-harness/operations/pioneer-skill-quality-rubric.md`
- `.agent-harness/operations/pioneer-skill-quality-scorecard.md`
- `.agent-harness/operations/pioneer-skill-quality-handoff.md`
- `.agent-harness/plans/pioneer-skills-quality-improvement.md`
- `.agent-harness/research/karpathy-autoresearch-for-pioneer-skill-quality.md`

Local ignored evidence files:

- `.agent-harness/evidence/pioneer-skills-quality/autoresearch-cycles.tsv`
- `.agent-harness/evidence/pioneer-skills-quality/baseline-27-case-results.md`
- `.agent-harness/evidence/pioneer-skills-quality/holdout-mutation-suite.md`
- `.agent-harness/evidence/pioneer-skills-quality/rubric-calibration.md`
- `.agent-harness/evidence/pioneer-skills-quality/task-0-live-invocation-record.md`
- `.agent-harness/evidence/pioneer-skills-quality/task-0-request-fulfillment-evaluation.md`

Those evidence files are under an ignored `evidence` path. The tracked handoff and scorecard summarize the parts needed to resume without committing ignored runtime evidence.

Draft-wiki material queued:

- `dwq-43dcf1dd6ac71c7c51009a10`

## Recommended Resume Order

1. Re-open the rubric, cases, scorecard, and this handoff.
2. Decide whether to run all 9 holdouts as fresh-context baseline runs before editing skills.
3. If running holdouts first, create fixtures for state-dependent cases before invoking sub-agents.
4. If starting improvements first, begin with `shannon` because it has the lowest score and already has a validated fresh-context fixture path pattern.
5. After each fix, rerun affected visible cases plus that skill's holdout.
6. Append each cycle to the AutoResearch ledger shape:

```text
cycle	skill	batch	before_score	after_score	gates_removed	gates_added	cases_rerun	decision	evidence_path
```

## Verification Already Run

- `python3 skills/atomic-commit-push/scripts/git_preflight.py /Users/habin/workspace/agent-harness`
- `python3 skills/atomic-commit-push/scripts/api_doc_gate.py /Users/habin/workspace/agent-harness`
- `rg` checks for rubric, holdout, scorecard, and ledger references
- Fresh-context `shannon` smoke without fixture
- Fresh-context `shannon` smoke with isolated git fixture
- `git check-ignore -v` confirmed evidence files are ignored by `.gitignore:16:evidence`
