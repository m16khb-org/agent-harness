---
name: CAUTIONS.md
description: Recurring mistakes, operational cautions, and avoidance guidance.
---

# 주의사항 모음

`agent-harness`에서 반복적으로 실수하기 쉬운 설계·운영 주의사항을 모은다.
이 파일은 canonical index다. 상세 절차·근거·사건 기록은 아래 module과 dated
lesson으로 분리됐고, 여기서는 핵심 한 줄과 탐색 링크만 둔다.

## Universal summary

- core behavior는 Go core에 두고 host adapter는 CLI/MCP wrapper로 제한한다.
- shell runner는 argv 우선, workspace root 밖 접근 기본 거부, secret redaction 적용.
- 프로젝트 지식은 `.agent-harness/`, runtime state는 user state dir / ignored `.harness/`.
- daemon/worker/socket은 local FS의 user state dir에만 둔다.
- IssueOps 상태 전이·lease·publication은 durable `issueops` 명령이 소유하고 hook은
  fast deterministic 위반만 차단한다.
- IssueOps worktree 밖 mutation은 hook guard + 절대경로 + status 재확인으로 막는다.
- self-verify는 외부 검증 메커니즘을 명시해야 하고 문서만 통과하는 가짜 안정성을 경계한다.
- Omo MCP catalog는 server config hash로 장기 cache되므로 installer가
  advertised catalog SHA를 config env에 포함해 binary-only schema update도
  fresh session에서 재조회되게 한다
  ([install.md](operations/install.md)).
- `.agent-harness/*.md` 편집은 response-contract golden을 드리프트시킨다.
- Dated 기록의 IssueOps 명령·필드·상태는 사고 당시 증거일 뿐 실행 지시가 아니다.
  현재 실행 계약은 `skills/issueops/references/execution.md`와
  `.agent-harness/OPERATIONS.md`를 따른다.

## Risk-category modules

| Module | Covers |
|---|---|
| [runtime.md](cautions/runtime.md) | daemon, worker, lock, SQLite state, /tmp·install hygiene |
| [security.md](cautions/security.md) | shell/command policy, secrets, git identity, publication git-config authority |
| [integrations.md](cautions/integrations.md) | host adapters, native hooks, MCP, shared skills, external tools, Slack, Stop-hook output |
| [issueops-lifecycle.md](cautions/issueops-lifecycle.md) | branches, worktree guards, numbered choices, domain vocab, readiness gates, golden drift |
| [issueops-orchestration.md](cautions/issueops-orchestration.md) | Orca create/dispatch/terminal/mailbox/rollover, sealed reconciliation, publication |
| [issueops-execution.md](cautions/issueops-execution.md) | v1 fence liveness, lease authority, operational-health diagnosis, exact-reader immutability |
| [audit-and-process.md](cautions/audit-and-process.md) | self-verify/augment drift, stability-audit contracts, JSON/QA process, cross-process helpers |

## Dated incident lessons

One file per incident under `cautions/lessons/`. Each carries the full
Kind/Source/Summary/Context/Resolution/Evidence record and a historical-evidence
footer; older notes also live in `archive/cautions-incidents.md`.

| Date | Lesson |
|---|---|
| 2026-07-02 | [Re-verify stale memory observations against HEAD](cautions/lessons/2026-07-02-reverify-stale-memory-observations-against-head.md) |
| 2026-07-07 | [IssueOps orchestration locks, additive fields, worker leases](cautions/lessons/2026-07-07-issueops-orchestration-locks-additive-fields-worker-leases.md) |
| 2026-07-07 | [SQLite sqlstore span discipline: active-root chain, fresh start](cautions/lessons/2026-07-07-sqlite-sqlstore-span-discipline.md) |
| 2026-07-08 | [Codex "invalid JSON output" was co-resident hook pipe truncation](cautions/lessons/2026-07-08-codex-invalid-json-output-pipe-truncation.md) |
| 2026-07-09 | [macOS pipe KVA exhaustion blocks stdout-capture CLI tests](cautions/lessons/2026-07-09-macos-pipe-kva-exhaustion.md) |
| 2026-07-10 | [Local MCP gateway FD exhaustion resets all loopback MCP connections](cautions/lessons/2026-07-10-local-mcp-gateway-fd-exhaustion.md) |
| 2026-07-28 | [update does not own host MCP; pending requests are not replayed](cautions/lessons/2026-07-28-update-mcp-lifetime-host-owned.md) |
| 2026-07-31 | [Released direct lease recovery needs a finite next_command chain](cautions/lessons/2026-07-31-released-direct-lease-recovery.md) |
| 2026-07-31 | [Orca task mutation seals explicit Run + coordinator consumer](cautions/lessons/2026-07-31-orca-task-mutation-explicit-run-coordinator-consumer.md) |
| 2026-08-03 | [Orca resume must not use current prompt template as trust root](cautions/lessons/2026-08-03-orca-resume-prompt-template-trust-root.md) |
| 2026-08-04 | [Resolve parent drift before completing reseed](cautions/lessons/2026-08-04-completed-reseed-parent-drift.md) |
| 2026-08-04 | [Do not trust generated IssueOps command PATH token alone](cautions/lessons/2026-08-04-generated-issueops-command-path-token.md) |
| 2026-08-04 | [Codex command-only payload omits workdir; cwd is turn cwd](cautions/lessons/2026-08-04-codex-command-only-hook-payload-workdir.md) |
| 2026-08-04 | [Released sync-base conflict needs scoped resolution writer](cautions/lessons/2026-08-04-released-sync-base-conflict-write-lease.md) |
| 2026-08-08 | [Command-only payload exempts cwd fence only for self-describing commands](cautions/lessons/2026-08-08-command-only-payload-cwd-fence-exemption.md) |
| 2026-08-11 | [self-verify `--full`/`--iterations` modes removed](cautions/lessons/2026-08-11-self-verify-iterations-full-modes-removed.md) |

## Removed CLI modes (historical only)

`self-verify --iterations=N requires --full` and `self-verify --full --iterations=10`
(10 seeded deterministic iterations; ~180s / ~3712s / 5400s budgets) were removed
**2026-08-11**. They are not current operational commands. Full historical record:
[2026-08-11 lesson](cautions/lessons/2026-08-11-self-verify-iterations-full-modes-removed.md).
Current `self-verify` behavior: testing family's `testing/self-verification.md`.

## Update workflow

1. Pick the canonical owner above; add a new module section only when a new
   responsibility class appears.
2. For a new incident lesson, create `cautions/lessons/YYYY-MM-DD-<slug>.md`
   with the full record and a back-link to this index.
3. Update a module in place for evergreen guidance; never summarize away a
   command, constraint, failure mode, or date.
4. Keep this index and every module within the manifest line budget (250).
5. After editing any `.agent-harness/*.md`, regenerate
   `cmd/harness/testdata/response_contracts.golden.json`
   (`go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update`);
   see [issueops-lifecycle.md §27](cautions/issueops-lifecycle.md).
6. Run the docs checker: `uv run --directory skills/project-docs-optimize
   python -m scripts.check --root "$PWD" --mode check --json`.
