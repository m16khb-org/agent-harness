# IssueOps devil's-advocate loop — design spec

> Date: 2026-07-01
> Status: design (spec-only; implementation gated on separate approval)
> Scope: close two gaps so the design-review devil's-advocate becomes real loop
> engineering — a fail-closed gate (Gap A) whose surfaced problems are reflected
> into the remote issue (Gap B) before the cycle re-runs.

## 1. Problem

Today the design-review devil's-advocate is only *skill-mandated*, and its output stays
local:

- **Gap A — not a machine gate.** `skills/issueops/SKILL.md` says Brooks "MUST
  run" and "No implementation until the design review is approved", but the
  state machine has no invariant that a devil's-advocate review actually ran.
  `IssueOpsImplementationReadiness` (`internal/core/issueops/issueops_readiness.go:107`)
  requires `design_review`, `compatibility_review`, and an `ExecutionDecision`;
  `devils-advocate-review` is merely one of twelve valid `subagent_pattern`
  enum values (`internal/core/issueops/executiondecision/execution_decision.go:21`).
  A cycle can reach `implement` without Brooks ever running.
- **Gap B — findings stay local.** On a `stop` verdict,
  `RegressIssueOpsForReplan` (`internal/core/issueops/issueops_regress.go:19`)
  regresses `plan`/`compatibility-review` → `grill`, records the stop as a local
  scope `Decision`, clears `DesignReview.Approved`, and marks ledger entries
  stale. It explicitly "does not ... remote artifacts" — the surfaced problems
  are never written back to the GitHub/GitLab issue.

Target loop:

```
grill → (plan authored) → design-review devil's-advocate [REQUIRED, verdict recorded]
   ├─ pass   → compatibility-review → implement → …
   ├─ revise → resolved in plan, or explicitly waived (rationale) → proceed
   └─ stop   → findings reflected into remote issue body → regress → grill (re-plan)
```

## 2. Non-goals

- The harness does **not** auto-run Brooks. The agent still spawns the
  devil's-advocate sub-agent (pattern #4); the harness only records the verdict
  and gates on it. (Consistent with how design/compatibility reviews work.)
- No change to the `verified-execution` PR-phase adversarial reviewer.
- Reflection-to-issue is required only on the `stop` → regress path; a `revise`
  verdict is resolved/waived in place and issue reflection is optional there.
- Waiver stays available (with mandatory rationale) so the gate is strict, not
  tyrannical.

## 3. Gap A — first-class devil's-advocate review + fail-closed gate

### 3.1 Record (mirror DesignReview / CompatibilityReview)

Add to `internal/core/issueops/model/types.go`, mirroring
`IssueOpsCompatibilityReview` (`:217`) and its `*Request`:

```go
type IssueOpsDevilsAdvocateReview struct {
    Verdict          string   `json:"verdict"`            // pass | revise | stop
    Findings         []string `json:"findings,omitempty"` // surfaced problems
    Waived           bool     `json:"waived,omitempty"`   // stop/revise explicitly waived
    WaiverRationale  string   `json:"waiver_rationale,omitempty"`
    ReviewerPattern  string   `json:"reviewer_pattern,omitempty"` // "devils-advocate-review"
    RecordedAt       string   `json:"recorded_at"`
    IssueReflectedAt string   `json:"issue_reflected_at,omitempty"` // Gap B stamp
}

type IssueOpsDevilsAdvocateReviewRequest struct {
    Verdict         string
    Findings        []string
    Waived          bool
    WaiverRationale string
}
```

Record field (`model/types.go`, alongside `DesignReview`/`CompatibilityReview`):

```go
DevilsAdvocateReview *IssueOpsDevilsAdvocateReview `json:"devils_advocate_review,omitempty"`
```

Validation (in a new `internal/core/issueops/devilsadvocate/` package or the
existing recorder file, matching the compatibility-review recorder shape):

- `Verdict` ∈ {pass, revise, stop} (else error).
- `stop`/`revise` require either concrete `Findings` **or** `Waived=true`.
- `Waived=true` requires non-empty `WaiverRationale`.
- `pass` clears any prior waiver.

### 3.2 Gate

Extend `IssueOpsImplementationReadiness` with a `devils_advocate_review` missing
key (same tier as `design_review`/`compatibility_review`):

- missing when `record.DevilsAdvocateReview == nil` or `RecordedAt` empty;
- missing when `Verdict ∈ {stop, revise}` and not `Waived` (unresolved
  adversarial verdict blocks implementation).

So `implement` is fail-closed until Brooks ran **and** the verdict is `pass`, or a
`stop`/`revise` was explicitly waived with rationale.

### 3.3 Recorders

- CLI: `issueops devils-advocate review --id ID --verdict pass|revise|stop
  [--finding TEXT]... [--waive --waiver-rationale TEXT] [--json]`
  (verb `review` mirrors `issueops design review` / `issueops compatibility
  review`).
- MCP: `issueops_record_devils_advocate_review` (mirrors
  `issueops_record_compatibility_review`).

### 3.4 Regress coupling

`RegressIssueOpsForReplan` already consumes a `stop`. Change:

- Regress precondition (Gap B): allowed only when
  `DevilsAdvocateReview.Verdict == "stop"` **and** `IssueReflectedAt != ""`
  (findings were written to the issue first).
- On regress, clear the devil's-advocate review (like `DesignReview.Approved`
  is cleared) so the re-planned cycle must earn a fresh verdict before
  `implement` — the gate re-fires.

## 4. Gap B — reflect findings into the remote issue body

### 4.1 New provider method

`port.IssueProvider` gains (`internal/port/provider.go`):

```go
UpdateIssueBodySection(req IssueProviderUpdateIssueBodySectionRequest) (IssueProviderUpdateIssueBodySectionResult, error)
```

- Request: `Repo`, `IssueURL`, `SectionTitle` (e.g. "Devil's-advocate findings"),
  `Body` (rendered findings markdown), `Confirm`.
- Result: `OK`, `URL`, `Updated bool`, `Preview string` (dry-run when
  `Confirm=false`, like every other provider op).

### 4.2 Idempotent section merge

Wrap the managed section in HTML-comment delimiters so re-runs replace, never
duplicate:

```
<!-- issueops:devils-advocate:start -->
## Devil's-advocate findings (<RFC3339>)
- <finding 1>
- <finding 2>
<!-- issueops:devils-advocate:end -->
```

- **GitHub** (`internal/adapter/provider/github/provider.go`): `gh issue view
  <url> --json body` → splice/replace the delimited block in the body → `gh
  issue edit <url> --body <merged>`.
- **GitLab** (`internal/adapter/provider/gitlab/provider.go`): `glab api
  projects/:proj/issues/:iid` (GET `description`) → splice → PUT
  `description`. Reuse `remoteparse.SplitGitLabIssuePath` + `url.PathEscape`
  (same shape as `fetchGitLabIssueArtifact`).

### 4.3 Reflection command + stamp

- CLI: `issueops remote reflect-devils-advocate --id ID [--confirm] [--json]` —
  reads `record.DevilsAdvocateReview.Findings`, renders the section, calls
  `UpdateIssueBodySection`, and on a confirmed success stamps
  `IssueReflectedAt` on the record.
- MCP: `issueops_remote_reflect_devils_advocate`.
- Without `--confirm`: dry-run preview only (no remote write, no stamp).

### 4.4 Enforced order

```
1. agent spawns design-review (sub-agent) on the plan
2. issueops devils-advocate review --verdict stop --finding …     (Gate A recorded)
3. issueops remote reflect-devils-advocate --confirm              (writes issue body; stamps IssueReflectedAt)
4. issueops regress                                              (fail-closed unless verdict=stop AND IssueReflectedAt set) → grill
5. re-plan → fresh design-review review required again before implement (Gate A re-fires)
```

## 5. Contract / golden / catalog impact

- New record field → regenerate `cmd/issueops/testdata/response_contracts.golden.json`.
- New CLI subcommands + 2 new MCP tools → update `internal/adapter/mcp`
  catalog + regenerate `cmd/issueops/testdata/mcp_tools.golden.json` and any
  usage golden (per CAUTIONS §7 / §27, CONVENTIONS §4).
- New `port.IssueProvider` method → implement in **both** github and gitlab
  adapters (interface change breaks compile until both satisfy it).
- ADR: record the "devil's-advocate is a first-class fail-closed gate + issue
  reflection" decision in `.issueops/ADR.md`.
- SKILL: update `skills/issueops/SKILL.md` plan/compatibility-review rows and the
  design-review routing row so the prose matches the now-enforced contract, including
  the new commands.

## 6. Test matrix

- Record/validate: pass / revise / stop; stop-without-findings-without-waive
  rejected; waive-without-rationale rejected.
- Gate: implement blocked when review missing; blocked on stop/revise unwaived;
  allowed on pass; allowed on waived stop/revise.
- Provider `UpdateIssueBodySection`: dry-run preview; confirm writes; idempotent
  re-run replaces the delimited block (no duplication); github + gitlab via fake
  gh/glab (mirror existing provider tests).
- Reflect command: dry-run no-write; confirm stamps `IssueReflectedAt`.
- Regress ordering: regress rejected when `IssueReflectedAt` empty; accepted
  after reflect; regress clears the review so the gate re-fires.

## 7. Risks / open points

- **Body-write is outward-facing and harder to reverse** than local state — hence
  confirm-gated + dry-run + idempotent delimiters. A malformed merge could
  clobber the issue body; the merge must be delimiter-scoped and never touch
  content outside the block.
- **GitLab description PUT** replaces the whole description; the splice must
  round-trip the untouched remainder exactly.
- **Contract churn** is broad (record + 2 tools + provider interface + goldens +
  SKILL + ADR). Best executed as its own IssueOps cycle.
- Naming: `devils-advocate review` vs `devils-advocate record` — chosen `review`
  for parity with design/compatibility review; revisit if it collides with the
  did-you-mean suggester.

## 8. Verification (definition of done, when implemented)

- `gofmt`/`go build ./...`/targeted package tests green; goldens regenerated with
  intentional-only diffs.
- E2E against fake gh/glab: implement blocked without a verdict; stop → reflect
  (issue body section written idempotently) → regress → grill; re-plan re-fires
  the gate.
- ADR + SKILL updated in the same change.
