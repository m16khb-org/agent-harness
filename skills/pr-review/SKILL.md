---
name: pr-review
description: "Use when reviewing, inspecting, or commenting on a GitHub pull request or GitLab merge request by number, URL, or branch, including a second review after an automated reviewer. Use review-agent-feedback for replies to existing bot threads; this skill does not review uncommitted local diffs."
---

# PR Review

**A claim about code you did not open is not a finding.** Every posted defect was traced
through its real definitions, one hop upstream to the validation boundary and one hop
downstream to the consumer, and checked against the cumulative diff. At `--level max` it was
also attacked by a blind tracer and, where that failed to refute it, a reproducer — one-pass
bots post what one model believed; `max` posts what survived refutation. The cheaper levels
keep the evidence rule and drop the adversarial stage, and say so in the review they post.

The method follows *Active Design Reviews* (Parnas & Weiss, 1985): reviewers answer
specific questions and demonstrate they used the artifact. Each inspector answers
one question with opened code, and each candidate must survive an attempt to refute it.

Canonical location: `issueops/skills/pr-review` (`~/.claude/skills/pr-review` is a symlink
installed by `issueops update`). Provider auto-detected from `origin`: GitLab (`glab`)
or GitHub (`gh`). `agents/openai.yaml` exposes it as `$pr-review` on Codex.

## Pipeline

```
0 Preflight  scripts/mr_context.py  → summary.md, defs.md, pack/, hunks/, context.json, workflow_args.json, checkout
1 Gate       scripts/quality_gate.py → gate.md / gate.json (deterministic; ~30s)
2 Find       max: references/workflow.js (fan-out)  ·  below max: you read the packs inline
2b Screen    scripts/prescreen.py (inline levels; workflow.js does this itself at max)
3 Merge      you → findings.json
4 Post       scripts/post_review.py → dry run, then --post, then read-back table
```

## Effort levels

`--level` on preflight picks how much is spent. **Only `max` spawns agents.** Every level below
it runs the same lenses inline in your own context — no finder sub-agents, no skeptics — which
is why its cost does not scale with the number of units. The default is `high`.

| level | gate | find | verify | 후보 조리개 (렌즈 8개 기준) |
|---|---|---|---|---|
| `low` | 생략 | `logic`+`boundary`만, 팩 1패스 | prescreen | 렌즈당 4 |
| `medium` | ✅ | 적용 렌즈 전부, 인라인 순차 (정밀도 편향) | prescreen | 렌즈당 3 |
| `high` (기본) | ✅ | 적용 렌즈 전부, 인라인 순차 (재현율 편향) | prescreen | 렌즈당 4 |
| `xhigh` | ✅ | 전부 + sweep 패스 | prescreen | 렌즈당 6 |
| `max` | ✅ | workflow.js 팬아웃 | prescreen + blind tracer + reproducer | 규모 기반 (기존) |

`summary.md` prints the resolved plan as a `level=` line; `context.json → level` drives the
disclosure `post_review.py` puts in the posted body. There is no hard cap on the number of
findings at any level — the confidence bar (inline ≥ 80 / 60–79 summary / < 60 dropped) and
`--max-inline` already decide what ships, and truncating by severity would drop a verified
critical from a review that gets posted once.

**Pick `max` when the review is the gate** — a release branch, a change to money/auth/data, or
any MR whose findings you intend to state as verified. Pick `high` (or below) for a running
review of ordinary work. Say which level ran when you report in chat; never describe an inline
level's output as verified.

### 0. Preflight

```bash
python3 <skill>/scripts/mr_context.py --mr <ref> --repo-dir <repo> --worktree --history 5 --level high
```

Read `<out_dir>/summary.md`. Stop and say so when `eligible=false` (closed/merged/draft, or a
review by this skill already exists for this head) unless the user explicitly asked to review it
anyway. When `large=true` (> 40 files or > 2000 added lines), tell the user the scale and
ask whether to narrow (directories or lenses) before spending agents.

`--worktree` means "guarantee an isolated-safe review checkout", not "always create a
detached worktree". When `--repo-dir` already points to a clean primary checkout or linked
worktree at the MR/PR `head_sha`, preflight reuses it and records that path in
`context.json → checkout`; `context.json → worktree` remains null. It creates
`<out_dir>/worktree` as a detached checkout only when the supplied checkout is dirty or at a
different commit. Never create a second detached worktree merely because `--repo-dir` is a
linked worktree.

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

Skipped at `--level low` only. It costs no tokens and it is the only place the review's numbers
can come from, so run it at every other level even when you are reviewing cheaply.

### 2. Find — inline (`low` / `medium` / `high` / `xhigh`)

**Do not dispatch sub-agents at these levels.** You are the finder. Work through the units in
`units.json` yourself, in this context, in sequence:

1. Read `pack/<unit>.md` whole, in one call — it already holds that slice's diff, the
   definitions and one-hop neighbours, matching rules, threads and prior lessons. Do not grep
   around it; that is what the pack exists to prevent.
2. Apply each of the unit's lenses to that pack **separately**, and tag every candidate with its
   `lens`. Do not let one lens's conclusion suppress another's.
3. Emit candidates in the finder JSON shape from `references/lenses.md` (`path`, `new_line`,
   `severity`, `category`, Korean `title`/`what`/`why`/`how`, `evidence`, `upstream`,
   `downstream`, `confidence`, `newly_reachable`), at most `perLensCap` per lens.
4. `xhigh` only: after every unit, take one more pass over the diff as a fresh reviewer holding
   the current list, looking **only** for defects not already on it — moved/extracted code that
   dropped a guard, setup/teardown asymmetry in tests, config defaults flipped. Add nothing you
   already have; return nothing rather than padding.

The evidence rule does not relax because the level is cheap: a candidate still needs the opened
definition, the upstream hop, the downstream hop and a concrete failure scenario. What the cheap
levels drop is the **adversarial** stage, not the evidentiary one. A hop you could not find caps
confidence at 50 and is stated, never guessed.

Bias by level: `low`/`medium` are precision — every candidate should be one a maintainer would
act on. `high`/`xhigh` are recall — surface anything with a nameable failure scenario and let
the confidence bar sort it out.

### 2b. Screen — inline levels

Write the candidates to `<out_dir>/candidates.json` and run the deterministic stage:

```bash
python3 <skill>/scripts/prescreen.py --args <out_dir>/workflow_args.json \
  --candidates <out_dir>/candidates.json --out <out_dir>/prescreened.json
```

This is the same dedup + off-hunk screen + committed refutation memory that `workflow.js` runs
at `max` (`security`/`data` are never suppressed), and it costs no agent. Merge from
`prescreened.json → candidates`; the dropped ones carry their reason and belong in the chat
report's refuted line, not in the MR.

### 2 (max). Find + Verify — fan-out

Claude Code: `Workflow({scriptPath: "<skill>/references/workflow.js", args: <contents of
workflow_args.json>})`. Omo native: **do not execute `workflow.js` through the current Omo
session**. Run the pinned adapter instead:

```bash
python3 <skill>/scripts/omo_driver.py --args <out_dir>/workflow_args.json \
  --profile omo-flash --provider zai --model glm-5.3-flash
```

The adapter is the coordinator and dispatches each finder/tracer/reproducer as a concurrent
`omo -p` agent. It rejects any provider/model other than `zai/glm-5.3-flash`, checks that the
installed Omo exposes `--no-model-fallback` before starting, and disables model fallback on every
child process. The default `--profile omo-flash` uses finder 24 turns, skeptic 18, candidate cap
floored at 40, 10-way concurrency, and high thinking on every role — cheap tokens buy wider
search and deeper traces, not a looser verdict rule; `--profile standard` reproduces the
Workflow budgets. Shared-account burst 429s are absorbed by dense per-agent staged backoff
(5s→10s→15s→20s→30s→40s→50s→60s→60s, jittered, up to 10 attempts) and the engine's model fallback is
force-disabled via `SENPI_NO_FALLBACK=1` (`--no-model-fallback` alone is wiped by the engine's
project-trust flag rebuild), so a rate-limited agent fails loudly on the pinned model instead of
silently switching providers; a 429 must not be mistaken for a provider switch or a format error. Finder and tracer run with Omo's `read-only` permission preset;
only the reproducer gets `workspace` so it can create a throwaway test, and that worktree is
discarded after the review. The adapter mirrors packs, hunks, definitions, and gate output into
a temporary read-only input directory inside that worktree so Omo's path boundary cannot block
the mandated first read; it removes the directory after all phases finish. Omo agents submit
finder/verdict data through schema-constrained `submit_pr_review_*` tools rather than relying on a
final text blob. The extension enforces the message budget by disabling investigation tools one
turn before the cap, and the driver recovers validated tool arguments from the session even when
the final assistant text is empty. A format retry receives the original prompt and candidate
again with only the submit tool enabled. Raw stdout/stderr, return code, timeout, tool denials,
parse errors, and schema errors remain in `agent_diagnostics`; `failure_counts` separates
`parse_failure`, `schema_failure`, `timeout`, `rate_limited`, `low_confidence_abstain`, and `coverage_gap`.
Every finder returns `reviewed_files` containing each exact file path assigned to its unit
once. Missing or duplicate assignments appear under `coverage`, make the result
`status: "degraded"`, and make the adapter exit non-zero. Any failed child or abstention does
the same; an empty or incomplete result must never be treated as a clean review.
Other hosts: dispatch the `finderPrompt` / `skepticPrompt` strings
from that file with the host's sub-agent tool and apply the prescreen and verdict rule in
`references/verification.md`. Budget: `units × 1 + candidates × (1..2)` agents — a
deterministic prescreen drops off-hunk and already-refuted candidates, the tracer runs first,
and the reproducer only where the tracer failed to refute. Every agent has a hard message
budget (finder 10, skeptic 8) and is told to batch reads, because an agent's cost is
Σ(context length per turn), not its output. For Claude Code and other hosts, every role runs on
the session model (opus) unless `args.models = {finder, tracer, reproducer}` says otherwise —
measured 2026-08-28 on !5617, cheaper models did not use fewer tokens (sonnet
finders took as many turns as opus) and a haiku critic burned 14M tokens for zero surviving
candidates, so the cost levers are structural: one pack per unit, a message budget, the
prescreen, and incremental re-review. The tracer is **blind** to the finder's
evidence/upstream/downstream (information asymmetry, OpenCodeReview) — it must find its own
hops; the reproducer sees everything. The result's `cost` block reports agents, prescreened,
reproducers skipped and output tokens — quote it in the chat deliverable. Save the result to
`<out_dir>/workflow-result.json` before reading it — the `refuted` list with verdicts is large.

`--phase verify` refuses a `find-stage.json` with a missing finder or incomplete/duplicate
`reviewed_files` receipt. Recover only those units before spending skeptic agents:

```bash
python3 <skill>/scripts/omo_driver.py --args <out_dir>/workflow_args.json \
  --phase verify --retry-failed-units
```

The driver replaces results and diagnostics only for failed units, rebuilds candidate dedup,
prescreen, and coverage over the whole stage, persists the repaired `find-stage.json`, and
starts verification only when coverage is complete. A failed retry exits non-zero without
running tracers. Run this recovery separately from `--retry-degraded-from`; the driver rejects
combining finder recovery with skeptic-abstention retry so newly found candidates cannot be
filtered against an older workflow result.

To rerun only candidates that were kept as `skeptics unavailable (abstain)` in a degraded run:

```bash
python3 <skill>/scripts/omo_driver.py --args <out_dir>/workflow_args.json \
  --phase verify --retry-degraded-from <out_dir>/workflow-result.json
```

This leaves the original result untouched and writes `<out_dir>/workflow-retry-result.json`.

### 3. Merge (you are the moderator)

Write `<out_dir>/findings.json` (schema: `post_review.py` docstring):

- Start from `gate.json → candidates` (keep their `source`/`metrics`/`pre_existing`/`minor`
  fields), then add the findings with ids `F1..` — at `max` from `workflow-result.json`
  (keep `skeptics_passed`, `upstream`, `downstream`); at the inline levels from
  `prescreened.json → candidates` (keep `upstream`/`downstream`; there is no
  `skeptics_passed`, and never invent one — `post_review.py` reads it as "both skeptics failed
  to refute this" and lowers the inline bar to 50 on the strength of it).
- The workflow's `partial` list (refuted candidates where a skeptic still stood at ≥ 70)
  becomes `open_questions`: one line each — the residual risk in the standing skeptic's words,
  the location, and what the author should confirm. Inline levels have no `partial`; put the
  hops you could not complete there instead.
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
- Before posting, invoke the `fluent-korean` skill and apply it to every Korean prose
  field the agents produced — `summary`, `verified_ok`, `open_questions`,
  `rule_candidates`, and each finding's `title`/`what`/`why`/`how`. Sub-agent prose
  reaches the reviewer's thread unedited otherwise; the polish pass is a precondition
  for `--post`, not an option. Facts, evidence lines, and identifiers stay as verified.

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

When the user did not ask to post, the deliverable in chat is: the level that ran, verdict, the
"지적" table, "저자 확인 요청", the 자동 검사 table, "검토했으나 문제 없음", rule proposals, and one
line per refuted candidate (title + winning skeptic reason; at the inline levels the prescreen
reason from `prescreened.json`). Do not paste the inline bodies. The
summary's first paragraph must name only defects that appear in the tables or in "저자 확인
요청" — never mention a finding the reader cannot find below.

### 4b. Remember refutations (team memory)

```bash
python3 <skill>/scripts/record_refuted.py --result <out_dir>/workflow-result.json --context <out_dir>/context.json
```

Appends evidence-backed refutations (skeptic confidence ≥ 80) to `<repo>/.issueops/pr-review/refuted.jsonl`
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

Read `context.json` before cleanup. Remove a worktree only when its non-null `worktree`
field resolves exactly to `<out_dir>/worktree`; a null field means preflight reused the
caller's checkout and it must never be removed. Remove an isolated review worktree with
`git worktree remove --force <out_dir>/worktree` on every exit path, including
`eligible=false` and validation failures. Delete throwaway specs the reproducer left.

## Hard rules

- No finding without an opened definition, an upstream hop, a downstream hop and a failure
  scenario; "probably/may/could" in `why` means unverified. This holds at every level — cheap
  buys fewer lenses and no skeptics, never a lower evidence bar. Gate findings are the only
  exception.
- Below `max`, never call the output verified, and never spawn finder or skeptic sub-agents to
  "top it up" — that is `max` with the disclosure of a cheaper level. Run `--level max`
  instead. The level goes in the posted body (`post_review.py`) and in the chat report.
- A finder must attest every assigned file exactly once in `reviewed_files`; a coverage gap or
  duplicate receipt degrades the review and must be disclosed, never summarized as clean.
  At the inline levels you are the finder: name any unit you did not read.
- A single call site never proves a signature or a field. `{ results: users }` reads
  `results`.
- Only changed or newly-reachable lines. Pre-existing debt appears only if the change makes it
  worse, and says so.
- A suggestion is posted only after it was applied in the worktree and passed the fastest
  relevant check.
- A clean supplied checkout at the exact review head is the review checkout. Do not create a
  redundant detached worktree, and never clean up a reused caller checkout.
- Inability to verify is not a refutation, and intent is not correctness — a skeptic needs
  evidence to kill a candidate; an `intent`-lens defect is not refuted by "the author meant it".
- Never resolve threads, never approve or merge through any other path, never post twice for
  one head.

| Rationalisation | Reality |
|---|---|
| "The diff makes it obvious, no need to open the callee" | !5581: the "obvious" empty-string bug was blocked by `@MinLength(1)` upstream and filtered downstream. Open it. |
| "It's only `high`, so a looser hop is fine" | The level buys fewer lenses and no skeptics, never a lower evidence bar. Without the hops there is no candidate to screen. |
| "I'll spawn a couple of finders to make `high` better" | Then you ran `max` and disclosed `high`. The level is what the author is told; change the level, not the fan-out. |
| "Two skeptics per candidate is expensive" | One false positive costs the author a reply and the next real finding its credibility. Cut candidates (`maxCandidates`), not skeptics — the 2026-08-28 measurement showed finders exploring by hand (105 turns each) were the cost, not the skeptics. |
| "The finder needs to grep around to be sure" | The pack already holds its diff slice, definitions and one-hop neighbours; 10 messages is the budget. Before packs, finders spent 30 turns grepping and never managed to read the 462 KB diff at all. |
| "It's in CLAUDE.md so I'll flag it" | Only if the rule text names it, the glob matches, and it's on a changed line. |
| "Post now, verify after" | The read-back table is the deliverable. |
| "No findings feels weak" | An approve with a traced `verified_ok` list is the strongest review there is. |
