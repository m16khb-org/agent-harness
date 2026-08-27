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
- stacked PR을 부모 브랜치 머지 뒤 기본 브랜치로 재타깃하면 `cleanup finish`가
  `base_branch_drifted`로, `cleanup abandon`이 `remote_artifact_unmerged`로 거부해
  레코드가 dead-end에 빠졌다(#490, `io-71af6dd82f0d` 실측). 이제 준비 base가 원격에서
  사라졌고 관측 base가 기본 브랜치일 때만 정상 재타깃으로 통과한다. 관측 실패는
  `merged_base_remote_unobserved`로 fail-closed이며 손으로 base를 주장하는 플래그는 없다.
- IssueOps 상태 전이·lease·publication은 durable `issueops` 명령이 소유한다. hook은
  `SessionStart` project-doc catalog 주입뿐이며 아무것도 차단하지 않는다(2026-08-27).
- IssueOps worktree 밖 mutation은 절대경로 + `issueops execution status` 재확인으로 막는다.
- 게이트 원장 `CHECK:`는 argv 한 줄이다. 따옴표 밖 `&& || ; |`는 셸이 아니라 첫 명령의
  인자가 되어 거짓 met을 만들었고(#484), 이제 `gates init`이 거부하고 `gates check`는
  unchecked로 둔다. 복합 검사는 스크립트나 `python3 -c` 하나로 감싼다. 리터럴 `EXPECT:`는
  출력 줄 전체 또는 줄 앞 토큰과만 일치하며, EXPECT가 있어도 CHECK는 exit 0이어야 met이다
  (#486; 비영 종료가 정상인 도구는 `python3 -c`로 감싸 0으로 끝낸다).
- IssueOps gate ledger는 root `GATES.md`가 아니라 이슈 폴더
  `.agent-harness/issues/<provider-issue-number>/gates.md`로 namespacing한다(#480;
  옛 `.agent-harness/gates/*.md`는 읽기 호환)
  ([2026-08-26 lesson](cautions/lessons/2026-08-26-gates-root-ledger-worktree-conflicts.md)).
- self-verify는 외부 검증 메커니즘을 명시해야 하고 문서만 통과하는 가짜 안정성을 경계한다.
- Omo MCP catalog는 server config hash로 장기 cache되므로 installer가
  advertised catalog SHA를 config env에 포함해 binary-only schema update도
  fresh session에서 재조회되게 한다
  ([install.md](operations/install.md)).
- `.agent-harness/*.md` 편집은 response-contract golden을 드리프트시킨다.
- 로컬 검증 배터리의 게이트 집합은 CI와 같아야 한다. CI가 첫 게이트(gofmt)에서 끊기면 뒤의
  test/golden 실패는 관측되지 않고, 환경 관측값(working tree, 로컬 심링크)을 그대로 박은
  golden/검증기는 clean checkout에서 깨진다
  ([2026-08-26 lesson](cautions/lessons/2026-08-26-ci-gofmt-gate-local-battery-drift.md)).
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
| 2026-08-21 | [api-doc dogfood: multiline-decorator routes bypassed static checks; review input lacked error evidence](cautions/lessons/2026-08-21-api-doc-route-block-assembly-and-evidence-bundling.md) |
| 2026-08-21 | [issueops lifecycle dogfood: whoami was claim-flags-only; branch errors cited foreign issues](cautions/lessons/2026-08-21-issueops-whoami-record-flags-and-branch-examples.md) |
| 2026-08-22 | [underused-surface dogfood: shipped benchmark panic, mcpsmoke data race, hook help noise](cautions/lessons/2026-08-22-underused-surface-dogfood-defects.md) |
| 2026-08-22 | [Kordoc install unblocks boehm pioneer; child tasks need CLI+handshake probe](cautions/lessons/2026-08-22-kordoc-install-unblocks-boehm-pioneer.md) |
| 2026-08-26 | [CI gofmt gate drifted from the local battery; golden captured a dirty working tree](cautions/lessons/2026-08-26-ci-gofmt-gate-local-battery-drift.md) |
| 2026-08-26 | [Merged-without-execution cycle had no typed cleanup exit; abandon accepts record-linked residue](cautions/lessons/2026-08-26-abandon-record-linked-residue-without-execution.md) |
| 2026-08-26 | [Root GATES.md caused add/add conflicts across IssueOps worktrees](cautions/lessons/2026-08-26-gates-root-ledger-worktree-conflicts.md) |
| 2026-08-26 | [GitLab work_items issue URL alias rejected by the provider parser and the create-issue live gate](cautions/lessons/2026-08-26-gitlab-work-items-url-provider-create-issue.md) |
| 2026-08-27 | [Skill added without agents/openai.yaml broke the self-verify QA gate on main](cautions/lessons/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md) |
| 2026-08-27 | [cleanup finish blocked on one Orca terminal shell; cleanup now stops worktree processes and terminals itself](cautions/lessons/2026-08-27-cleanup-stops-worktree-processes.md) |
| 2026-08-27 | [Daemon accept-loop burst dial exceeded the unix backlog](cautions/lessons/2026-08-27-daemon-accept-loop-burst-dial-backlog.md) |
| 2026-08-27 | [Record delete bypassed the sqlstore span gate and orphaned related state](cautions/lessons/2026-08-27-record-delete-bypassed-the-span-gate.md) |

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
