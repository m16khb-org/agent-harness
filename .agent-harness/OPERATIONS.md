---
name: OPERATIONS.md
description: Install, sync, runtime, and operational procedures.
---

# Operations Map

`agent-harness` gives Codex and Claude Code the same Go binary, MCP schema, command policy, state store, shared skills, lifecycle hooks, and project-doc workflow.

Use this file as the quick map. Read the focused operation file that matches the task:

| Task | Read |
|------|------|
| Install, bootstrap, and refresh | `.agent-harness/operations/install.md` |
| Release checklist and clean-machine install reproducibility | `.agent-harness/operations/release-reproducibility.md` |
| Release dogfood transcripts and observed release UX gaps | `.agent-harness/operations/release-dogfood-notes.md` |
| Codex/Claude native skills, MCP registration, lifecycle hooks | `.agent-harness/operations/hosts.md` |
| Direct CLI, daemon-backed MCP, command policy, guard, worker commands | `.agent-harness/operations/cli-and-mcp.md` |
| Web-fetch deterministic benchmark and opt-in live parity | `.agent-harness/operations/web-fetch-live-parity.md` |
| self-verify, self-augment, API documentation gates, smoke checks | `.agent-harness/operations/verification.md` |
| project bootstrap, project-doc routing, MCP document update rules | `.agent-harness/operations/project-docs.md` |

## Core Surfaces

1. Native skills: `atomic-commit-push`, `issueops`, `self-augment`, `project-bootstrap`, `self-verify`, `stability-audit`, plus the named specialist skills in `skills/`.
2. MCP stdio proxy: `agent-harness mcp` starts or connects to the shared user-level `agent-harness daemon`.
3. CLI: `agent-harness inspect/preflight/status/verify-work/doctor/docs/project/policy/guard/state/issueops/workpool/loop/contract/daemon/worker/self-verify/self-augment/api-doc/hook`.
4. Loop contracts: `agent-harness loop start/record-attempt/status/stop` records verify-until-done state and strict readiness gates without executing verification commands.

## Daily Commands

```bash
agent-harness bootstrap --dry-run --json
agent-harness bootstrap
agent-harness project bootstrap --repo /path/to/repo --dry-run --json
agent-harness project bootstrap --repo /path/to/repo --sync --json
agent-harness doctor --repo . --json
agent-harness status --json
agent-harness docs --json
```

## Operational Health and One-Time Reconciliation

`agent-harness doctor` is the sole public cross-system health gate for canonical Git state, all IssueOps records/bindings, optional Orca inventory, and unexpected user-state artifacts. Invocation-only preservation never writes state:

```bash
agent-harness doctor --repo . --preserve-cycle EXACT_CYCLE_ID --preserve-terminal EXACT_HANDLE --json
```

- An active owner cycle is live only with complete fenced identity/resources and a heartbeat no older than 15 minutes. The threshold is diagnostic: heartbeat age alone never authorizes interrupt, delete, or IssueOps release. `issueops cleanup stale --apply` still requires its existing confirmed worktree/remote signal and locked fresh re-probe.
- Preserve flags are repeatable exact values for one doctor invocation. They do not create persistent exceptions or cure incomplete/duplicate identity.
- Orca remains optional. Absence is healthy only when no durable cycle claims Orca resources; otherwise inventory is unknown and doctor fails closed.
- The stability audit builds the binary, then delegates operational judgement to `doctor`. `--preserve-terminal EXACT_HANDLE` is a singular explicit assertion and takes precedence over the inherited environment; only when it is absent does a non-empty `ORCA_TERMINAL_HANDLE` remain the fallback. Sealed reconciliation passes its resolved `manifest.current_terminal.handle` explicitly and does not overwrite the environment variable.

The approved one-time full reconciliation uses an external mode-`0700` bundle at `~/.local/state/agent-harness-backups/<repo-fingerprint>/<UTC-timestamp>/`, not a product cleanup command. Git and SQLite backups are restore-tested; Orca snapshots are archival evidence only because the installed CLI exposes global reset but no conditional reset/import/restore. Stop before reset if the final full digest drifts. After a reset or crash seam, resume the sealed append-only journal and complete idempotently forward; do not infer a partial rollback.

## Tool Contract Conformance

```bash
agent-harness contract conformance baseline --json
HARNESS_TOOL_CONFORMANCE_LIVE=1 agent-harness contract conformance live \
  --hosts codex,claude,gjc \
  --model codex=default \
  --model claude=default \
  --model gjc=PROVIDER/MODEL \
  --gjc-auth-env PROVIDER_API_KEY \
  --profile clean \
  --target-completed 1 \
  --max-attempts-per-case 3 \
  --evidence-dir .agent-harness/evidence/tool-conformance \
  --json
agent-harness contract conformance replay --fixture PATH --json
```

`baseline`과 `replay`는 deterministic local gates다. `live`는 외부 model 비용 경계이며 opt-in env가 없으면 host process를 시작하지 않는다. Codex는 ephemeral/read-only/ignore-user-config 실행, Claude는 strict temp MCP config와 settings-source isolation, GJC는 temp project/plugin/`GJC_CODING_AGENT_DIR`와 명시한 auth env만 사용한다. 사용자 MCP 등록, GJC registry, credential DB는 수정하거나 복사하지 않는다.

Initial live report가 `defer_hardening`이면 현재 preregistered matrix에서 confirmed drift가 없다는 뜻이며 production contract를 변경하지 않는다. `needs_reproduction`이면 report가 지정한 한 host+fixture만 별도 10/20-completed batch로 재현한다. `authorize_hardening`은 같은 normalized signature가 두 번 이상 관측된 경우에만 가능하다. 상세 denominator와 fixture promotion 규칙은 `.agent-harness/TESTING.md`를 따른다.

## State Store Maintenance

The sqlite-backed state stores accumulate WAL frames and sidecar files that need periodic checkpointing. Two surfaces handle this:

**Automatic (session-start hook):** `MaybeMaintainStateStores` runs WAL truncate + permission repair at most once per 24h via a `.last-store-maintain` sentinel in the state root. No user action needed.

**Manual CLI:**
```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json

# Clean up orphan session bindings (cycle done or absent) and stale cycles
agent-harness issueops cleanup stale --repo /path/to/repo --apply --prune-done 720h
```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows. Stale binding cleanup is destructive and gated behind `--apply`; without it, stale bindings are reported but not deleted.

## Kubectl Live-Access Approval

With `--enforce-gitops-kubectl`, live access requires explicit confirmation. Claude uses its native `ask`. Codex cannot emit native PreToolUse `ask`, so the first eligible request blocks with a short instruction such as `승인 AH-XXXXXX`.

Codex can reuse approval only for exact-allowlisted read-only exec diagnostics that state both kube context and namespace. For example:

```bash
kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user
kubectl --context bc-stgdev -n stg exec -c linkerd-proxy deploy/rest-api-gateway -- curl -fsS http://localhost:4191/metrics
```

Enter the exact token in the same session. The approval must be activated by an allowlisted diagnostic within 10 minutes. The first allowed command and each later allowed command refresh a 30-minute idle TTL for the same session, canonical repo, context, and namespace; workload target and container may change. Changing context or namespace, allowing the TTL to expire, or losing state requires a new token. Runtime state uses mode `0600` and stores only request/scope fingerprints, never raw commands or cluster identifiers.

Codex `kubectl port-forward` remains exact-command one-shot: the next identical request consumes its 10-minute grant. Unsafe or unclassified Codex exec, including generic shells, interactive flags, arbitrary file/env reads, redirects, and non-allowlisted curl/dig options, blocks without an approval token. Do not remove `--enforce-gitops-kubectl` or use a generic shell as routine recovery. Direct mutating kubectl commands remain blocked and must go through GitOps.

## Release Smoke

```bash
scripts/release-repro-smoke.sh
```

Use `.agent-harness/operations/release-reproducibility.md` before deciding Homebrew, tarball, or other release packaging.

## Invariants

- Default install writes only user-level host configuration. Target repos get files only through explicit project bootstrap or project-local opt-in.
- Host adapters are thin wrappers around the same CLI/core behavior. They must not duplicate policy, schema, or state semantics.
- Hooks provide routing, lifecycle state, and bounded reminders only. They must not create issues/PRs, run tests, edit shared docs, or perform long network/file reads.
- IssueOps implementation must pass durable `compatibility_review` and `execution_decision` gates. Hooks do not decide backward compatibility, side effects, auto-proceed, human gates, or sub-agent usage; `issueops compatibility review` / MCP `issueops_record_compatibility_review` records the compatibility and side-effect judgement, then `issueops execution decide` / MCP `issueops_record_execution_decision` records the main-agent execution judgement before `implement`.
- Native install/update paths are standalone. External tools are neither installed nor required by `agent-harness`; use their own setup paths when a separate workflow needs them.
- Worker functionality remains policy-gated and state-first until write/network/background execution has explicit audit, timeout, cancellation, and redaction coverage.

## Optional Orca supervised execution

Orca is user-installed and optional. Preview with `agent-harness issueops worktree prepare --id ID --orchestrator auto --json`; add `--confirm` only after reviewing the resolved mode and path. `--orchestrator orca` requires readiness and `--orchestrator inline` requires an explicit user or documented recovery reason. An existing raw Git worktree is rejected; remove or re-create it through the current Orca-managed preparation flow instead of adopting it through a compatibility command.

For a resolved Orca path, follow `skills/issueops/references/orca-handoff.md`. The source session prepares the workspace and plan/gates, then previews and confirms `handoff start`. The fresh owner claims the exact cycle, acknowledges context, implements, verifies, publishes, creates the PR/MR, and calls `handoff complete`. The single flow then waits at the human cleanup boundary. Exact lifecycle ID routing wins before source-wide inference, including for a prep-only linked worker, so unrelated active cycles never capture its IssueOps status or gate mutation. A literal source-root `issueops start --repo <exact-source>` always remains available for a new cycle.

Every external create/dispatch boundary re-attests the complete exact-worktree terminal inventory and server-filtered dispatched tasks. Ambiguity persists `recovery_required`; inspect `issueops status --json` and reconcile explicitly rather than repeating an external mutation or switching modes. Runtime recovery requires exact persisted worktree, terminal, task, and dispatch identities plus a clean exact branch/HEAD. A dynamic title, stale handshake, missing/duplicate candidate, conflicting identity, connected worker, or uncommitted WIP is not replacement authority.

Completion enters `cleanup_pending_human_decision`. After a human merges the PR/MR, any fresh authenticated exact-source session may preview the inventory and, after explicit human approval, record the ordered cleanup receipts. The completed owner cannot approve cleanup. Stale scan, elapsed time, Stop hooks, session binding, and the original source session's absence never grant cleanup authority.

Operational release evidence includes fake-runner recovery matrices, installed Codex/Claude/GJC ownership-block smokes, and one disposable live Orca cycle with per-resource cleanup receipts. Native installation and `self-verify` do not require Orca availability.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.

## Orca ownership handoff

Use this exact order for a new ownership transfer:

```text
worktree prepare
-> plan and gates
-> plan-only commit
-> handoff start preview/confirm
-> owner claim
-> acknowledge-context
-> implement through PR/MR
-> handoff complete
-> human cleanup choice
```

Workspace provisioning before ownership transfer is deliberate. At every arrow, the source main worktree remains available before, during, and after handoff for unrelated work. “Disengage” means the source does not mutate or steer this exact cycle; it never makes the source checkout read-only. The fence is the canonical worker root, exact cycle ID, native owner, or persisted Orca resource, never a source CWD fallback or generic session binding.

The worker owner runs orientation and has post-handoff authority to publish and complete. `handoff complete` ends at `cleanup_pending_human_decision`; no automatic cleanup occurs. Use `handoff cleanup-preview` from the exact source root, present the three choices, then use `cleanup-approve --confirm` and ordered `cleanup-record` receipts only after the human directs it.
