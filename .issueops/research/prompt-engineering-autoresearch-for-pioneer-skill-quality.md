# Karpathy AutoResearch Notes for Pioneer Skill Quality

Sources checked on 2026-06-10:

- `https://github.com/prompt-engineering/autoresearch`
- `https://raw.githubusercontent.com/prompt-engineering/autoresearch/master/README.md`
- `https://raw.githubusercontent.com/prompt-engineering/autoresearch/master/program.md`
- `https://github.com/prompt-engineering/autoresearch/issues/57`
- `https://github.com/prompt-engineering/autoresearch/discussions/43`

## Relevant Pattern

AutoResearch is a tight autonomous experiment loop:

1. Start from a fixed baseline.
2. Mutate a bounded surface.
3. Run a fixed-budget evaluation.
4. Measure one primary metric.
5. Keep the mutation if it improves the metric.
6. Discard or revert if it regresses, crashes, or adds unjustified complexity.
7. Log every experiment.
8. Repeat.

The repository applies this to LLM training:

- `prepare.py` is fixed and not modified.
- `train.py` is the bounded mutation surface.
- `program.md` is the agent instruction surface.
- `val_bpb` is the primary metric; lower is better.
- `results.tsv` logs commit, metric, memory, status, and description.
- Bad experiments are discarded or reset.

## Adaptation for Skill Quality

For pioneer skills, the analog is:

| AutoResearch | Pioneer Skill Quality |
|--------------|-----------------------|
| fixed training harness | fixed 27-case skill quality fixture suite |
| `train.py` mutation surface | one `skills/<name>/SKILL.md` or directly linked reference file |
| `val_bpb` | weighted skill score from the qualitative rubric |
| crash | unsafe, stale-contract, fake-tool, or evidence-missing gate |
| keep | score improves and no gate worsens |
| discard/reset | score regresses, new gate appears, or complexity rises without quality gain |
| `results.tsv` | `.issueops/evidence/pioneer-skills-quality/autoresearch-cycles.tsv` |

## Important Codex Adjustment

Karpathy's `program.md` tells the agent to continue indefinitely. That does not map cleanly to Codex sessions and Stop hooks. Karpathy's own issue #57 says Codex did not work well with that setup because it did not keep going in the intended way.

For this repo, use a resumable loop instead:

- Run one explicit cycle at a time.
- Write cycle state to evidence files.
- End with a clear continue/stop choice when user judgement is needed.
- Never rely on a single uninterrupted agent session.

## Loop Rules to Import

1. **Single metric**: use the rubric score and gate flags, not vague judgement.
2. **Bounded mutation**: improve one defect class or one skill at a time.
3. **Fixed fixture suite**: every improvement must re-run affected cases plus regression cases.
4. **Keep/discard**: keep only if score improves or a gate flag is removed without introducing a new one.
5. **Complexity tax**: small score gains that make the skill much longer or more fragile can be discarded.
6. **Ledger first**: every cycle records baseline, mutation, score delta, gate delta, and keep/discard decision.
7. **Dead-end value**: failed attempts still matter if they identify a non-working improvement path.

## Skill-Quality Autoresearch Stop Condition

Stop only when:

- all 9 skills score `>= 4.2/5.0`;
- all 27 cases score `>= 3.5/5.0`;
- no `unsafe`, `stale-contract`, or `fake-tool` gate remains;
- all final evidence is A, B, or C strength;
- the scorecard has a cycle ledger proving the path from baseline to final.
