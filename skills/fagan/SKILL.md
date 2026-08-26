---
name: fagan
description: "Use when asked to review, inspect, or comment on a GitLab merge request or GitHub pull request (by number, !iid, #n, URL, or branch) — 'MR 리뷰', 'PR 리뷰', '코드 리뷰 남겨줘', 'Kody처럼 리뷰', '머지 전 검토', 'review this MR/PR' — including when a bot (Kody/Kodus, CodeRabbit, Copilot) already reviewed it and a stronger, evidence-verified review is wanted. Not for replying to existing bot threads (kody-review-feedback) or for an uncommitted local diff (code-review)."
---

# Fagan — evidence-verified MR/PR inspection

**A claim about code you did not open is not a finding.** Every posted defect was traced
through its real definitions, one hop upstream to the validation boundary and one hop
downstream to the consumer, checked against the cumulative diff, and attacked by three
skeptics. One-pass bots post what one model believed; Fagan posts what survived refutation.

Canonical location: `agent-harness/skills/fagan` (`~/.claude/skills/fagan` is a symlink
installed by `agent-harness update`). Provider auto-detected from `origin`: GitLab (`glab`)
or GitHub (`gh`). `agents/openai.yaml` exposes it as `$fagan` on Codex.

## Pipeline

```
0 Preflight  scripts/mr_context.py  → summary.md, context.json, workflow_args.json, worktree
1 Gate       scripts/quality_gate.py → gate.md / gate.json (deterministic; ~30s)
2 Find+Verify references/workflow.js → confirmed findings, refuted, verified_ok
3 Merge      you → findings.json
4 Post       scripts/post_review.py → dry run, then --post, then read-back table
```

### 0. Preflight

```bash
python3 <skill>/scripts/mr_context.py --mr <ref> --repo-dir <repo> --worktree --history 5
```

Read `<out_dir>/summary.md`. Stop and say so when `eligible=false` (closed/merged/draft, or a
Fagan review already exists for this head) unless the user explicitly asked to review it
anyway. When `large=true` (> 40 files or > 2000 added lines), tell the user the scale and
ask whether to narrow (directories or lenses) before spending agents.

`workflow_args.json` already contains the applicable lenses, their text, the checkout path
and the candidate cap — never hand-build it.

### 1. Gate

```bash
python3 <skill>/scripts/quality_gate.py --context <out_dir>/context.json
```

Runs lint / scoped typecheck / targeted tests for the changed files in the worktree and
measures LOC, longest touched function, approximate complexity and the test-to-source ratio
against `base_sha`. `gate.json → candidates` (ids `G1..`, confidence 95 tool failure / 90
new breach / 60 pre-existing / 50 unanchorable) are the **only findings exempt from the
upstream/downstream rule** — they are measurements, not claims. Numbers in the review come
from the gate; never estimate them.

### 2. Find + Verify

Claude Code: `Workflow({scriptPath: "<skill>/references/workflow.js", args: <contents of
workflow_args.json>})`. Other hosts: dispatch the `finderPrompt` / `skepticPrompt` strings
from that file with the host's sub-agent tool, in parallel, and apply the verdict rule in
`references/verification.md`. Budget: `lenses × 1 + candidates × 3` agents (default cap 24
candidates → ≤ 83; `large` MRs cap 12). Save the result to `<out_dir>/workflow-result.json`
before reading it — the `refuted` list with verdicts is large.

### 3. Merge (you are the moderator)

Write `<out_dir>/findings.json` (schema: `post_review.py` docstring):

- Start from `gate.json → candidates` (keep their `source`/`metrics`/`pre_existing`/`minor`
  fields), then add the workflow's `findings` with ids `F1..` (keep `skeptics_passed`,
  `upstream`, `downstream`).
- The workflow's `partial` list (refuted candidates where a skeptic still stood at ≥ 70)
  becomes `open_questions`: one line each — the residual risk in the standing skeptic's words,
  the location, and what the author should confirm.
- `verified_ok` entries stay as `{concern, why_ok, loc, thread}` objects; pick at most 8,
  preferring ones that contradict an existing bot thread (`thread` set).
- Drop a finding whose line already has a bot/human thread unless it contradicts that thread
  (then say so in `what`). Never repeat a claim in `prior_review_lessons`.
- Re-review of a moved head: `context.json → prior_fagan_threads` lists earlier Fagan
  threads; resolved ones go to `verified_ok` as "이전 지적 Fn 해결 확인", unresolved ones are
  referenced, not re-posted.
- `verified_ok` = the risky-looking things inspectors traced and cleared, each with evidence.
- `rule_candidates` = each refutation that hinged on a project fact, as one Korean sentence
  naming the fact and the proving file.
- Verdict: `request_changes` if any critical/high; `approve` if nothing ≥ medium and the
  tests lens found the changed behavior covered; else `comment`.
- Korean prose; identifiers, paths, commands verbatim. Never quote secret values.

### 4. Post

```bash
python3 <skill>/scripts/post_review.py --context <out_dir>/context.json --findings <out_dir>/findings.json --gate <out_dir>/gate.json          # dry run
python3 <skill>/scripts/post_review.py ... --post                                                                                             # only when asked
```

Inline bar is severity-weighted: critical/high ≥ 50, medium ≥ 65, low ≥ 80, and any
finding all three skeptics failed to refute (`skeptics_passed`) qualifies at ≥ 50. At most 8
inline, agent findings first; deterministic gate findings get one inline slot (the worst by
complexity) and the rest a table. Medium+ findings under the bar are shown in "저자 확인
요청" with their `what`, never folded away. Pre-existing and minor (≤ 10 LOC over, complexity
≤ 3) breaches are table-only.
Findings on non-diff lines are demoted to the summary with a warning (`--strict` to fail
instead). Secrets are masked. GitHub `approve` posts as `COMMENT` unless `--allow-approve`.
The script refuses a moved head, a closed MR, and a second post for the same head. Report
the read-back table it prints, not the POST responses.

When the user did not ask to post, the deliverable in chat is: verdict, the "지적" table,
"저자 확인 요청", the 자동 검사 table, "검토했으나 문제 없음", rule proposals, and one line per
refuted candidate (title + winning skeptic reason). Do not paste the inline bodies. The
summary's first paragraph must name only defects that appear in the tables or in "저자 확인
요청" — never mention a finding the reader cannot find below.

### 5. Clean up

`git worktree remove --force <out_dir>/worktree` on every exit path, including
`eligible=false` and validation failures. Delete throwaway specs the reproducer left.

## Hard rules

- No agent-found finding without an opened definition, an upstream hop, a downstream hop and
  a failure scenario; "probably/may/could" in `why` means unverified. Gate findings are the
  only exception.
- A single call site never proves a signature or a field. `{ results: users }` reads
  `results`.
- Only changed or newly-reachable lines. Pre-existing debt appears only if the change makes it
  worse, and says so.
- A suggestion is posted only after it was applied in the worktree and passed the fastest
  relevant check.
- Inability to verify is not a refutation, and intent is not correctness — a skeptic needs
  evidence to kill a candidate; an `intent`-lens defect is not refuted by "the author meant it".
- Never resolve threads, never approve or merge through any other path, never post twice for
  one head.

| Rationalisation | Reality |
|---|---|
| "The diff makes it obvious, no need to open the callee" | !5581: the "obvious" empty-string bug was blocked by `@MinLength(1)` upstream and filtered downstream. Open it. |
| "Three skeptics per candidate is expensive" | One false positive costs the author a reply and the next real finding its credibility. Cut candidates (`maxCandidates`), not skeptics. |
| "It's in CLAUDE.md so I'll flag it" | Only if the rule text names it, the glob matches, and it's on a changed line. |
| "Post now, verify after" | The read-back table is the deliverable. |
| "No findings feels weak" | An approve with a traced `verified_ok` list is the strongest review there is. |
