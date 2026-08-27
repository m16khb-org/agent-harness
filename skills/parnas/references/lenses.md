# Finder lenses

Each finder is an independent inspector with ONE lens. Finders never post; they
return candidate defects for adversarial verification. Run every applicable lens
in parallel. A lens is applicable when any changed file matches its `applies`
column (tags come from `context.json` → `files[].tags`).

| id | applies | lens |
|---|---|---|
| `logic` | always | Correctness of the changed control/data flow: wrong condition, off-by-one, null/undefined path, lost `0`/`''`/`false`, wrong field name, wrong signature, async misuse (missing await, unhandled rejection), state mutation order, error mapping. |
| `boundary` | always | Cross-file consequences: every changed exported symbol — who calls it, does every caller still hold? Every changed call — does the callee's real definition accept it? Renamed/removed fields, generated artifacts vs source, event payload shapes. |
| `security` | `security`, `controller`, `gateway`, `dto`, `ops`, or diff mentions auth/token/secret/permission/env | Auth/authz (object-level IDOR), input validation on external inputs, secrets in code/log/CI, injection (SQL/command/SSRF), server-side re-validation of client flags, sensitive data in logs/responses. |
| `data` | `db`, `migration`, or diff touches repository/query/transaction | N+1 in loops, unbounded reads, missing transaction boundary across multi-writes (state the partial-failure), index/order assumptions, `up()`/`down()` symmetry, destructive migrations, cross-domain DB access. |
| `async` | `async`, `gateway`, or diff touches kafka/queue/stream/websocket/retry/timeout | Ack-before-durable-write, lost idempotency, missing timeout/cancellation on external calls, backpressure/disconnect cleanup, retry storms, ordering assumptions, transaction-then-publish ordering. |
| `contract` | `dto`, `controller`, `gateway`, `generated` | Public API contract drift: DTO ↔ validators ↔ Swagger/OpenAPI ↔ actual behavior; proto ↔ generated TS; response shape/status/error docs; PATCH-vs-PUT and other repo conventions. |
| `tests` | always | Do the added/changed tests actually exercise the changed behavior? Would they fail before the fix? Missing regression for a bugfix, mocked-away risk that needs a real dependency, assertions that can't fail, tests asserting the wrong thing. |
| `rules` | always | Compliance with the repo rule pack (`context.json` → `rule_pack`): only rules whose globs match the changed file, only on changed lines, only when the rule text explicitly covers it. Cite the rule file. |
| `scope` | always | Description ↔ diff alignment against the CUMULATIVE diff: hidden scope (deploy/CI/migration/secret/generated changes not mentioned), missing migration/rollback notes, behavior changes not described, commits that contradict the description. |
| `intent` | when `linked_issues` is non-empty | Does the change do what the linked issue asks — each background claim, acceptance criterion, checklist item, and table row in the issue body mapped to code/tests in the diff? Report unmet criteria (with the issue line quoted), silent extra scope, and places where the issue's own analysis (e.g. "producer missing at X") is contradicted by the code. Cite `summary.md` Linked issues section lines. |

`mr_context.py` selects the applicable lenses from file tags and diff keywords and writes
them, with this table's `lens` text, to `<out_dir>/workflow_args.json`. Prior-review
history is handled by the `scoper` skeptic, not a finder lens.

## Finder prompt

The finder and skeptic prompts live in `references/workflow.js` (`finderPrompt`,
`skepticPrompt`) — that file is the single source of truth. On a host without the
`Workflow` tool, dispatch one sub-agent per lens with the `finderPrompt` string filled in
by hand (replace `${...}` with values from `workflow_args.json`), then three skeptics per
candidate with `skepticPrompt`, and apply the verdict rule from `verification.md`.
Give each finder the SAME context and a DIFFERENT lens; never share findings between finders.
Each finder returns at most `perLensCap` (6) candidates.

Reference copy of the finder contract (keep in sync with workflow.js):

```
You are one inspector in a formal design inspection. Lens: <lens id> — <lens text>.
Repository checkout at head: <worktree or repo_dir> (read-only; never edit, never checkout, never post).
Context pack: <out_dir>/summary.md (read first), <out_dir>/diff.patch (the cumulative diff).
Rule pack files are listed in summary.md; read the ones whose globs match the files you inspect.
<if codegraph: `codegraph explore "<symbol>"` prints definitions + call paths — use it before grep.>

Inspect ONLY through your lens. For every candidate defect you MUST, before reporting:
1. Open the real definition of every symbol the claim depends on (not inferred from a call site).
2. Walk one hop upstream (who calls / what validates the input before it gets here) and one hop
   downstream (who consumes the result) and state what you saw.
3. Confirm the defect is on lines this change added/modified (see hunks in summary.md), or that
   the change makes an existing problem newly reachable.
Report a candidate only if you can state a concrete failure scenario (input/state → wrong result).
Do NOT report: style, naming, things a linter/typechecker/CI catches, pre-existing issues on
untouched lines, speculative refactors, "consider adding" without a failure scenario.

Return JSON only:
{"lens":"<id>","inspected":["<files/symbols actually read>"],
 "candidates":[{"path":"...","new_line":N,"end_line":N|null,"severity":"critical|high|medium|low",
   "category":"bug|security|performance|business-logic|data|api-contract|test|rule|scope",
   "title":"<Korean, one sentence>","what":"<Korean>","why":"<Korean failure scenario>","how":"<Korean fix>",
   "evidence":["path:line — what it proves", ...],"upstream":"<what you saw>","downstream":"<what you saw>",
   "suggestion":"<replacement code or null>","rule":"<rule file or null>","confidence":0-100}],
 "verified_ok":[{"concern":"<우려, 한 구절>","why_ok":"<왜 괜찮은지 한 문장, 근거 file:line 포함>","loc":"file:line","thread":"<contradicted existing thread id or null>"}]}
```

Rules baked into the prompt: at most `perLensCap` candidates and 6 `verified_ok` per lens;
`suggestion` is required for `api-contract` and any one-line decorator/description/config
fix; a hop you cannot find caps confidence at 50 and is stated, never guessed.

```
```

Severity guide: `critical` = data loss/corruption, auth bypass, secret exposure, payment/quota
error; `high` = wrong behavior on a real path, partial writes, crash on reachable input;
`medium` = degraded behavior, missing timeout/cleanup, contract drift consumers will notice;
`low` = correctness-adjacent hygiene with a real (rare) scenario. Anything without a scenario is
not a finding.
