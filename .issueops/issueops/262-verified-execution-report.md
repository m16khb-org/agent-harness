# #262 verification report

## Execution provenance and lifecycle disclosure

- Durable IssueOps lifecycle: `io-8ba44d4383fb`, generation 1.
- Durable execution mode is **direct** because the coordinator created branch
  `262-orca-plan-readiness` before execution prepare. Direct preview/confirm
  completed before the coordinator's stop message arrived.
- The actual implementation process was launched by Orca Run
  `run_36d1bc632484`, task `task_a6b236973bca`, terminal
  `term_8575ec23-425f-48b2-924e-bb89cc348329`.
- These facts describe different authorities: durable IssueOps mode remains
  direct while process launch provenance is Orca. This run is **not** evidence
  of successful IssueOps mode-orca dogfood. The main coordinator will close the
  true mode-orca proof in issue #248 after the fixed binary is built.
- Per coordinator instruction, no `switch-mode`, `release`, `replace`, or other
  execution-state mutation was run after the stop. Lifecycle execution remains
  owned by the current active holder.

## Acceptance matrix

| Acceptance | Test / evidence | Result |
| --- | --- | --- |
| Fresh auto/Orca prepare rejects a missing child plan before external mutation | `TestRequireStagedExecutionOwnerPlanArtifact`; `TestIssueOpsPreparationPlanArtifactFailureStopsBeforeOwnerMutations` | GREEN |
| `parent_plan_path` alone is not child readiness | `delegation parent plan alone does not satisfy readiness` table row | GREEN |
| Worktree receipt materializes, seals, and atomically persists the durable plan | `TestPrepareExecutionOwnerMaterializesPlanAndSealsManifest`; `TestOrcaIntentWorktreeReceiptPersistsPlanBeforeNextIntent`; repository CAS assertion | GREEN |
| Replacement preserves an existing in-worktree plan identity and rejects missing/outside/mismatch | `TestReplacementResealRequiresExistingPlanIdentity` | GREEN |
| Resume requires matching manifest, sealed file, and durable plan before mutation | `TestResumePlanIdentityRejectsUnsealedOrDriftedPlan`; wiring zero-mutation assertion | GREEN |
| Clean released Orca recovery can link/stage a plan only for the next reseed | `TestArtifactStagingReleasedRecoveryPredicate`; `TestReleasedArtifactRecoveryLinksPlanBeforeStaging`; sealed packet byte comparison | GREEN |
| Active/claimable/revoking/completed/direct near-misses remain blocked | core predicate matrix and lifecycle exact-command matrix | GREEN |
| Cross-lifecycle recovery commands cannot borrow another released worktree's hook authority | `TestExecutionReleasedOrcaRejectsCrossLifecyclePlanRecovery` covers stage and link | GREEN |
| Direct mode and GitHub/GitLab identity remain compatible | focused, full normal, and full race verification | GREEN |

## RED evidence

- Task 1 focused test failed to compile because `PlanIdentity` and
  `RequireStagedExecutionOwnerPlan` did not exist; the app path reached the
  remote issue reader and failed with `remote issue snapshot identity or
  bounded body is invalid` instead of a plan-readiness error.
- Task 2 focused test failed to compile because `OwnerArtifacts.PlanPath` and
  `materializeExecutionOwnerArtifacts` did not exist.
- Task 3 accepted an empty plan manifest and ignored missing/outside/drifted
  durable `PlanPath`; sealed-plan failures returned unrelated generic errors.
- Task 4 core had no released-recovery predicate and rejected every execution
  solely because `Execution != nil`. The lifecycle focused test returned
  `unclassified shell command is blocked` for exact plan staging.
- Source-to-plan readback exposed a second Task 4 recovery blocker: exact
  released `link-plan` returned `IssueOps execution generation 3 has no active
  write lease`, making the documented `link → stage → reseed` recovery sequence
  impossible until the same clean-released predicate was applied.
- One composite read command containing a pipe was blocked by the active holder
  hook with code `unsafe_mutation` and reason `shell substitution or wrapper
  target is not statically resolvable; use one exact foreground command with
  literal paths`. It was not bypassed; subsequent reads were split into literal
  single commands.
- The first documentation regression asserted the earlier #163
  `gh issue develop --name` sequence and failed for all four target documents.
  The coordinator then supplied the superseding #176 evidence, so the test was
  corrected before any documentation used that obsolete command. With the
  #176 expectation, the same test failed for all four documents until their
  complete order was explicit.
- A literal `gh issue develop --help` read was blocked with code
  `unsafe_mutation` because the active IssueOps hook classifies it as an
  unrecognized shell command. The guard was not bypassed or disabled; existing
  repository command grammar and the verified #176 commit were used instead.
- The first major-package verification run exposed seven pre-existing Orca
  preparation fixtures that did not stage a plan. They failed with
  `Orca execution requires a staged plan artifact`, or with a nil pending
  execution caused by that same early failure. The fixtures were updated to
  stage a non-empty plan; the exact failing tests and the major-package suite
  then passed.
- Independent Brooks review found that a released lifecycle A could authorize
  an exact recovery command whose `--id` named lifecycle B when the file/cwd
  selected A's worktree. `TestExecutionReleasedOrcaRejectsCrossLifecyclePlanRecovery`
  reproduced the defect with `Decision: allow` before the parser returned and
  bound the command target ID.

## GREEN implementation

- Added one typed `orca_plan_artifact_required` contract with `missing: [plan]`
  and context-specific stage or replacement-preview action.
- Added fresh staged-plan preflight before remote issue evidence and before
  intent/workspace/lease mutation.
- Materialized the staged plan at `.issueops/artifact/plan.md`, returned
  `OwnerArtifacts.PlanPath`, and persisted it in the worktree-receipt CAS before
  terminal/Run/task/dispatch stages.
- Required exact staged, sealed, and durable SHA-256 identity for replacement
  reseal; replacement never invents or changes `PlanPath`. Resume deliberately
  uses the sealed packet's manifest digest, the private sealed plan file, and
  the durable in-worktree `PlanPath` as its three-part trust root.
- Allowed recovery link/stage only for a clean released Orca generation with no
  holder, pending intent, or completion. Staging changes only the next reseal
  input; `execution replace --reseed` remains mandatory before resume.
- Bound exact released `artifact stage` and `link-plan` recovery commands to the
  same lifecycle ID as the worktree-matched record, closing cross-lifecycle
  authority borrowing.
- Updated the active IssueOps skill, execution/operational-start references,
  operations guide, and testing contract with the corrected artifact-stage
  flags and the #176 GitHub Orca order: record the sealed base without a link,
  stage the plan, let Orca create the local-only branch/worktree, call GraphQL
  `createLinkedBranch` with the sealed SHA as `oid`, then record
  `--link-verified`. The superseded `gh issue develop --name` mutation is
  explicitly excluded.

## GitHub branch-order evidence

- Issue #163 was re-read after its completion. Its experiment established the
  local-only prerequisite: a same-name remote branch makes provider linking
  fail, while an Orca-created branch that exists only locally can be linked
  afterward.
- Issue #176 superseded the branch-creation command while preserving that
  ordering. Local commit `386c605ca9704638a3f42cd84a04a80728889979`
  (`fix(branchprepare): linked branch를 봉인 base SHA에 못박는다 (#176)`) records
  that `gh issue develop --base <branch>` resolves the branch HEAD at link time
  and can diverge from the sealed base. Its verified replacement is the
  two-command GitHub API path ending in GraphQL `createLinkedBranch`, with the
  sealed base SHA passed directly as `oid`.
- `TestIssueOpsDocumentationPreservesGitHubOrcaBranchOrdering` requires the
  complete #176 order in `SKILL.md`, `OPERATIONS.md`, `execution.md`, and
  `operational-start.md`, and rejects any reintroduced
  `gh issue develop --name` mutation.

## Review disposition

- An external bounded Brooks review returned `revise` because resume does not
  reread the staged plan digest. That finding was technically rejected after
  applying the receiving-code-review procedure. Approved plan Task 3 requires
  `artifact_manifest.plan`, the sealed private plan file, and the durable
  `PlanPath`; `validateExecutionResumePacket` implements exactly those checks
  before operation allocation. Staging is fresh prepare/reseed input, not a
  fourth resume authority. Requiring it again would couple a sealed generation
  to separate input state that is not part of the resume trust root.
- The fresh internal bounded xhigh Brooks review returned `revise` with one
  high cross-lifecycle hook finding, one documentation-order ambiguity, and
  stale pending report fields. The high finding was reproduced RED and fixed
  by binding the parsed target ID. Operational-start now separates the
  coordinator's exact `createLinkedBranch` step from the owner's later claim
  and link-verification recorder without changing the #176 branch order. This
  report resolves the stale fields below.
- A second fresh bounded xhigh Brooks review inspected the corrected full diff
  and returned `proceed` with no blocker, high, medium, or low findings. Its
  only residual risk is the live #248 Orca/provider recovery path assigned to
  the main coordinator; it did not classify that deferred dogfood as an
  implementation blocker.

## Shannon quality evidence

- Diff inventory at measurement time: 26 tracked modifications and five
  untracked issue files; untracked Go tests were included explicitly.
- Diff-only code heuristic: 1,303 added Go lines, with 1,300 signal candidates
  and three comment/debug noise candidates (`SNR=0.9977`). Production-only was
  268/271 (`SNR=0.9889`); tests were 1,032/1,032.
- Structural overhead in the 271 added production Go lines was 18 lines
  (`6.64%`). The heuristic is lexical, while correctness/unused/concurrency
  evidence is backed by full normal tests, race tests, and `go vet`.
- Cleanup reduced the durable-plan helper from three return channels to two,
  removed a duplicate plan digest computation, normalized recovery actor host
  identity once, and removed the empty-manifest compatibility claim. No global
  tool was installed. Repo-local `quality inspect` was attempted but the active
  lifecycle hook correctly blocked it as an unclassified shell command; the
  guard was not bypassed.

## Verification log

- Task 1 focused RED then GREEN — pass.
- Task 2 focused core and application/outbound preparation tests — pass.
- Task 3 focused resume tests and response-contract goldens — pass.
- Task 4 focused core/lifecycle RED then GREEN — pass.
- Task 5 GitHub Orca documentation contract RED then GREEN — pass:
  `go test ./internal/core/issueops -run TestIssueOpsDocumentationPreservesGitHubOrcaBranchOrdering -count=1`.
- Major package suite — pass after plan-ready fixture correction.
- Both response-contract goldens — pass.
- `go test ./... -count=1` — pass.
- `go test -race ./... -count=1` — pass, no race report.
- `go vet ./...` — pass.
- `go build -o bin/issueops ./cmd/issueops` — pass; repo-local output only.
- `git diff --check` — pass.
- Shannon AI-slop comparison — pass with the lexical caveat recorded above.
- Fresh independent bounded xhigh Brooks implementation review — initial
  `revise`; all actionable findings resolved. Post-fix fresh review —
  `proceed`, no findings.
- Issue #248 live recovery — intentionally not run in this execution; reserved
  for the main coordinator with the exact fixed binary. It remains pending and
  is not claimed as #262 evidence.
- User-scope binary installation — intentionally not run.
