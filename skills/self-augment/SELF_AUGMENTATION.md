# Self-verification and self-augmentation loops

This reference separates two contracts that were previously conflated.

- **Self-verification loop**: verifies that the service or harness behaves as intended, including tests and QA.
- **Self-augmentation loop**: chooses a necessary feature, performance, quality, documentation, or operations improvement, implements it, and verifies it with the self-verification loop.

Both loops use concrete goal scores. The default exit condition is that every goal score must be greater than 95. If any score is 95 or below, the result is not complete; improve, retry, or report the blocker.

## 1. Self-verification loop

### Purpose

Verify that the harness produces consistent results across Codex and Claude Code, and that CLI, MCP, native integration, state, policy, docs, and skills behave as intended. This loop is a QA gate; it does not choose improvements by itself.

### CLI/MCP surface

```bash
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/agent-harness self-verify candidates --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
```

Legacy `self_augment_history`, `compare`, and `promote` calls remain compatibility aliases only. New docs and automation should use `self_verify_*` names.

### Required steps

Each iteration includes at least these checks:

1. Core invariant checks.
2. Go test suite and contract/golden tests.
3. Build artifact check.
4. **Risk QA tier**: run `go test -race ./... -count=1` plus `go vet ./...` for sensitive Go changes, `go vet ./...` for ordinary Go changes, and explicitly record skips for docs/config-only changes.
5. CLI/MCP response contract checks.
6. Native integration smoke checks.
7. Candidate export: verify `self-verify candidates` exports next candidates and open/satisfied status.
8. Step budget baseline: verify label-level p95 budget comparison and regression promotion.
9. Install dry-run smoke: verify temp HOME/CODEX_HOME/HARNESS_ROOT dry-run reports planned writes/links without writing files.
10. Command policy and path-fuzz checks.
11. Parallel temp isolation.
12. Duplicate MCP warning detection.
13. Daemon resilience for stale locks, sockets, restart, and socket permissions.
14. Redaction audit for docs, skill metadata, and golden response artifacts.
15. QA gate for `GENIUS_THINK.md`, loop docs, skill frontmatter, OpenAI metadata, and skill existence.

### Score goals

| Goal | Evidence label | Exit condition |
| --- | --- | --- |
| Test suite | `go test`, `contract golden tests` | score > 95 |
| Risk-based QA | `risk QA tier` | score > 95 |
| Build artifact | `go build` | score > 95 |
| QA smoke | invariants, inspect/docs, candidate export, QA gate | score > 95 |
| Candidate export | candidate export | score > 95 |
| Step budget baseline | step budget baseline | score > 95 |
| Install dry-run | install dry-run smoke | score > 95 |
| Policy/security | command policy, preflight fuzz, redaction audit | score > 95 |
| Concurrency isolation | parallel isolation | score > 95 |
| Daemon resilience | daemon resilience | score > 95 |
| Native integration | native integration | score > 95 |

Use `summary.contract`, `summary.goal_scores`, `summary.coverage`, `summary.coverage_gaps`, failure `summary.failure_class`, `summary.failure_clusters`, `summary.rerun_commands`, `summary.minimum_goal_score`, and `summary.termination_eligible` as the decision surface. `--progress=jsonl` writes progress events to stderr without corrupting the final JSON summary on stdout.

## 2. Self-augmentation loop

### Purpose

The self-augmentation loop must materially improve the repository. It must perform at least one of these actions:

- Add a necessary feature.
- Improve performance.
- Improve quality, safety, or tests.
- Improve documentation or operations experience.
- Add automation or memory structures that reduce recurring failures.

Repeating tests or producing only an analysis report is not self-augmentation.

### Surface

```bash
./bin/agent-harness self-augment --json
./bin/agent-harness self-augment --save-state --state-key self-augment-latest --json
./bin/agent-harness self-augment lesson --lesson "..." --next-action "..." --json
```

Skill contract: use `$self-augment` when the user asks for autonomous improvement, repo enhancement, next valuable improvement, or 95-point improvement loops.

### Exit goals

| Goal | Description | Exit condition |
| --- | --- | --- |
| Improvement selection | Generate and score at least 10 candidates from `GENIUS_THINK.md`, docs index, skill inventory, and git evidence. | score > 95 |
| Implementation | The selected candidate is implemented as an actual diff. | score > 95 |
| Verification and QA | Targeted tests and self-verification pass. | score > 95 |
| Learning capture | Failure/success lessons and next candidates are stored in suitable state/docs. Default state artifact: `self_augmentation_plan`. | score > 95 |

### GENIUS_THINK.md usage

Candidate generation must use at least two formulas from `GENIUS_THINK.md`. Prefer:

- Problem reframing: redefine the real repository bottleneck.
- Innovative solution generation: score value, novelty, feasibility, and risk together.
- Thought evolution: carry prior failures and lessons into the next cycle.
- Complexity resolution: split broad improvements into small, verifiable subproblems.

## 3. External strategies adopted

| Source | Adopted idea |
| --- | --- |
| Reflexion | Store language lessons from failures and reuse them in the next cycle. |
| Self-Refine | Apply generate → feedback → refine loops to candidate design and retry. |
| Voyager | Use curriculum and skill-library thinking to choose the next necessary improvement. |
| SWE-agent | Treat repo navigation, file editing, and test execution as an explicit agent-computer interface. |
| AgentBench | Score multidimensional agent goals instead of single pass/fail. |
| SWE-bench | Prefer repo-local, test-backed improvements like real issue resolution. |
| LangGraph | Use durable execution, state, recovery, and human-oversight constraints for long loops. |
| AutoGen | Use termination conditions and max-turn safeguards as score-gate and cycle-budget constraints. |
| DSPy optimizers | Use metric-first optimization for candidate scoring and regression comparison. |
| OpenAI Evals | Reuse eval artifacts and baseline promotion concepts. |

## 4. Operating rules

- The self-verification loop only writes to temp/state locations and must not modify user repo source files.
- The self-augmentation loop may create real improvement diffs, but should keep them small and reversible.
- Any new CLI/MCP/native capability must add evidence labels to self-verification tests or QA steps.
- Never lower the 95-point gate. Skipped verification does not count as a passing score.
- Capture reusable failures with `self-augment lesson` or the appropriate `.agent-harness` document.
