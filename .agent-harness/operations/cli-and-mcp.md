---
name: cli-and-mcp.md
description: Direct CLI, daemon-backed MCP, policy, guard, worker, and command smoke operations.
---

# CLI And MCP Operations

## Direct CLI

```bash
agent-harness version
agent-harness inspect --json
agent-harness status --json
agent-harness preflight --json /path/to/git-repo
agent-harness docs --json
agent-harness doctor --repo . --json
agent-harness guard check --staged --json
agent-harness verify-work --json -- git status --short
agent-harness quality inspect --json
```

`quality inspect`는 수집 성공(`collection_status`), repository health
(`health_status`), automation gate(`gate_status`)를 분리한다. Collector 오류는
`ok=false`, `health_status=unknown`, `gate_status=block`이고 repository debt는
`health_status=needs_attention`, `gate_status=report_only`다. 각 finding은 stable
ID, severity, evidence, remediation, verification command를 가지며
`pioneer_coverage`는 canonical 12종 missing name을 그대로 노출한다.
`gate_status=block`은 JSON/text 결과를 먼저 출력한 뒤 process exit를 nonzero로
끝내므로 CI는 payload와 exit code를 함께 사용할 수 있다. `report_only`는
repository debt를 보이되 exit 0을 유지한다.

## Command Policy

```bash
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness policy run --read-only --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
```

`policy run --read-only` executes only argv-form allowlisted read-only commands with workspace/cwd policy, timeout, env allowlist, audit metadata, redaction, and bounded stdout/stderr. It does not open write, network, arbitrary shell, or background execution. Use `policy fake-run` for write-intent planning.

`guard check` is a portable quality gate. It blocks clear anti-patterns, warns on likely quality smells, and marks context-dependent cases for review. Current rules include secret-like paths, test sleeps, real external URLs in tests, ambiguous test names, snapshot/golden review needs, production-only changes, CLI/MCP/adapter contract changes without golden updates, and likely duplicate helpers.

## State

```bash
agent-harness state write --key checkpoint-1 --value "작업 메모" --json
agent-harness state read --key checkpoint-1 --json
agent-harness state list --json
agent-harness state prune --max-age 720h --json
agent-harness state prune --max-age 720h --confirm --json
agent-harness state doctor --json
agent-harness state migrate --json
agent-harness state migrate --confirm --json
```

State commands use user-state storage, not target repo source files.

## Loop Contracts

```bash
agent-harness loop start --repo PATH --name NAME --goal TEXT [--max-attempts N] [--json] -- [VERIFY_ARGV...]
agent-harness loop record-attempt --id ID --verdict pass|fail --evidence TEXT [--evidence TEXT...] [--json]
agent-harness loop status (--id ID | --repo PATH --name NAME) [--json]
agent-harness loop stop --id ID (--success | --reason TEXT) [--json]
```

`loop` records a durable verify-until-done contract. `start` stores `verify_argv` but never executes it; `record-attempt` requires evidence; `stop --success` requires the latest attempt to be `pass`. Same-repo active or exhausted loops block strict PR readiness with `loop_incomplete:<loop-id>`.

## Task Gate Ledgers

```bash
agent-harness gates init [--file PATH] --scope TEXT --gate "G1: outcome | CHECK: cmd | EXPECT: expect" [--gate SPEC...] [--json]
agent-harness gates check [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--timeout-seconds N] [--env NAME,NAME] [--write] [--network] [--json]
agent-harness gates status [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
agent-harness gates report [--file PATH]... [--workspace-root PATH] [--cwd PATH] [--json]
agent-harness gates abandon --gate ID --reason TEXT [--file PATH] [--json]
```

`gates` evaluates unlazy-compatible task gate ledgers (`GATES.md` plus
`gates/*.md`). The format is the unlazy v2 contract: one checkbox per outcome,
`CHECK:` command plus `EXPECT:` substring-or-`/regex/` match, and `EVIDENCE:`
recorded from the deciding output tail. A checkbox is a claim; evidence is the
proof — a checked gate whose evidence still reads `pending` counts as unmet
(`evidence_pending`, worse than `unchecked`). `ABANDON: <id> <reason>` is the
honest exit and resolves the gate for readiness while keeping it visible in
reports. Unlike upstream unlazy, CHECK commands never run through a raw shell:
they are tokenized to argv and executed through the command policy engine
(workspace boundary, env allowlist — default `HOME,PATH`, secret redaction,
timeout, audit log, shell interpreters denied). Exit codes follow unlazy: `0`
all met or abandoned, `1` unmet remain, `2` usage error.

MCP exposes the same operations as `gates_init`, `gates_check`, `gates_status`,
`gates_report`, and `gates_abandon` sharing one contract DTO (schema version 1).

IssueOps integration is opt-in through file presence: when a cycle's worktree
contains `GATES.md` or `gates/*.md`, unmet gates add `gates_incomplete:<file>`
to strict PR readiness and block entering the `pr` phase until the ledger is
complete (all gates met with evidence, or honestly abandoned). Repos without
ledger files are unaffected. Because the ledger lives in the worktree, real
cycles must commit it before PR readiness checks `worktree_clean`.

## Daemon And MCP

```bash
agent-harness daemon start --json
agent-harness daemon status --json
agent-harness daemon stop --json
agent-harness mcp
agent-harness mcp cleanup --json
agent-harness mcp cleanup --apply --json
```

daemon admission은 기본 256개 동시 MCP 연결을 허용한다. 장기 실행 multi-session
host에서 더 큰 bounded pool이 필요하면 daemon 시작 전에
`HARNESS_DAEMON_MAX_CONNECTIONS`를 `1..4096` 범위로 설정하고 daemon을
재시작한다. 범위를 벗어나거나 해석할 수 없는 값은 기본 256으로 fail-safe
복귀한다. `daemon status --json`의 `active_connections`,
`max_connections`, `accepting`으로 실제 admission 상태를 확인한다.

`mcp cleanup`은 기본 dry-run이다. Darwin의 `--apply`만 현재 checkout의 exact `agent-harness mcp` 명령, `PPID=1`, 확인된 executable/start time을 모두 만족하고 signal 직전 동일 identity가 다시 확인된 고아를 종료한다. Linux 컨테이너처럼 `PPID=1`이 살아 있는 host일 수 있는 플랫폼은 `skip-unsupported-platform`으로 거부한다. 살아 있는 host proxy, 다른 checkout, 외부 MCP, identity 미확정 프로세스는 건드리지 않는다.

MCP smoke:

```bash
tmp_state="$(mktemp -d)"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}' \
  | HARNESS_STATE_DIR="$tmp_state" HARNESS_DAEMON_DIR="$tmp_state/daemon" agent-harness mcp
HARNESS_DAEMON_DIR="$tmp_state/daemon" agent-harness daemon stop --json
rm -rf "$tmp_state"
```

## Worker

```bash
agent-harness worker enqueue --kind smoke --payload "TOKEN=redacted" --json
agent-harness worker status --id "$JOB_ID" --json
agent-harness worker list --json
agent-harness worker cancel --id "$JOB_ID" --json
agent-harness worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
```

`worker` currently records lifecycle jobs and can run policy-gated read-only evidence commands. It is not a general writable shell runner. Future process execution must pass command policy, audit logging, timeout/cancellation, and redaction checks.

## Contract And Audit

```bash
agent-harness contract schema --json
agent-harness contract check --json
HARNESS_AUDIT_LOG="$(mktemp)" agent-harness policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
```

`policy audit` appends redacted JSONL policy decisions and does not execute the command.
