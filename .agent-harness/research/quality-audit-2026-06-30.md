# Agent-Harness Quality Audit — 2026-06-30

> Scope: IssueOps (core state machine + CLI/MCP surface + remote/providers), the 19 skills, and repo-wide health/conventions/doc-drift — investigated for concrete, evidence-backed **quality-improvement** opportunities.
> Method: 6 parallel dimension surveys → adversarial per-finding verification (each finding's cited file:line re-opened by an independent skeptic) → ranked synthesis. 25 sub-agents, ~1.8M tokens, 377 tool calls. Plus an independent main-agent cross-check (§ Independent cross-check) that corroborated the headline verdicts and surfaced one finding the fleet missed (LK-01).
> Baseline: `go build ./...` exit 0, `go vet ./...` exit 0. gofmt flags only `internal/adapter/hook/output.go`. ~3 TODO/FIXME/panic (all benign) in non-test code; ~26 `t.Skip` all legitimately env-gated.

## Executive Summary

**Overall verdict: structurally healthy, ship-quality core with concentrated edge-case and doc-drift debt.** The IssueOps state machine's headline P0/P1s from the prior `ISSUEOPS_AUDIT.md` (Start() locking, stale-scan TOCTOU, force-release reason, atomic writes) are **genuinely fixed**, layering rules hold (no `core→adapter` / `port→core` / `adapter→cmd` violations), and hooks fail open by design. **No P0/P1 survived adversarial verification** — every confirmed finding is P2 or below. The remaining debt clusters in three places:

1. **Concurrency/data-loss edge cases reintroduced *by* the fixes** — the very P0 remediations left narrow windows.
2. **The new phase-ledger feature mis-reporting status** (`regress` persists a sparse ledger that suppresses derivation).
3. **Audit/spec docs presenting already-fixed work as open** — wastes triage and erodes trust in their still-valid findings.

**Fix first (all P2, all quick, all correctness):**
1. **IO-CORE-01** — prune-done path deletes `.lock` files *unlocked*, silently reintroducing the documented flock inode-split bug.
2. **LEDGER-01** — `regress` persists a sparse 2-entry ledger that permanently suppresses `issueops status` phase derivation.
3. **IO-CORE-04** — stale-reset on `start` irreversibly discards Intent/DomainReview/DesignReview/Decisions on a single `os.Stat` miss.
4. **LK-01** *(independent cross-check)* — `StartIssueOps` takes its lock on the **raw-repo** id but writes under the **abs-repo** id, so concurrent relative-vs-absolute `start` calls re-open the P0 TOCTOU.

---

## Prioritized Findings

| # | Finding | Area (file:line) | Sev | Effort | Category | Fix |
|---|---------|------------------|-----|--------|----------|-----|
| 1 | prune-done re-deletes `.lock` files unlocked → flock inode split | `internal/core/issueops/issueops_stale_scan.go:174-177` | P2 | quick | correctness | Drop `os.Remove(lockPath)`; let orphan-lock sweep reclaim it once `.json` is gone, and unlink under `withIssueOpsLock` with phase==done recheck |
| 2 | `regress` persists sparse 2-phase ledger; `IssueOpsStatus` only derives when `len==0` → status loses all phases | `issueops_regress.go:62,73-85`; `issueops_phase_ledger.go:264-273`; also via `linking/link.go:37`, `compatibility_review.go:32` | P2 | quick | correctness | Seed from `DeriveIssueOpsPhaseLedger(record)` before `markIssueOpsLedgerStale`, OR make `IssueOpsStatus` merge-derive any phase missing from a partial ledger; clear stale note on legit re-advance; add regress→status test |
| 3 | stale-reset discards recoverable metadata on single `os.Stat` miss | `start/start.go:82-96`; `readinesspaths/paths.go:24-25` | P2 | quick | correctness | Carry Intent/DomainReview/DesignReview/Decisions/PlanPrep into reset struct (or snapshot prior record); optionally second-probe worktree absence |
| **LK** | **`StartIssueOps` locks on raw-repo id, writes under abs-repo id → concurrent relative-vs-abs `start` re-opens the P0 TOCTOU** | `package.go:197,211`; `start/start.go:29,36,43`; `issueops_state.go:40` | **P2** | **quick** | **correctness** | **Normalize `req.Repo` via `filepath.Abs` before `newIssueOpsID` in `StartIssueOps` (match `start.Start`), so the lock id == the record id** |
| 4 | session bind/unbind is unlocked RMW outside cycle lock; cycle lock keyed by id can't serialize per-repo binding file | `package.go:286-294,369-373`; `session/session.go:41-95` | P2 | medium | correctness | Add a per-repo lock around the binding file; make unbind a compare-and-delete under it (folding into `withIssueOpsLock` is insufficient — it's keyed by cycle id) |
| 5 | Engelbart Canvas Stop gate lacks `!stopHookActive` loop guard; judges whole transcript | `hookcli/hook_stop.go:81-84,211-233`; `installutil/hook_policy.go:25` | P2 | medium | correctness | Add `!stopHookActive` guard (mirror next-actions gate); scope evidence to most-recent create / writes since last user turn; accept `slack_update_canvas` as clearing evidence |
| 6 | self-hosted GitLab URL rejected by child create/close parsing, accepted everywhere else | `provider/gitlab/provider.go:605,628` (+ `remotecmd/remote.go:752`) | P2 | quick | consistency | Replace `strings.Contains(parts[i],"gitlab")` with `url.Hostname()` + `remoteparse.SplitGitLabIssuePath`; add self-hosted-domain tests |
| 7 | live remote verify runs CLI once, no retry (vs 3× LLM judge) | `remoteverify/issueops_remote_fetch.go:15,54`; `issueops_remote_child.go:37,73` | P2 | medium | robustness | Add bounded auth-aware retry/backoff (classify transient vs auth in `commandOutputError`); keep auth errors fail-fast |
| 8 | every MCP failure collapses to JSON-RPC `-32602`, bypasses normalization; diverges from CLI `{ok:false,error}` | `mcpcli/mcp_tool_issueops.go:63-68`; `mcp_tools.go:46-48` | P2 | medium | contract | Return tool-level failures as normalized results mirroring CLI; reserve `-32602` for schema violations, use `-32000`/error-kind for not-found vs internal; add error-case goldens |
| 9 | golden contract gate exercises none of 4 newest tools + ~14 others | `harnessapp/response_contract_{mcp,cli}_snapshot_test.go` | P2 | medium | test-gap | Extend both snapshot builders through domain-review/compat/ai-slop/feedback/regress/set_phase/status + cleanup/remote; regen goldens (min: cover the 4 ledger tools) |
| 10 | CI is `workflow_dispatch`-only, no gofmt/vet gate; unformatted source file persists | `.github/workflows/ci.yml:11,32-39`; `internal/adapter/hook/output.go` | P2 | quick | dx | `gofmt -w` now; add `gofmt -l ./...` + `go vet ./...` CI steps; re-enable on push/PR or file a tracking issue for the Linux test failures |
| 11 | brooks has no standalone routing hint — adversarial design/plan review is a dead trigger outside IssueOps | `hookprompt/rules.go` (no brooks entry); `skills/brooks/SKILL.md` | P2 | quick | routing | Add a `PrioritySecondary` brooks rule keyed on devil's-advocate / over-engineered / second-system / 과설계 / 계획 검토, Reason noting sub-agent dispatch |
| 12 | engelbart frontmatter/body breaks pioneer-family contract (no role/"Named after…"/insight/skeleton) | `skills/engelbart/SKILL.md:3,12` | P2 | quick | consistency | Add role sentence + "Named after Douglas Engelbart — …" + core insight; optional `<identity>`/Core Rule parity |
| 13 | both audit docs present already-fixed P0/P1s as open (inconsistent RESOLVED marking) | `.agent-harness/ISSUEOPS_AUDIT.md:10`; `.agent-harness/PROJECT_AUDIT.md:11` | P2 | quick | docs-drift | Mark ISSUEOPS 1.1 + PROJECT daemon 1.1/1.3 RESOLVED with fixing commits (5536cc8); add "last reconciled against `<sha>`" line; archive as dated snapshots |
| 14 | orphan `.lock` cleanup only runs when ≥1 cycle released (audit 1.3 PARTIAL); test bypasses the gate | `issueops_stale_scan.go:89-91,135-147` | P3 | quick | correctness | Run sweep + `git worktree prune` whenever `req.Apply`, independent of `len(Released)`; add ScanStale-driven zero-release test |
| 15 | non-Unix lock fallback is in-process only (audit 1.2 deferred, out of scope) | `issueops_lock_other.go:10-38` | P3 | quick | correctness | Keep deferred; document single-session-on-Windows constraint at call sites before any Windows claim |
| 16 | codd 217-line concurrency section unrouted/undescribed | `skills/codd/SKILL.md:3,672-887`; `rules.go:129-134` | P3 | quick | routing | Add deadlock/lock-contention/isolation/transaction (+ 데드락/락/트랜잭션) to codd keywords and "Use when" |
| 17 | created parent issues have no live verify path (only PR/MR + children verified) | `provider/github/provider.go:20-52`; `gitlab:20-44` | P3 | medium | contract | Add github:issue/gitlab:issue verify symmetric to verify-artifact; call after `--confirm` |
| 18 | "CLI-first then MCP fallback" has no code support; harness MCP wraps same CLI, errors unclassified | `mcpcli/mcp_tool_issueops.go:122,161,201` | P3 | medium | dx | Classify auth/permission/missing-token into typed error, OR document that fallback needs external github/gitlab MCP |
| 19 | issue/PR/MR `Number` population dead in prod; JSON/IID branch only hit by fake CLIs | `provider/gitlab/provider.go:434-456`; `github:263-275` | P3 | medium | test-gap | Request JSON on create + parse number for real, or drop `Number`/IID branch and feed tests the real bare-URL output |
| 20 | hook latency metric recorded post-completion → timeout-killed worst cases invisible | `hookcli/hook.go:22-30`; `hookmetrics/metrics.go:36-48` | P3 | large | correctness | Defer-write metric at hook entry with in-progress sentinel, or emit start-marker; at minimum document the blind spot |
| 21 | skillcontract pins only 4 skills; no all-skill frontmatter gate; atomic-commit-push unprotected | `skillcontract/skill_contract_test.go`; `ci.yml` | P3 | quick | test-gap | Add phrase pins for atomic-commit-push (git-add/force-push/secret), gitlab-usecase, self-verify/-augment, project-bootstrap; add a Go test/CI step running `validate-skill.py` over `skills/*` |
| 22 | issueops help text stale: 8 subcommands undocumented, phase list omits compatibility-review | `issueopscli/issueops_cli_support.go:12-44` | P3 | quick | docs-drift | Add domain-review/ai-slop-clean/regress/feedback-resolve/decision/record-routing/routing-score/remote-score + compatibility-review; generate usage from dispatcher map |
| 23 | cleanup-stale MCP can't prune done cycles; CLI `--prune-done` has no MCP equivalent | `mcp_tool_issueops_handlers.go:288-298`; `issueops_lifecycle_catalog.go:93` | P3 | quick | routing | Add `prune_done` to schema + parse into `PruneDoneAge` (default 720h to match CLI) |
| 24 | TECH_STACK frames deps as future candidates; MCP SDK undocumented/no ADR; skill count 10 vs 19 | `.agent-harness/TECH_STACK.md:49,68-77`; `ADR.md`; `go.mod:11` | P3 | quick | docs-drift | Replace §3 candidates with resolved deps; add ADR for `modelcontextprotocol/go-sdk`; fix count to 19 |
| 25 | CONVENTIONS references nonexistent `adapter/worker`, `adapter/fs` + flat files now split into packages | `.agent-harness/CONVENTIONS.md:26-30,38-39,58-59` | P3 | quick | docs-drift | Point worker→`internal/core/worker`, drop `adapter/fs`, reflect partitioned core subpackages + `*_facade.go` surface |
| 26 | stability-audit Stop criterion contradicts harness's own `continue` key | `skills/stability-audit/SKILL.md:54`; `adapter/hook/output.go:91-97,139-145` | P3 | quick | docs-drift | Allow `continue`/`decision`/`reason`/`systemMessage`; reject only truly invalid keys |
| 27 | self-augment SKILL.md has a malformed inline-code block merging two commands | `skills/self-augment/SKILL.md:31-32` | P3 | quick | docs-drift | Split into two separate commands (`--save-state` vs `--cycles N`) |
| 28 | brooks "IssueOps Benchmark Artifact Contract" heading has no consuming scorer signature | `skills/brooks/SKILL.md:36-48`; `benchmark/issueops_pioneer_checks.go` | P3 | quick | docs-drift | Retitle block (e.g. "Devil's-Advocate Verdict Evidence — feeds regress audit") or add a brooks scorer signature; document the 3-set membership policy |
| 29 | duplicate slice-flag types with inconsistent `String()` separator | `issueops_decision_cli.go:47-50`; `issueops_flags.go:5-14` | P3 | quick | consistency | Collapse to one repeatable `[]string` flag type; delete the duplicate |
| 30 | duplicate `mustJSON` panic helper across two cmd packages | `harnessapp/misc_facade.go:148-153`; `apidoc/api_doc_review_output.go:7-13` | P3 | quick | consistency | Extract one shared helper; low priority |

> **Verification corrections applied:** audit-survey findings IO-02 (orphan-lock cleanup), IO-06 (non-Unix lock), PS-01 (codd concurrency routing), and SC-01 (skillcontract pins) were *downgraded* P2→P3 by the adversarial pass (real but lower impact than first surveyed). Finding #13 was *downgraded* P1→P2. These corrected severities are reflected above.

---

## Recommendations by Bucket

### A. Quick wins (quick effort, real value) — do this first
- **Correctness, quick:** #1 (drop `os.Remove(lockPath)`), #2 (seed/merge-derive ledger before mark-stale), #3 (carry metadata across stale-reset), **LK-01** (abs-normalize the lock key in `StartIssueOps`), #14 (run orphan sweep on any `--apply`).
- **Routing/consistency, quick:** #6 (structural GitLab host detection), #11 (brooks routing hint), #12 (engelbart frontmatter), #16 (codd concurrency keywords).
- **DX, quick:** #10 (`gofmt -w` + add gofmt/vet CI steps).

### B. Structural / correctness (medium+ effort)
- **#4** — per-repo lock for the session binding file (the cycle lock is keyed by id and **cannot** serialize two cycles against the shared per-repo file; the "fold into `withIssueOpsLock`" shortcut is insufficient).
- **#5** — Engelbart Stop gate loop guard + evidence scoping + update-canvas acceptance.
- **#7** — auth-aware bounded retry for live remote verify (needs new transient-vs-auth classification in `commandOutputError`).
- **#8** — normalize MCP error contract to match CLI + distinct error codes + error-case goldens.
- **#9** — extend golden snapshot builders to the 4 newest + ~14 uncovered tools.

### C. Skills & DX polish (mostly P3)
- Routing/capability parity: #23 (MCP `prune_done`), #18 (classify CLI auth errors or document fallback), #28 (brooks benchmark heading).
- Verification depth: #17 (issue-create verify), #19 (real `Number` population or drop dead branch), #20 (hook timeout latency blind spot — large, defer), #21 (skillcontract phrase pins + all-skill frontmatter CI gate).
- Help/doc clarity: #22 (issueops usage text), #26, #27, #29, #30 cleanup.

### Doc/code drift to reconcile
- **#13 (P2):** reconcile `ISSUEOPS_AUDIT.md` 1.1 and `PROJECT_AUDIT.md` daemon 1.1/1.3 to RESOLVED (commit 5536cc8 et al.) — both currently present fixed P0/P1s as open and waste triage. Line refs in the docs are also stale; prefer archiving as dated snapshots.
- **#22, #24, #25, #26, #27, #28 (P3):** stale usage text, pre-implementation TECH_STACK (MCP SDK undocumented, 10-vs-19 skill count), CONVENTIONS pointing at nonexistent `adapter/worker`/`adapter/fs`, stability-audit contradicting the harness's own `continue` Stop key, malformed self-augment command block, brooks benchmark heading without a scorer. None affect runtime; all are quick edits to the doc-governance-heavy `.agent-harness/` tree.

---

## What's Already Solid

- **IssueOps core locking is real, not aspirational:** every RMW mutator routes through `withIssueOpsLock` (33 sites); `StartIssueOps` is lock-wrapped (`package.go:196-205`); the stale-scan TOCTOU is closed by re-read+re-classify under the per-id lock. Layering boundaries (no core→adapter, port→core, adapter→cmd) verified by grep with zero hits. *(Caveat: LK-01 — the lock key normalization gap — is the one crack in this otherwise-solid surface.)*
- **State persistence is correctly atomic & traversal-safe:** temp+chmod(0600)+rename with cleanup on every error branch; `normalizeIssueOpsID` rejects `..`/separators on every read/write/lock.
- **Phase gates are fail-closed:** every entry gate (`issueops_phase.go:61-98`) errors with the missing keys and never defaults open; needs-review findings are never auto-released; the `pr` phase is excluded from stale-reset so remote linkage survives.
- **Hooks fail open & are latency-bounded:** hook errors map to non-blocking exit 1; PreToolUse defaults to no-op; gofmt gate has a 2s timeout and self-gates to `.go` files; 5s host timeout; failure logs are flock-serialized, secret-redacted, atomically pruned. No hook performs VCS writes or prints tokens.
- **Surface parity & validation centralized:** exact 36-tool catalog↔dispatcher parity (zero drift); validation lives once in core recorders so CLI/MCP enforce identical rules; `resolve_feedback` fails closed (index −1).
- **Remote layer is injection-safe & verification-thorough:** GraphQL values JSON-encoded; create-child does hierarchy+labels+assignees readback on both providers and refuses to link unless `HierarchyVerified`; no token printing anywhere.
- **Pioneer skills are substantive, not themed fluff:** 11/12 share a coherent skeleton with runnable commands and falsifiable gates; benchmark signatures resist keyword-soup gaming (tests prove hollow evidence is rejected); sub-agent vs main-agent scoping is self-consistent.
- **Test hygiene:** all ~26 `t.Skip` are legitimately env-gated (POSIX-only fake CLIs, gofmt/git availability, symlink fixtures); no hidden broken tests, no `.only`. Non-test code is free of real TODO/FIXME placeholders.

---

## Independent cross-check (main-agent)

To validate the fleet, the main agent independently re-opened the highest-value claims:

- **Corroborated:** ISSUEOPS_AUDIT P0 "Start() lacks locking" is **FIXED** (lock hoisted to `package.go:199`). Phase-ledger grill/plan gates are **genuinely fail-closed** (`issueops_phase.go:61-98`, each returns `OK:false`+error on unmet readiness). All 19 skills carry consistent `name`+`description` frontmatter. Build+vet clean.
- **New finding the fleet missed — LK-01 (P2, quick):** `StartIssueOps` (`package.go:197`) computes its lock id via `newIssueOpsID(strings.TrimSpace(req.Repo), branch)` using the **raw** repo string, but the canonical record id is computed inside `start.Start` (`start/start.go:29-36`) from the **abs-normalized** repo (`filepath.Abs` → `store.NewID`). Since `store.NewID == newIssueOpsID` (`package.go:211`) does **no** abs-normalization, raw and abs paths hash to different ids — and `start.go:43`'s `legacyID := store.NewID(rawRepo, branch)` proves the code already knows they diverge. Two concurrent `start` calls for the same physical repo, one with a relative path (`.`) and one absolute, therefore acquire **different** lock files yet read-modify-write the **same** record, re-opening exactly the lost-update TOCTOU the P0 fix closed. **Fix:** abs-normalize `req.Repo` before `newIssueOpsID` in `StartIssueOps` so the lock id equals the record id (a one-line change mirroring `start.Start`). This is distinct from finding #4 (session-binding lock, keyed by repo-hash).

---

*Generated 2026-06-30 by a 6-dimension survey→adversarial-verify→synthesis workflow (25 agents) plus independent main-agent cross-check. Findings cite verified file:line at time of audit (HEAD ≈ ca50ab3). No code was modified.*
