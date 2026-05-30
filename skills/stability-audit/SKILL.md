---
name: stability-audit
description: Run an exhaustive agent-harness stability audit from install/update through hooks, MCP, daemon, worker, state, process hygiene, memory-growth signals, stale sockets, zombie/orphan processes, and regression verification. Use when the user asks for E2E stability checks, full operational sweep, hook/MCP reliability, install verification, memory leak investigation, zombie process investigation, or “전수조사/안정성 점검”.
---

# Stability Audit

## Goal

Prove agent-harness is operationally stable across install/update, native host integration, hooks, MCP, daemon, state, worker, and process hygiene. Fix any confirmed failure before reporting success.

## Safety model

- Default to evidence-first audit; do not kill processes or alter user/global config unless the user asked to resolve issues or the stale target is clearly owned by this harness checkout.
- Use temp `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`, and `HARNESS_WORKER_DIR` for stress tests.
- Never kill active `codex`, `claude`, `tmux`, or unrelated MCP processes. Only clean up confirmed stale `agent-harness`/legacy `bin/harness` daemons or temp watchers after recording evidence.
- Treat host-level install commands as side-effecting. Use dry-run first; run real install only when the user asked for install E2E or the current task is explicitly about installed hooks/MCP.

## Fast path

From the harness repo root:

```bash
python3 skills/stability-audit/scripts/e2e_stability_audit.py --json
```

For a full installation and cleanup pass:

```bash
python3 skills/stability-audit/scripts/e2e_stability_audit.py --full-install --cleanup-stale --json
```

If the script reports `ok: false`, inspect `failures`, patch the root cause, and rerun the same command plus the relevant targeted tests.

## Manual workflow

1. **Preflight scope**
   - Read `.agent-harness/OPERATIONS.md`, `.agent-harness/CONVENTIONS.md`, and `.agent-harness/TESTING.md` when install, hooks, MCP, daemon, worker, or native integration behavior is in scope.
   - Capture `git status --short --branch`, process baseline, daemon status, and host MCP registrations.

2. **Install/update E2E**
   - Build: `go build -o bin/agent-harness ./cmd/harness`.
   - Dry-run both high-level paths with JSON when available:
     - `./bin/agent-harness bootstrap --skip-upstream-tools --dry-run --json`
     - `./bin/agent-harness update --skip-upstream-tools --dry-run --json`
   - Verify native install surfaces:
     - `./bin/agent-harness install-native --dry-run --json`
     - `./bin/agent-harness install-native --json` only for full install tasks.
     - `codex mcp get agent_harness`
     - `claude mcp list` and check for duplicate/conflicting `agent_harness` scopes.

3. **Hook contract sweep**
   - Invoke every configured `~/.codex/hooks.json` event with representative JSON.
   - Fail on non-zero exit, invalid JSON, unsupported `suppressOutput`, Stop hook keys outside `decision`/`reason`, or noisy multi-line `UserPromptSubmit` context.

4. **MCP and daemon sweep**
   - Run standalone JSON-RPC through `./bin/agent-harness mcp` with temp state/daemon dirs.
   - Call at least `initialize`, `tools/list`, `resources/list`, `harness_inspect`, `docs_index`, `state_doctor`, `project_docs_route`, and `daemon_status`.
   - Stop the temp daemon and verify its pid is gone.

5. **State/worker/policy sweep**
   - With temp dirs, run state migrate/write/read/doctor, policy check on `git status --short`, worker enqueue/list/run.
   - Verify worker remains no-shell/read-only unless explicitly testing policy denial.

6. **Leak/zombie/orphan sweep**
   - Repeat temp daemon start/status/stop cycles and assert no new daemon pids remain.
   - Scan `ps` for zombie state `Z` matching `agent-harness`, `bin/harness`, or project codegraph watchers.
   - Classify legacy `bin/harness daemon --internal`, temp socket daemons, duplicate MCP servers, and temp codegraph watchers as stale only when their socket/path proves they are not current.
   - Sample daemon RSS after warmup; treat repeated monotonic growth across multiple rounds as suspicious, not a single Go runtime warmup jump.

7. **Regression gate**
   - Run at minimum:
     - `go test ./... -count=1`
     - `go test -race ./... -count=1` for code/runtime changes
     - `go build -o bin/agent-harness ./cmd/harness`
     - `./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json`
   - Regenerate golden files only when a public contract intentionally changed.

## Completion report

Report:

- changed files and fixes applied
- install/update evidence
- hook and MCP evidence
- daemon/worker/state evidence
- process hygiene evidence: remaining harness daemons, stale processes removed, zombies found/none
- RSS/leak conclusion and sampling caveats
- tests run and pass/fail status
- remaining risks or host approvals needed
