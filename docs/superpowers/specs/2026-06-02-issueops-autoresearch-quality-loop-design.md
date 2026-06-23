# IssueOps Autoresearch Quality Loop Design

## Problem

IssueOps already has durable workflow state, a Korean remote artifact gate, isolated worktree requirements, and a benchmark with deterministic plus `Z.AI Coding Plan` judging. What it lacks is an explicit improvement loop that turns IssueOps changes into repeatable experiments.

The requested improvement is to adapt the useful parts of Karpathy-style autoresearch to IssueOps itself. The goal is not to recreate the ML training repository. The goal is to make IssueOps changes follow a small research loop: write a brief, limit the edit surface, run the same quality gate against baseline and candidate, and keep only changes that improve the measured result.

## Source Pattern

Karpathy autoresearch is useful here because its loop is intentionally constrained:

- one narrow editable surface,
- a fixed experiment budget,
- one objective metric,
- a keep-or-discard decision after every experiment.

For IssueOps, those translate into:

- one bounded IssueOps improvement candidate,
- one declared set of editable files,
- one benchmark command matrix,
- one comparison result deciding whether the candidate is accepted.

## Success Criteria

- IssueOps has a repo-local autoresearch quality loop contract for improving IssueOps prompts, docs, state, and benchmark behavior.
- Each candidate records a research brief with hypothesis, target dimensions, edit surface, metric commands, and keep/discard criteria.
- The loop uses the existing IssueOps benchmark as the quality gate instead of introducing a separate scoring system.
- Baseline and candidate runs can be compared through stable JSON output.
- The candidate passes only when the quality gate reports no critical failures and no dimension regression.
- The deterministic benchmark remains usable without network credentials or real model calls.
- The `Z.AI Coding Plan` judge remains opt-in for real LLM review, while tests use fake Z.AI.
- The IssueOps skill and project docs explain when to use this loop and when not to.

## Non-Goals

- Do not build a detached overnight runner.
- Do not let hooks create issues, branches, worktrees, commits, PRs, or MRs.
- Do not call real `zai` from automated tests.
- Do not relax fixtures, judge prompts, or benchmark thresholds to make a weak candidate pass.
- Do not add a provider-specific score schema.
- Do not create a broad workflow engine outside IssueOps.

## Domain Model

### Autoresearch Candidate

An IssueOps autoresearch candidate is one proposed improvement to the IssueOps workflow. It records:

- `id`: stable candidate id.
- `hypothesis`: what quality gap the candidate should improve.
- `target_dimensions`: benchmark dimensions expected to improve.
- `edit_surface`: file globs or exact paths the candidate may touch.
- `baseline_command`: command used to capture current quality.
- `candidate_command`: command used after the candidate change.
- `keep_criteria`: measurable acceptance rule.
- `discard_criteria`: failure or regression rule.

### Quality Gate

The quality gate is the existing IssueOps benchmark:

```bash
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge llm --json
agent-harness issueops benchmark compare --baseline KEY --candidate KEY --json
```

`--judge none` is the deterministic local gate. `--judge llm` is the opt-in LLM gate when Z.AI quota is available. A candidate must not require real GitHub, GitLab, or real model calls to pass local tests.

### Keep Or Discard

Keep a candidate only when:

- `critical_failure_count` is `0`,
- `minimum_score` is not lower than baseline,
- `average_score` is not lower than baseline,
- no target dimension regresses,
- relevant Go tests and golden tests pass.

Discard or revise a candidate when:

- any critical failure appears,
- benchmark comparison reports a regression,
- the candidate changes fixture or judge rules without explicit justification,
- the candidate touches files outside the declared edit surface.

## Workflow

1. Start IssueOps and create or link the remote issue.
2. Create the issue-derived branch and isolated sibling worktree.
3. Write the candidate research brief before editing implementation files.
4. Capture deterministic baseline with `issueops benchmark run --judge none`.
5. Implement one bounded candidate.
6. Run focused tests and the deterministic candidate benchmark.
7. Compare baseline and candidate.
8. Optionally run fake Z.AI tests and real `--judge llm` only when explicitly available.
9. Keep the candidate only if the gate passes; otherwise record the failure and revise or discard.
10. Draft the PR only after IssueOps `pr-readiness` reports issue and plan links.

## Files And Surfaces

Expected implementation surfaces:

- `skills/issueops/SKILL.md`: workflow contract and quality gate wording.
- `internal/core/issueops_benchmark.go`: candidate comparison or regression helper if needed.
- `internal/core/issueops_benchmark_test.go`: deterministic gate coverage.
- `cmd/harness/issueops.go`: CLI output or command wiring if needed.
- `cmd/harness/issueops_benchmark_test.go`: CLI behavior tests.
- `docs/superpowers/specs/issueops-issue-pr-guidelines.md`: only if the benchmark requires clearer quality wording.
- `testdata/issueops/fixtures/*.json`: only when adding a new fixture that exposes a real gap.

The implementation should prefer documenting and enforcing the loop through existing IssueOps benchmark structures before adding new command surfaces.

## Testing

Required local verification:

```bash
go test ./internal/core -run IssueOps -count=1
go test ./cmd/harness -run IssueOps -count=1
go test ./cmd/harness -run Golden -count=1
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
```

Fake Z.AI tests must cover strict JSON success, malformed output failure, schema failure, and gate/regression handling when the implementation touches judge behavior.

## Risks

- The loop can become performative if it only writes research briefs without comparing metrics.
- The benchmark can become self-serving if fixtures are weakened during candidate implementation.
- A broad edit surface would recreate the same ambiguity autoresearch is meant to prevent.
- Real `zai` calls can make CI nondeterministic, so local tests must stay fake-backed.

## Issue Link

Remote issue: https://github.com/example/agent-harness/issues/9
