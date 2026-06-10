# Pioneer Skill Quality Handoff

Date: 2026-06-10

Purpose: preserve the current stopping point so the pioneer skill quality work can resume without relying on conversation memory.

## Current State

The work has moved past the baseline documentation phase. Cycles 1-9 applied first-pass guardrail fixes to all nine pioneer skill bodies, regenerated the response-contract golden file, and recorded ignored evidence entries for each cycle.

Current tracked working-tree scope:

- `skills/shannon/SKILL.md`: tracked/untracked-aware measurement, zero-input guard, and global install safety.
- `skills/hopper/SKILL.md`: current `project lint-diagnose --json -- <command...>` CLI contract.
- `skills/turing/SKILL.md`: current state CLI syntax, IssueOps feedback path, and host-tool capability gating.
- `skills/von-neumann/SKILL.md`: narrower activation boundary and removal of nonexistent planning CLI claim.
- `skills/karpathy/SKILL.md`: private reasoning boundary and host-neutral tool contract guidance.
- `skills/berners-lee/SKILL.md`: current-host fetch/search wording, access-control boundaries, and no silent dependency installs.
- `skills/torvalds/SKILL.md`: destructive git recovery confirmation ladder.
- `skills/codd/SKILL.md`: live DDL/transaction-lock safety gate.
- `skills/dijkstra/SKILL.md`: benchmark validity gate, scale-factor analysis, and proportional proof burden.
- `cmd/harness/testdata/response_contracts.golden.json`: regenerated docs/response contract golden output.

No Go source changes have been made. The baseline scores below remain the pre-improvement reference point; calibrated post-cycle scores are tracked in `.agent-harness/operations/pioneer-skill-quality-scorecard.md` and the ignored AutoResearch ledger.

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

This was not a final `shannon` quality pass because the fixture used simple text files, not realistic code for entropy/redundancy metrics. A later `shannon` rerun artifact exercised the corrected tracked/untracked and zero-input behavior and was promoted through calibration.

## Files Added Or Updated

Tracked documentation already committed in the baseline phase:

- `.agent-harness/operations/pioneer-skill-live-invocation-summary.md`
- `.agent-harness/operations/pioneer-skill-quality-cases.md`
- `.agent-harness/operations/pioneer-skill-quality-rubric.md`
- `.agent-harness/operations/pioneer-skill-quality-scorecard.md`
- `.agent-harness/operations/pioneer-skill-quality-handoff.md`
- `.agent-harness/operations/pioneer-skill-rerun-fixtures.md`
- `.agent-harness/plans/pioneer-skills-quality-improvement.md`
- `.agent-harness/research/karpathy-autoresearch-for-pioneer-skill-quality.md`

Current local ignored evidence files:

- `.agent-harness/evidence/pioneer-skills-quality/autoresearch-cycles.tsv`
- `.agent-harness/evidence/pioneer-skills-quality/task-shannon-cycle-1.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-hopper-cycle-2.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-turing-cycle-3.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-von-neumann-cycle-4.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-karpathy-cycle-5.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-berners-lee-cycle-6.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-torvalds-cycle-7.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-codd-cycle-8.txt`
- `.agent-harness/evidence/pioneer-skills-quality/task-dijkstra-cycle-9.txt`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/shannon/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/hopper/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/calibration/shannon-hopper-calibration.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/turing/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/von-neumann/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/calibration/turing-von-neumann-calibration.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/karpathy/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/berners-lee/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/calibration/karpathy-berners-lee-calibration.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/torvalds/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/codd/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/dijkstra/result.yaml`
- `.agent-harness/evidence/pioneer-skills-quality/reruns/calibration/torvalds-codd-dijkstra-calibration.yaml`

Historical baseline raw evidence paths referenced by older planning text may not exist in the current workspace because `.agent-harness/evidence/` is ignored. The tracked handoff, scorecard, cases, rubric, and rerun fixture contract are the durable resume source; new runtime artifacts should still be written under the ignored evidence path.

Draft-wiki material queued:

- `dwq-43dcf1dd6ac71c7c51009a10`

## Recommended Resume Order

1. Review the current uncommitted diff for the nine skill bodies, this handoff, the scorecard, the plan, and `response_contracts.golden.json`.
2. If approved, commit the first-pass guardrail fixes as one atomic docs/skill commit.
3. Review the complete cycles 1-9 evidence and tracked diff.
4. Append any later post-rerun scoring cycle to the AutoResearch ledger shape:

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
- `python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py` for all nine pioneer skills
- Targeted `rg` stale-contract scan for removed commands/tool names/safety phrases
- `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1`
- `git diff --check`
- `go test ./... -count=1`
- `go build -o bin/agent-harness ./cmd/harness`
