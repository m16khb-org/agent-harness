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

- An active IssueOps execution is live only with a complete generation, native process receipt, canonical worktree, and mode-specific resource identity. Process absence alone never authorizes interrupt, deletion, or lease replacement; use the previewed generation-CAS replacement sequence and prove quiescence.
- Preserve flags are repeatable exact values for one doctor invocation. They do not create persistent exceptions or cure incomplete/duplicate identity.
- Orca remains optional. Absence is healthy only when no durable cycle claims Orca resources; otherwise inventory is unknown and doctor fails closed.
- The stability audit builds the binary, then delegates operational judgement to `doctor`. `--preserve-terminal EXACT_HANDLE` is a singular explicit assertion and takes precedence over the inherited environment; only when it is absent does a non-empty `ORCA_TERMINAL_HANDLE` remain the fallback. Sealed reconciliation passes its resolved `manifest.current_terminal.handle` explicitly and does not overwrite the environment variable.

The approved one-time full reconciliation uses an external mode-`0700` bundle at `~/.local/state/agent-harness-backups/<repo-fingerprint>/<UTC-timestamp>/`, not a product cleanup command. Git and SQLite backups are restore-tested; Orca snapshots are archival evidence only because the installed CLI exposes global reset but no conditional reset/import/restore. Stop before reset if the final full digest drifts. After a reset or crash seam, resume the sealed append-only journal and complete idempotently forward; do not infer a partial rollback.

## Tool Contract Conformance

```bash
agent-harness contract conformance baseline --json
HARNESS_TOOL_CONFORMANCE_LIVE=1 agent-harness contract conformance live \
  --hosts codex,claude \
  --model codex=default \
  --model claude=default \
  --profile clean \
  --target-completed 1 \
  --max-attempts-per-case 3 \
  --evidence-dir .agent-harness/evidence/tool-conformance \
  --json
agent-harness contract conformance replay --fixture PATH --json
```

`baseline`과 `replay`는 deterministic local gates다. `live`는 외부 model 비용 경계이며 opt-in env가 없으면 host process를 시작하지 않는다. Codex는 ephemeral/read-only/ignore-user-config 실행, Claude는 strict temp MCP config와 settings-source isolation을 사용한다. 사용자 MCP 등록이나 credential DB는 수정하거나 복사하지 않는다.

Initial live report가 `defer_hardening`이면 현재 preregistered matrix에서 confirmed drift가 없다는 뜻이며 production contract를 변경하지 않는다. `needs_reproduction`이면 report가 지정한 한 host+fixture만 별도 10/20-completed batch로 재현한다. `authorize_hardening`은 같은 normalized signature가 두 번 이상 관측된 경우에만 가능하다. 상세 denominator와 fixture promotion 규칙은 `.agent-harness/TESTING.md`를 따른다.

## State Store Maintenance

The sqlite-backed state stores accumulate WAL frames and sidecar files that need periodic checkpointing. Two surfaces handle this:

**Automatic (session-start hook):** `MaybeMaintainStateStores` runs WAL truncate + permission repair at most once per 24h via a `.last-store-maintain` sentinel in the state root. No user action needed.

**Manual CLI:**
```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json

```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows.
IssueOps v1 lease recovery is not part of store maintenance and never happens
from a time threshold.

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
- IssueOps implementation must pass durable design, compatibility, devil's-advocate, and execution v1 gates. Hooks do not decide compatibility, side effects, sub-agent usage, or lease ownership. `issueops execution prepare/status/claim/release/replace/reconcile/complete` and MCP `issueops_execution` are the single execution contract.
- Native install/update paths are standalone. External tools are neither installed nor required by `agent-harness`; use their own setup paths when a separate workflow needs them.
- Worker functionality remains policy-gated and state-first until write/network/background execution has explicit audit, timeout, cancellation, and redaction coverage.

## Optional Orca execution v1

Orca is user-installed and optional. Preview
`agent-harness issueops execution prepare --id ID --mode auto ... --json`,
review the mode, branch, base SHA, canonical worktree, and owner model, then
repeat the identical request with `--confirm`. `auto` selects Orca only when
readiness succeeds before mutation; otherwise it selects direct. The only
first-party owner hosts are Codex and Claude.

For Orca mode, follow `skills/issueops/references/execution.md`. Preparation
seals the remote issue body, context packet, fully rendered owner prompt, and
private claim-token file before launch. The fresh owner verifies both SHA-256
digests and runs the exact `issueops execution claim` command. Only the active
generation holder implements, verifies, creates the draft PR/MR, and completes
from the canonical worktree. The source main worktree remains available for
unrelated cycles.

Every external mutation stores intent first. Timeout or ambiguous output is not
absence: inspect `issueops execution status` and use preview/confirm
`issueops execution reconcile`; never repeat create or switch to direct after a
possible Orca mutation. Failed-holder recovery uses the ordered generation-CAS
replacement sequence and proves the prior process/resource is quiescent before
creating a new claimable generation.

`issueops execution complete` requires phase `pr`, the exact active generation,
final HEAD, committed Turing report, verification evidence, and the verified
durable remote artifact URL. It records `done` and releases the lease
atomically. It never merges or deletes a worktree, branch, terminal, or remote
resource. Operational release evidence includes fake-provider recovery
matrices, installed Codex/Claude hook smokes, and disposable live Orca
ready/absent scenarios. Native installation and `self-verify` do not require
Orca availability.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.

## Orca owner sequence

Use this order:

```text
provider branch + base SHA
-> execution prepare preview/confirm
-> sealed packet/prompt + native owner launch
-> execution claim with token file and both digests
-> plan/TDD/verification in the canonical worktree
-> generation-fenced draft PR/MR + readback
-> execution complete
-> separate human merge and cleanup choice
```

Preparation seals the caller-selected owner host, model, and effort before
terminal creation. Orca must expose `terminal create --command`; a launch shape
that cannot carry the exact model contract is rejected. The write fence is the
exact lifecycle ID, generation, native process receipt, canonical worktree, and
persisted Orca identity. The source main worktree remains available for
unrelated work throughout the sequence.
