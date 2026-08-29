---
name: parnas
description: "Use when asked to review, inspect, or comment on a GitLab merge request or GitHub pull request (by number, !iid, #n, URL, or branch) — 'MR 리뷰', 'PR 리뷰', '코드 리뷰 남겨줘', 'Kody처럼 리뷰', '머지 전 검토', 'review this MR/PR' — including when a bot (Kody/Kodus, CodeRabbit, Copilot) already reviewed it and a stronger, evidence-verified review is wanted. Not for replying to existing bot threads (review-agent-feedback) or for an uncommitted local diff (code-review)."
---

# Parnas — evidence-verified MR/PR inspection

**A claim about code you did not open is not a finding.** Every posted defect was traced
through its real definitions, one hop upstream to the validation boundary and one hop
downstream to the consumer, checked against the cumulative diff, and attacked by three
skeptics. One-pass bots post what one model believed; this skill posts what survived
refutation.

Named after David Parnas, whose *Active Design Reviews* (Parnas & Weiss, 1985) showed that
a passive review produces "looks fine": reviewers must be forced to answer specific
questions and demonstrate they actually used the artifact. That is what the lenses and
skeptics below do — each inspector answers one question with opened code, and each
candidate must survive an attempt to refute it.

Canonical location: `agent-harness/skills/parnas` (`~/.claude/skills/parnas` is a symlink
installed by `agent-harness update`). Provider auto-detected from `origin`: GitLab (`glab`)
or GitHub (`gh`). `agents/openai.yaml` exposes it as `$parnas` on Codex.

## Pipeline

```
0 Preflight  scripts/mr_context.py  → summary.md, defs.md, pack/, hunks/, context.json, workflow_args.json, worktree
1 Gate       scripts/quality_gate.py → gate.md / gate.json (deterministic; ~30s)
2 Find+Verify references/workflow.js → confirmed findings, refuted, verified_ok, refuted_for_history
3 Merge      you → findings.json
4 Post       scripts/post_review.py → dry run, then --post, then read-back table
```

### 0. Preflight

```bash
python3 <skill>/scripts/mr_context.py --mr <ref> --repo-dir <repo> --worktree --history 5
```

Read `<out_dir>/summary.md`. Stop and say so when `eligible=false` (closed/merged/draft, or a
review by this skill already exists for this head) unless the user explicitly asked to review it
anyway. When `large=true` (> 40 files or > 2000 added lines), tell the user the scale and
ask whether to narrow (directories or lenses) before spending agents.

`workflow_args.json` already contains the finder units, the checkout path, the candidate caps,
the hunk ranges and prior lessons for the prescreen — never hand-build it. A **unit** is one
lens bundle × one shard of the changed files (bundles: behavior = logic/boundary/data/async,
contract = security/contract/rules, intent = tests/scope/intent); each unit has one
self-contained pack (`pack/<unit>.md`: that slice's cumulative diff, the definitions and
one-hop neighbours of the symbols it defines, matching rules, threads and prior lessons). One
finder applies every lens of its bundle over one pack read — measured 2026-08-28, re-reading the
same pack per lens cost more than lens independence bought. Shards keep a pack under ~150 KB
of diff so a finder can read it whole in one call; `defs.md` and `hunks/<file>.patch` serve
the skeptics. The number of units is printed in `summary.md` — a `large` MR yields ~9; ask the
user whether to narrow before spending them.

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
workflow_args.json>})`. Omo: `python3 <skill>/scripts/omo_driver.py --args
<out_dir>/workflow_args.json` runs the whole find+verify stage as concurrent `omo -p` agents
(default `--profile omo-flash`: zai/glm-5.3-flash, finder 24 turns, skeptic 18, candidate cap
floored at 40, 10-way concurrency, thinking high on every role — cheap tokens buy wider
search and deeper traces, not a looser verdict rule; `--profile standard` reproduces the
Workflow budgets).
Other hosts: dispatch the `finderPrompt` / `skepticPrompt` strings
from that file with the host's sub-agent tool and apply the prescreen and verdict rule in
`references/verification.md`. Budget: `units × 1 + candidates × (1..2)` agents — a
deterministic prescreen drops off-hunk and already-refuted candidates, the tracer runs first,
and the reproducer only where the tracer failed to refute. Every agent has a hard message
budget (finder 10, skeptic 8) and is told to batch reads, because an agent's cost is
Σ(context length per turn), not its output. Every role runs on the session model (opus) unless `args.models = {finder, tracer, reproducer}`
says otherwise — measured 2026-08-28 on !5617, cheaper models did not use fewer tokens (sonnet
finders took as many turns as opus) and a haiku critic burned 14M tokens for zero surviving
candidates, so the cost levers are structural: one pack per unit, a message budget, the
prescreen, and incremental re-review. The tracer is **blind** to the finder's
evidence/upstream/downstream (information asymmetry, OpenCodeReview) — it must find its own
hops; the reproducer sees everything. The result's `cost` block reports agents, prescreened,
reproducers skipped and output tokens — quote it in the chat deliverable. Save the result to
`<out_dir>/workflow-result.json` before reading it — the `refuted` list with verdicts is large.

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
- Re-review of a moved head: `context.json → prior_review_threads` lists earlier threads
  this skill posted (either marker); resolved ones go to `verified_ok` as "이전 지적 Fn 해결 확인", unresolved ones are
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
finding both skeptics failed to refute (`skeptics_passed`) qualifies at ≥ 50. At most 8
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

### 4b. Remember refutations (team memory)

```bash
python3 <skill>/scripts/record_refuted.py --result <out_dir>/workflow-result.json --context <out_dir>/context.json
```

Appends evidence-backed refutations (skeptic confidence ≥ 80) to `<repo>/.agent-harness/parnas/refuted.jsonl`
— commit it with the repo. The next run's prescreen drops a same-file candidate whose title/what overlap
≥ 0.5 with a recorded refutation; `security`/`data` candidates are never suppressed. Prescreen kills are
not recorded (they carry no evidence).

### 4c. Re-review of a moved head

Run preflight with `--incremental`: when `<out_dir>` already holds `workflow-result.json` and
`context.prev.json` from the previous head, only units whose files changed since that head are
re-inspected and findings on untouched files are carried (`carried_from`). Findings on changed
files are dropped and re-found or not. `post_review.py` still validates every line against the
new diff.

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
| "Two skeptics per candidate is expensive" | One false positive costs the author a reply and the next real finding its credibility. Cut candidates (`maxCandidates`), not skeptics — the 2026-08-28 measurement showed finders exploring by hand (105 turns each) were the cost, not the skeptics. |
| "The finder needs to grep around to be sure" | The pack already holds its diff slice, definitions and one-hop neighbours; 10 messages is the budget. Before packs, finders spent 30 turns grepping and never managed to read the 462 KB diff at all. |
| "It's in CLAUDE.md so I'll flag it" | Only if the rule text names it, the glob matches, and it's on a changed line. |
| "Post now, verify after" | The read-back table is the deliverable. |
| "No findings feels weak" | An approve with a traced `verified_ok` list is the strongest review there is. |
