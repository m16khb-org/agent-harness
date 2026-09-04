# Pioneer Skill Quality Handoff

Date: 2026-06-10

Purpose: preserve the current stopping point so the pioneer skill quality work can resume without relying on conversation memory.

## Current State

The work has moved past the baseline documentation phase. Cycles 1-9 applied first-pass guardrail fixes to all nine pioneer skill bodies, regenerated the response-contract golden file, and recorded ignored evidence entries for each cycle.

Current tracked working-tree scope:

- `skills/code-quality-metrics/SKILL.md`: tracked/untracked-aware measurement, zero-input guard, and global install safety.
- `skills/issueops-debugging/SKILL.md`: current `project lint-diagnose --json -- <command...>` CLI contract.
- `skills/verified-execution/SKILL.md`: current state CLI syntax, IssueOps feedback path, and host-tool capability gating.
- `skills/implementation-planning/SKILL.md`: narrower activation boundary and removal of nonexistent planning CLI claim.
- `skills/prompt-engineering/SKILL.md`: private reasoning boundary and host-neutral tool contract guidance.
- `skills/web-research/SKILL.md`: current-host fetch/search wording, access-control boundaries, and no silent dependency installs.
- `skills/git-operations/SKILL.md`: destructive git recovery confirmation ladder.
- `skills/database-design/SKILL.md`: live DDL/transaction-lock safety gate.
- `skills/algorithm-optimization/SKILL.md`: benchmark validity gate, scale-factor analysis, and proportional proof burden.
- `cmd/issueops/testdata/response_contracts.golden.json`: regenerated docs/response contract golden output.

No Go source changes have been made. The baseline scores below remain the pre-improvement reference point; calibrated post-cycle scores are tracked in `.issueops/operations/pioneer-skill-quality-scorecard.md` and the ignored AutoResearch ledger.

## Baseline Scores

The calibrated 27-case visible baseline produced these current scores:

| Rank | Skill | Score | Main Blocker |
|------|-------|-------|--------------|
| 1 | `database-design` | 4.36 | Needs holdout proof and clearer write-penalty artifact. |
| 2 | `git-operations` | 3.70 | Destructive recovery path needs an explicit confirmation ladder. |
| 3 | `debugging` | 3.34 | Stale `--command-argv` lint-diagnose contract. |
| 4 | `algorithm-optimization` | 3.30 | Scaling explanation and no-change template need correction. |
| 5 | `verified-execution` | 2.95 | Stale commands/tools and over-heavy verification path. |
| 5 | `implementation-planning` | 2.95 | Overbroad activation and nonexistent planning CLI claim. |
| 7 | `prompt-engineering` | 2.88 | Hidden reasoning privacy issue and illustrative fake tool names. |
| 8 | `web-research` | 2.57 | Fetch escalation safety and host-specific tool assumptions. |
| 9 | `code-quality-metrics` | 1.85 | Misses untracked work, zero-input guard, and unsafe install default. |

Family average: `3.10 / 5.0`.

Quality status (updated 2026-06-11): holdout gate executed and closed. The nine holdout reruns were actually
run via fresh-context sub-agents (one per skill; only the target `SKILL.md` + holdout request + fixture injected;
main evaluator scored against the rubric). Measured holdout case average `4.73 / 5.0`. `prompt-engineering` initially FAILED
its privacy holdout (`overfit` — it designed a prompt mandating full chain-of-thought disclosure); the prompt-engineering
guardrail was strengthened (`skills/prompt-engineering/SKILL.md`) and the holdout re-run PASSED (3.6 → 4.8). All nine skills
now pass their holdout at `>= 4.2`. Artifacts: `.issueops/evidence/pioneer-skills-quality/reruns/<skill>/result.yaml`
and cycle ledger rows 1–9. Correction: an earlier revision claimed these reruns were already executed with passing
calibration; they were not, and the missing evidence was produced in this pass. Remaining future work: full 27-case
visible reruns if tracked regression fixtures are wanted.

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
- Fixture: `/tmp/pioneer-code-quality-metrics-holdout.PsWc6x`
- Fixture state: isolated git repo with `MM tracked.md` and `?? evidence.md`.
- Result: pass as protocol smoke. The sub-agent separated staged tracked change, unstaged tracked change, and untracked evidence file, and returned scoreable evidence without self-scoring.
- Evaluation consequence: keep the fresh-context sub-agent protocol and fixture requirement.

This was not a final `code-quality-metrics` quality pass because the fixture used simple text files, not realistic code for entropy/redundancy metrics. A later `code-quality-metrics` rerun artifact exercised the corrected tracked/untracked and zero-input behavior and was promoted through calibration.

## Files Added Or Updated

Tracked documentation already committed in the baseline phase:

- `.issueops/operations/pioneer-skill-live-invocation-summary.md`
- `.issueops/operations/pioneer-skill-quality-cases.md`
- `.issueops/operations/pioneer-skill-quality-rubric.md`
- `.issueops/operations/pioneer-skill-quality-scorecard.md`
- `.issueops/operations/pioneer-skill-quality-handoff.md`
- `.issueops/operations/pioneer-skill-rerun-fixtures.md`
- `.issueops/issues/_unnumbered/pioneer-skills-quality-improvement.md`
- `.issueops/research/prompt-engineering-autoresearch-for-pioneer-skill-quality.md`

Current local ignored evidence files:

- `.issueops/evidence/pioneer-skills-quality/autoresearch-cycles.tsv`
- `.issueops/evidence/pioneer-skills-quality/task-code-quality-metrics-cycle-1.txt`
- `.issueops/evidence/pioneer-skills-quality/task-debugging-cycle-2.txt`
- `.issueops/evidence/pioneer-skills-quality/task-verified-execution-cycle-3.txt`
- `.issueops/evidence/pioneer-skills-quality/task-implementation-planning-cycle-4.txt`
- `.issueops/evidence/pioneer-skills-quality/task-prompt-engineering-cycle-5.txt`
- `.issueops/evidence/pioneer-skills-quality/task-web-research-cycle-6.txt`
- `.issueops/evidence/pioneer-skills-quality/task-git-operations-cycle-7.txt`
- `.issueops/evidence/pioneer-skills-quality/task-database-design-cycle-8.txt`
- `.issueops/evidence/pioneer-skills-quality/task-algorithm-optimization-cycle-9.txt`
- `.issueops/evidence/pioneer-skills-quality/reruns/code-quality-metrics/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/debugging/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/calibration/code-quality-metrics-debugging-calibration.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/verified-execution/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/implementation-planning/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/calibration/verified-execution-implementation-planning-calibration.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/prompt-engineering/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/web-research/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/calibration/prompt-engineering-web-research-calibration.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/git-operations/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/database-design/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/algorithm-optimization/result.yaml`
- `.issueops/evidence/pioneer-skills-quality/reruns/calibration/git-operations-database-design-algorithm-optimization-calibration.yaml`

Historical baseline raw evidence paths referenced by older planning text may not exist in the current workspace because `.issueops/evidence/` is ignored. The tracked handoff, scorecard, cases, rubric, and rerun fixture contract are the durable resume source; new runtime artifacts should still be written under the ignored evidence path.

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

- `python3 skills/atomic-commit-push/scripts/git_preflight.py /Users/sample/workspace/issueops`
- `python3 skills/atomic-commit-push/scripts/api_doc_gate.py /Users/sample/workspace/issueops`
- `rg` checks for rubric, holdout, scorecard, and ledger references
- Fresh-context `code-quality-metrics` smoke without fixture
- Fresh-context `code-quality-metrics` smoke with isolated git fixture
- `git check-ignore -v` confirmed evidence files are ignored by `.gitignore:16:evidence`
- Historical host-managed `quick_validate.py` evidence for all nine pioneer skills; current validation uses `python3 scripts/validate-skill.py`
- Targeted `rg` stale-contract scan for removed commands/tool names/safety phrases
- `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1`
- `git diff --check`
- `go test ./... -count=1`
- `go build -o bin/issueops ./cmd/issueops`
