# Pioneer Skill Optimization Strategy (firsthand dogfood)

Date: 2026-06-11

Method: the main agent invokes each pioneer skill directly via the Skill tool and applies it to a real task in
this repo, then records ① strengths observed firsthand, ② friction the skill itself caused (activation point,
branching judgement, token cost, doc navigability — things sub-agent holdouts do not surface), and ③ a concrete
optimization strategy classified as EDIT (skill body needs a change), MEASURE (needs more sampling, not editing),
or HOLD (already good; do not touch).

Complements the sub-agent measurement in `pioneer-skill-quality-scorecard.md` and
`evidence/pioneer-skills-quality/reruns/`. This file is the firsthand-use layer.

Decision rule (keep/discard): only an EDIT strategy when there is a firsthand-observed, reproducible gap that a
small change closes without adding complexity. Otherwise MEASURE or HOLD.

## Round 1

### code-quality-metrics — firsthand: measured `ae5832a..a859773` (this session's commits)

Strengths (HOLD): zero-input guard works; staged/unstaged/untracked discipline is explicit; "label heuristics as
approximate" is honest; the metric card is reproducible. The method is sound.

Friction observed firsthand (sub-agent holdouts did not fully surface these):
1. **Portability defect — the skill's own commands fail on this host.** Every documented measurement command uses
   `grep '^\+[^+]'` / `grep -cE '^\+...'`. On this machine `grep` is aliased to `ugrep`, which rejects the `^\+`
   pattern with "invalid syntax at position 5". The SNR command returned `insufficient-input` (a false zero) until
   re-run with `command grep`. This reproduced twice (my run + the earlier SHANNON-H1 sub-agent, which silently
   worked around it). A measurement skill whose commands mis-measure on a common host is a real defect.
2. **Domain scope gap.** code-quality-metrics's four metrics (SNR=code noise, entropy=cyclomatic, redundancy=code blocks,
   overhead=boilerplate) assume source code. The actual change set here was 100% markdown (83/83 lines: 25 table
   rows, headers, prose; 0 Go lines). SNR computed to 0.97 but is meaningless for prose. The skill has no mode for
   docs/config/markdown diffs and does not warn that its metrics are code-only.

Optimization strategy:
- **EDIT (high)**: make the documented commands ugrep/rg-alias-safe — use `command grep` (or `git diff | grep`
  with a pattern ugrep accepts) and add a one-line note: "if `grep` is aliased (ugrep/rg), these patterns need
  `command grep` or they silently mis-measure."
- **EDIT (medium)**: add a short "non-code diffs" rule — when the change set is predominantly markdown/docs/config,
  report scope and mark SNR/entropy/redundancy/overhead as N/A (code-only), or fall back to line-delta reporting.
- **HOLD**: the zero-input guard, scope discipline, and metric-card structure — already good.

### debugging — firsthand: diagnosed the real pre-existing `TestResponseContractsGolden` failure

Outcome (the method works): debugging's 7-step flow drove straight to a confirmed root cause. REPRODUCE → ISOLATE
(regenerate golden with `-update` in place, `git diff`, `git restore`) → HYPOTHESIZE → VERIFY in four steps.

Root cause found: the response-contract golden is **non-hermetic**. The snapshot captures the live
`$ISSUEOPS_ROOT/.issueops/` docs index, which includes the **gitignored `evidence/` subtree**. The committed
golden (regenerated in ae5832a) baked in references to local-only evidence files present at that moment
(`reruns/web-research/fixture-request.md`, `database-design/analysis.md`, …); any other working tree indexes different
evidence files (e.g. `baseline-27-case-results.md`, 30844 bytes) and the golden mismatches. The earlier `?? .env`
manifestation is the same class (the worker fixture captured an untracked file's `git status`). This is a real
harness defect (docs index / golden should exclude the ignored `evidence/` subtree, or scan a fixture dir), filed
here as a finding — it is a Go change, separate from the skill-quality scope.

Strengths (HOLD): the 7-step method, the falsifiable-hypothesis discipline, the cross-language tables, and the
"reproduce before diagnosing" rule are all excellent and did the job.

Friction observed firsthand:
1. **No snapshot/golden-test isolation pattern.** Step 3 offers Bisect / Divide&Conquer / Trace-Diff, but the
   single most common Go failure class — a golden/snapshot mismatch — is fastest isolated by "regenerate with
   `-update` into a scratch copy, `git diff`, then restore." debugging doesn't mention this; I had to know it.
2. **Step 2 lint_diagnose assumes the daemon/MCP is up.** The `issueops` MCP disconnected mid-session, so
   the MCP `lint_diagnose` form was unavailable. The skill lists skip conditions but not "daemon/MCP unavailable",
   and for this golden failure lint_diagnose was the wrong first move anyway (regenerate-and-diff was).

Optimization strategy:
- **EDIT (medium)**: add a "Strategy D: Snapshot/Golden diff" to Step 3 — regenerate the golden/snapshot into a
  scratch location (or in place + `git restore`), `git diff` for the exact divergence, then restore. This is the
  highest-frequency Go isolation move and is currently missing.
- **EDIT (low)**: add "daemon/MCP unavailable" to the lint_diagnose skip list in Step 2.
- **HOLD**: the 7-step backbone — proven effective firsthand.

### git-operations — firsthand: dated the golden pollution via history archaeology

Outcome (the method works): used git-operations' History Analysis toolset directly.
`git log -S'.issueops/evidence/pioneer-skills-quality' -- <golden>` pinpointed the polluting commit instantly
= **ae5832a** (whose own message says "Regenerate the response contract golden after the docs index changed").
Confirmed the current committed golden carries **120 lines** of gitignored `issueops/evidence` paths, and
`git check-ignore -v` confirmed `.gitignore:16:evidence`. This independently corroborates and dates the debugging
root cause.

Strengths (HOLD): the History Analysis & Recovery section (`git log -S`, `--follow`, blame, `check-ignore`) is the
right toolset and worked firsthand; "read state first" (`git status --short`) is good discipline. The variance
study already proved the destructive-op confirmation ladders are correct, so those stay untouched.

Friction observed firsthand:
1. **No read-only "investigation/archaeology" entry point.** git-operations is organized around mutating ops (rebase,
   reset, cherry-pick, conflict) with elaborate safety ladders. Pure read-only history investigation — a large
   share of real advanced-git use — only appears under "History Analysis & **Recovery**", framed as recovering
   lost data. I had the right commands but had to extract them from a recovery-framed section.

Optimization strategy:
- **HOLD** (primary): operations, safety ladders, and confirmation rules — proven correct by the n=3 variance
  study and effective firsthand.
- **EDIT (low)**: add a one-line "read-only archaeology" pointer near the top of History Analysis noting these
  commands serve investigation (understanding history), not only lost-data recovery.

## Cross-round finding (harness defect, not a skill issue)

Dogfooding debugging + git-operations surfaced a real harness bug: `TestResponseContractsGolden` is non-hermetic. The
response-contract snapshot's docs index scans the live `$ISSUEOPS_ROOT/.issueops/` tree **including the
gitignored `evidence/` subtree**, so commit ae5832a baked 120 lines of local-only evidence paths into the tracked
golden. Any other working tree fails the golden. Fix (Go, separate from skill scope): exclude the ignored
`evidence/` subtree from the docs index (or scan a hermetic fixture dir) and regenerate the golden. This explains
the "pre-existing, unrelated" golden failure noted earlier in the session.

## Round 2

### algorithm-optimization — firsthand: decided no-change on the bounded startup loop

Outcome (the method works, and it points the right way): applied Step 1 (profile, don't guess) to `profile.txt`.
dedupe_pairs is **0.057%** of startup; network I/O is **99.94%**; N≤300 (worst case 90,000 ops ≈ 0.09 ms). The
Stop Rule ("I/O-bound → do not optimize further") and NEVER rule ("Optimize O(n) code that runs once at startup")
unambiguously yield **no-change**, with the threshold where it would matter (n² approaching ~0.5 s I/O → N≈22,000).
Applied directly by the main agent, the skill drives cleanly to the correct decision — confirming the n=3 variance
finding that the skill is correct and the sub-agent rewrites were discipline lapses, not skill defects.

Strengths (HOLD): the method is rigorous; the Stop Rules and NEVER rules are crystal clear; the complexity
reference card and scaling-test protocol are excellent.

Friction observed firsthand:
1. **The "should I optimize at all?" gate is buried at the very bottom (lines ~498/516).** A reader must load ~500
   lines (structured-programming essay, 6-step method, huge classification tables, concurrency section) before
   reaching the gating Stop Rules / NEVER rules that actually decide this case. This is almost certainly *why* some
   sub-agents optimized anyway — they internalized the optimization machinery up top and met the no-change gate too
   late. Same friction class as git-operations: the key decision rule is buried.

Optimization strategy:
- **HOLD**: the method content — correct and rigorous; variance-confirmed.
- **EDIT (medium, highest-value of this round)**: hoist a short "Optimize-at-all? gate" to the TOP, before the
  6-step deep dive: "First check the profile. If the code is off the hot path, bounded-N startup, or I/O-bound →
  STOP: recommend no-change and state the input-size threshold that would change the decision. Only enter the
  method if CPU is the proven bottleneck." This directly attacks the measured executor variance.

### database-design — firsthand: applied to the events-table task; confirmed a structural verification gap

Outcome: invoked database-design and applied Step 4. The multi-index CRITICAL rule I added earlier is present and well-placed
(candidate A full composite vs. candidate B partial `WHERE type='click'`, chosen by selectivity + insert rate).
The skill is the most thorough of the nine.

Friction observed firsthand:
1. **Verification gold standard is structurally unreachable in the common agent context.** database-design's NEVER rule
   ("Recommend DDL without the full before/after EXPLAIN ANALYZE evidence") and all of Step 7 assume a live DB
   connection. In issueops — and most agent sessions — there is **no DB**, so every database-design recommendation can
   only ever be "estimated + run EXPLAIN to confirm." The "or state the missing-input blocker" escape (which I
   added to the index rule) exists, but the NEVER rule / Step 7 are still framed as hard requirements, creating a
   permanent tension every database-design run hits.
2. **Applicability gate missing.** database-design is 1000+ lines (longest skill) yet this repo has no RDBMS at all — database-design is
   N/A here. There's no quick top-of-file "does a relational DB even exist?" gate (same buried-gate pattern as
   algorithm-optimization/git-operations).

Optimization strategy:
- **HOLD**: the method, the just-added multi-index rule, the live-DDL safety gate, engine-portability tables.
- **EDIT (medium)**: add an explicit "Advisory mode (no live DB)" note at Step 1/Step 7 — when no DB connection is
  available, database-design outputs the recommendation + the exact EXPLAIN/row-count commands for the operator + marks it
  UNVERIFIED. Clarify the EXPLAIN NEVER-rule governs *claiming an optimization works*, not *advising one*. This
  resolves the structural tension and matches what every real run already does.
- **EDIT (low)**: add an early applicability gate ("no relational DB / pure file-state repo → database-design does not apply").

### prompt-engineering — firsthand: critiqued my own holdout sub-agent prompt; it caught a real bug

Outcome (strong validation): applied Phase 4 DIAGNOSE to the holdout sub-agent prompt I wrote ad hoc this session.
The method immediately flagged the exact bug that caused the SHANNON-H1 misfire earlier: the user-request text said
"measure my current work in **this repo**" while a separate STRICT RULE said "work ONLY inside /tmp/...fixture" —
conflicting location signals, so the sub-agent measured the wrong repo. prompt-engineering's "ambiguous → single source of
truth" rule names this precisely. The privacy + fictional-tool adversarial rows I added earlier are present and
read well in context.

Improved prompt (key changes): put the workspace path as the single authority **inside** the request ("the repo
at <path>"), drop the conflicting "this repo" phrasing, and add "Output ONLY these four sections — no preamble" to
the format spec.

Friction observed firsthand:
1. **No lightweight / one-shot mode.** prompt-engineering mandates a 5-case TEST SUITE, A/B testing, and versioned storage
   under `.issueops/prompt-engineering/prompts/` ("NEVER publish a prompt without a test suite"). That is right for a
   reused production prompt, but disproportionate for an ephemeral orchestration prompt used once per sub-agent
   (like mine). The full ceremony does not fit inline, single-use prompts. Same proportionality friction class as
   verified-execution.

Optimization strategy:
- **HOLD**: the 5-phase method, the privacy/fictional-tool guardrails (just added), the Patterns Library.
- **EDIT (medium)**: add a proportionality note distinguishing a **reused/production prompt** (full method + test
  suite + versioning) from a **one-shot/orchestration prompt** (lightweight: write the input/output contract +
  1–2 sanity checks; skip the test-suite/versioning ceremony). Removes the over-heavy friction for inline prompts.

## Round 3

### verified-execution — firsthand: applied to a 2-file validate + build; confirmed the proportionality gap

Outcome: captured proportionate evidence via the auxiliary CLI surface (both skills `Skill is valid!`, build exit
0, cleanup receipt). The auxiliary-surface allowance for CLI-shaped criteria is the right escape valve and worked.

Friction observed firsthand (confirms the TURING-H1 holdout concern):
1. **Ceremony does not scale down.** Applying verified-execution's *full* method to a 2-file validate still demands
   goals.json + ledger.jsonl + an evidence/ artifact + recomputing 5 metrics + a **mandatory Final Quality Gate
   with a binding adversarial-reviewer sub-agent** ("REVIEWER IS BINDING"). The auxiliary-surface escape covers the
   *channel* choice but NOT the *ceremony* (state files, metrics, reviewer gate are framed as always-required).
   The TURING-H1 holdout only passed because the sub-agent improvised proportionality — the skill does not grant it.

Optimization strategy:
- **HOLD**: the evidence discipline ("tests passing is never completion proof"), the 4 QA channels, the 12
  sub-agent patterns, cleanup-paired-with-evidence — excellent for real delivery.
- **EDIT (medium)**: add an explicit "proportionate mode for low-risk tasks" — for trivial/low-risk criteria
  (docs, single-file validate, config) allow auxiliary-surface evidence + a one-line ledger entry, and make the
  Final Quality Gate's adversarial-reviewer step conditional on risk (skip for trivially-reversible low-risk work).
  Directly closes the TURING-H1 gap and matches what the holdout had to improvise.

### implementation-planning — firsthand: correctly routed the golden fix to direct execution (no over-planning)

Outcome (activation discipline works): asked implementation-planning to decide the altitude for the golden-bug fix. Phase 0
classified it as a small Standard bugfix (root cause already known, ~1–2 files, no architectural impact, user did
not request a plan) and the planner-mode boundary correctly routed it to **direct execution**, surfacing the one
real design choice (exclude `evidence/` at the docs-index-builder level vs. the test fixture; recommended: builder
level). This is exactly the VON-NEUMANN-H1 behavior — it did NOT hijack a "fix X" into a heavyweight plan.

Strengths (HOLD): unlike algorithm-optimization/git-operations/database-design, implementation-planning puts its decision gate at the TOP (identity para +
Phase 0 + the explicit planner-mode boundary "imperative language alone does not force planning"). Applied
directly, it routes correctly. This is the model the other skills' buried gates should imitate.

Friction observed firsthand:
1. **No structured output for the no-plan decision.** All the machinery (Phase 1 Ground → Phase 2 Interview →
   clearance checks → draft → plan template) assumes you ARE planning. When implementation-planning correctly decides NOT to
   plan, it just bows out via the boundary note — there's no crisp "routing record" artifact. This is precisely the
   0.2 that VON-NEUMANN-H1 docked (correct routing, but no explicit routing note recorded).

Optimization strategy:
- **HOLD**: Phase 0 classification + the top-of-file planner-mode boundary — correct and well-placed.
- **EDIT (low)**: add a 3-line "routing decision" output for the no-plan case — `{decision: direct-execution,
  agent category, the single choice to confirm}` — so the decline-to-plan path produces a record instead of just
  exiting. Closes the VON-NEUMANN-H1 gap.

### web-research — firsthand: researched Go golden-test hermeticity (and it confirmed the fix direction)

Outcome (the method works and produced real value): fan-out search → fetched two primary sources → cross-checked →
cited findings. Confirmed: (1) golden files live in the go-ignored `testdata/` (the harness golden does); (2) the
established practice is to **normalize/limit non-deterministic content before comparison** — gotest.tools/golden
normalizes CRLF→LF ([pkg.go.dev/gotest.tools/v3/golden], retrieved 2026-06-11); (3) `-update` is local-only, never
CI ([ieftimov.com/posts/testing-in-go-golden-files], retrieved 2026-06-11). This directly validates the golden-bug
fix: the harness normalizes *paths* ($ISSUEOPS_ROOT, $STATE_DIR) but not the *set of files indexed*, so regenerating
`-update` locally (ae5832a) baked in local-only `evidence/` files. The input must be hermetic, not just path-normalized.

Strengths (HOLD): the research method, the FR-0/FR-1 fetch-resilience ladder, the access-control stop rules (no
login/paywall/CAPTCHA bypass), and the cited-report discipline are all excellent and safe.

Friction observed firsthand:
1. **Quick-lookup class vs. absolute Critical Rules.** Phase 0 has a "Quick lookup → don't over-engineer" intent
   class, but the Critical Rules still say ALWAYS write the report to `.issueops/research/<slug>.md` and NEVER
   single-source. For a tight 2-fetch lookup like this one, writing a full report file is disproportionate. The
   skill names the lightweight case but its absolute rules don't honor it. (Better than prompt-engineering/verified-execution, which lack
   the class entirely — but the same proportionality theme, now seen in a third skill.)

Optimization strategy:
- **HOLD**: research method, fetch-resilience, access-control safety, citation discipline.
- **EDIT (low)**: reconcile the Quick-lookup class with the Critical Rules — explicitly allow quick lookups to use
  inline citations and skip the `.issueops/research/` report file, so Phase 0 and the ALWAYS/NEVER rules agree.

## Cross-skill synthesis

Dogfooding all nine surfaced two dominant, repeating friction patterns plus two unique high-value items:

1. **Buried decision gate** (algorithm-optimization, database-design, git-operations; code-quality-metrics's non-code case). The "should I even act / which mode
   / does this apply?" gate sits at the *bottom* of long skills, so a reader internalizes the heavy machinery before
   reaching the rule that says "don't." **implementation-planning is the positive model** — its activation boundary is at the
   TOP (Phase 0 + identity), and applied firsthand it routed correctly with no over-planning. **Strategy: hoist each
   skill's decision/applicability gate to the top.** This is also the most likely cause of the measured sub-agent
   variance (algorithm-optimization/git-operations optimized/acted because the gate came too late).

2. **No proportionality / lightweight mode** (prompt-engineering, verified-execution, web-research; database-design's advisory case). Skills calibrated
   for high-stakes reuse over-ceremony simple/low-risk tasks (test suites, reviewer gates, report files, EXPLAIN).
   **Strategy: add an explicit lightweight mode to each** — one-shot prompt (prompt-engineering), low-risk proportionate
   evidence (verified-execution), quick lookup (web-research), advisory-no-DB (database-design).

3. **code-quality-metrics portability defect (unique, highest individual value)**: the skill's own measurement commands silently
   mis-measure on a host where `grep` is aliased to `ugrep`/`rg`. Real, reproduced twice. **Strategy: ugrep-safe
   commands + alias warning.**

4. **Harness defect (not a skill issue)**: the non-hermetic golden, diagnosed via debugging + git-operations and confirmed by
   web-research research. **Strategy (Go, separate): exclude the gitignored `evidence/` subtree from the docs index;
   regenerate the golden.** implementation-planning routed this to direct execution (not a plan).

### Recommended edit priority (all are skill-body/doc edits, reversible)

| Priority | Skill | Edit | Why |
|----------|-------|------|-----|
| 1 | code-quality-metrics | ugrep-safe commands + alias note | Skill's own commands fail on this host (reproduced) |
| 2 | algorithm-optimization | hoist "optimize-at-all?" gate to top | Attacks measured executor variance |
| 3 | verified-execution | proportionate low-risk mode | Confirmed TURING-H1 gap firsthand |
| 4 | debugging | add Snapshot/Golden isolation strategy | Most common Go failure class, missing |
| 5 | database-design | advisory-mode (no live DB) note | Resolves structural EXPLAIN tension |
| 6 | prompt-engineering | one-shot/orchestration prompt mode | Over-heavy for inline prompts |
| 7 | web-research | reconcile quick-lookup vs Critical Rules | Minor proportionality |
| 8 | implementation-planning | routing-record for no-plan case | Closes VON-NEUMANN-H1 0.2 gap |
| 9 | git-operations | read-only archaeology pointer | Low; mostly HOLD (variance-confirmed correct) |
| — | (harness) | hermetic docs index (Go) | Separate from skill scope; fixes the pre-existing golden failure |
