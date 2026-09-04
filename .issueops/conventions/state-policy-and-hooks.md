# State, policy, guard, hook, and lifecycle conventions

> Family index: [`../CONVENTIONS.md`](../CONVENTIONS.md). This module owns
> worker lifecycle, config/env precedence, logging and secret hygiene, hook
> output shape, the language-agnostic guard, policy tiers, doctor and lifecycle
> state, the IssueOps state-machine reducer contract, and the external-
> orchestration adapter boundary. Go/package/port/adapter rules live in
> [`go-and-packages.md`](go-and-packages.md); CLI/MCP and response contracts
> live in [`cli-mcp-and-output.md`](cli-mcp-and-output.md).

## 5. Worker 컨벤션

- worker는 로컬 전용으로 시작한다. 원격 API는 별도 요구가 생기기 전까지 만들지 않는다.
- Unix socket 또는 localhost binding을 사용하고, 권한을 제한한다.
- job은 idempotency key, timeout, cancellation을 갖는다.
- worker 시작/종료는 stale lock과 orphan process를 처리한다.
- 장기 작업 상태와 project lifecycle queue/profile은 user state dir에 저장하고, repo에 secret/state 원문을 쓰지 않는다. lifecycle state는 `projects/<repo-id>/` namespace로 격리해 같은 머신의 여러 repo가 섞이지 않게 한다.

---

## 6. Config / env

- 현재 runtime 입력은 command flag, `ISSUEOPS_*` env, installer가 관리하는 host JSON/TOML config다.
- `~/.config/issueops/config.yaml`과 `.issueops-runtime/config.yaml`은 reserved 경로다. 현재 loader는 구현되지 않았으므로 이 경로를 읽거나 만들지 않는다.
- 향후 generic config loader의 우선순위는 `flag → env → workspace config → user config → default`로 고정한다.
- secret 원문은 저장하지 않는다.

---

## 7. Logging / secret hygiene

- 구조화 로그를 사용한다.
- secret으로 보이는 값은 adapter 경계에서 redaction한다.
- command stdout/stderr를 그대로 저장하기 전에 redaction filter를 거친다.
- 로그에는 workspace, command id, duration, exit code를 남기되 token/key 원문은 남기지 않는다.

---

## Hook 컨벤션

- 기본 설치는 `SessionStart` 하나를 context-only로 등록한다. 이 hook은 project-doc catalog만 읽어 host-compatible context를 만들며 `startup`·`resume`·`clear`·`compact` 모든 source에서 같은 catalog를 주입한다(Claude Code 2.1.247과 codex-cli 0.150.1 모두 압축 뒤 `SessionStart`를 `source:"compact"`로 다시 실행하고, `PostCompact`에는 모델용 컨텍스트를 실을 수 없다 — [ADR 2026-08-27](../adr/decisions/2026-08-27-session-start-owns-compaction-context.md)). IssueOps list/read, lifecycle reminder, runtime diagnostic, telemetry, SQLite maintenance, worker recovery, state write를 하지 않는다.
- `issueops hook`의 subcommand는 `session-start`와 `post-compact` 둘뿐이다. `post-compact`는 Omo 확장(`session_compact` → `--json`)과 진단용이며 host shape는 `systemMessage`만 싣는다. 주입할 문서가 없으면 두 hook 모두 `{}`를 낸다. enforcement·relay·telemetry hook은 존재하지 않으며 다시 추가하지 않는다.
- Hook 출력은 event별 host schema를 따른다. Codex는 `--host codex`로 readable catalog를 `additionalContext`에 싣고 `systemMessage`를 생략하며, Claude Code는 readable `systemMessage`와 compact `additionalContext`를 분리한다. `--json`은 snake_case DTO(`should_inject`, `project_docs`, `compact`, `user_view`)를 그대로 낸다.
- `.issueops/*.md` frontmatter descriptions are canonical bootstrap/sync metadata. Keep them concise English category descriptions, not prose summaries, because every `project bootstrap`/`project bootstrap --sync` target and every project-doc catalog inherits them.
- Codex/Claude별 hook 설정은 adapter/template에서만 다루고, catalog 구성은 공통 `issueops hook ...` CLI/core에 둔다. `ISSUEOPS_DISABLE_HOOKS`는 두 hook을 무출력 no-op으로 만든다.

## Guard 컨벤션

- `issueops guard check`는 언어 무관 1차 방어선이다. path, diff, regex, token similarity처럼 deterministic하고 빠른 신호만 core rule로 둔다.
- 확실한 금지만 `block`한다. 예: secret-like path, test sleep, real external service in tests. 기존 코드 재사용 여부, snapshot 품질, production-only 변경처럼 의미 판단이 필요한 항목은 `warn` 또는 `review`로 보고한다.
- 새 symbol/helper가 기존 symbol과 유사하면 `reuse-before-new` review finding을 낸다. 이 finding은 자동 실패가 아니라 기존 코드 탐색 증거 또는 새 구현 근거를 요구하는 신호다.
- 언어별 AST/linter 통합은 optional adapter로 붙이고, core guard가 특정 언어 toolchain에 의존하지 않게 한다.
- 차단은 **왜 막혔는지**를 함께 낸다. 게이트가 판정 과정에서 이미 관측한 것을 버리지 않는다 — 슬러그나 코드만 받은 owner는 명령을 조금씩 바꿔가며 추측 재시도를 반복한다(이슈 #90 발견 4, #154). 구조화된 deny가 사람이 읽을 사유를 대체하는 출력 경로라면 그 사유를 deny 안에 실어야 한다. 담는 것은 분류 결과와 이미 추출된 경로이며 명령 원문은 담지 않는다 — 인자에 토큰이 있을 수 있다.
- 해소 경로가 **하나로 정해지는** 차단만 그 명령을 안내한다. 상황에 따라 갈리는 항목에는 붙이지 않는다. 틀린 안내는 안내가 없는 것보다 나쁘다.
- **관측 불가와 조건 위반을 다른 슬러그로 구분한다.** 둘 다 fail-closed지만 다음 행동이 다르다 — 전자는 관측 도구를 고치고 후자는 상태를 고친다(#154의 `workspace_processes_observable` vs `workspace_processes_quiescent`).
- **`missing` 안의 슬러그는 요구형 극성으로 통일한다**(#185). `missing`은 *충족되지 않은 요구*의 목록이므로 상태 차단도 요구형으로 뒤집어 적는다 — 원격 브랜치가 남아 막혔으면 `remote_branch_absent`이고, 워크트리가 더러워 막혔으면 `worktree_clean`이다. 차단 사실을 그대로 적으면(`remote_branch_present`, `worktree_dirty`) "그 상태라는 요구가 미충족" = **반대 상태**로 읽힌다. `#181` 정리에서 `cleanup status`와 `cleanup finish`가 같은 상태를 반대 극성으로 보고해 운영자가 실제 상태를 따로 확인해야 했다. 이 축은 위의 관측/조건 축과 직교한다 — 관측 실패 슬러그(`remote_branch_check_failed`)는 그대로 둔다. 같은 이름이 다른 표면에서 진짜 요구일 수 있으므로(`execution sync-base`의 `remote_branch_present`는 브랜치가 **있어야** 한다는 요구다) 표면별로 그 조건이 요구인지 차단인지 먼저 정한다.
- preview는 **자기 근거의 강도를 밝힌다.** 외부 자원을 조회하지 않고 낸 결과가 관측 증거로 읽히면 오진단이 생긴다(#99의 잘못된 의혹이 그렇게 나왔다, #154). 아울러 실행 가능성 판정은 preview에서도 수행한다 — 실행할 수 없는 계획을 preview가 성공으로 보여주면 운영자는 confirm에서 처음 막히고, 모드 자동 선택에서는 preview가 보여준 모드와 실제 모드가 달라진다(#152).
- PreToolUse 실행 가드(observation/typed control plane/owner mutation 3층 allowlist, exact matcher, 정적 분류 불가 명령 형태 거부)는 [ADR 2026-08-27](../adr/decisions/2026-08-27-session-start-owns-compaction-context.md)로 제거됐다. 명령 형태 판정이 필요한 곳은 `issueops` CLI/MCP가 자기 입력에서 직접 수행하며, hook에 되살리지 않는다. `internal/domain/commandparse`에는 IssueOps 명령 파서(`ParseExactIssueOpsCommand`, `ExactIssueOpsOwnerMutation`)만 남는다.
- `nondeterministic-context-serialization` rule은 immutable-prefix 결정성 계약에서 유래한 opt-in 규칙이다. agent가 stable cache prefix로 재사용하는 context를 만드는 파일은 `// harness:immutable-prefix` marker로 opt-in하고, 그 파일에서 `time.Now`/`rand`/`uuid` 같은 비결정 값을 도입하면 `warn`을 낸다. 의도된 volatile 값은 해당 줄에 `volatile-ok`를 달아 면제한다. volatile field 어휘와 stable projection은 `internal/domain/contextregion/context_region.go`(`VolatileContextFields`, `StableProjection`, `Region*` 상수)가 source of truth이며, response-contract golden의 dynamic time key 정규화와 같은 집합을 공유한다.

## Policy tier 컨벤션

- `PolicyTier`는 흩어진 capability 플래그(write/network/shell)를 host-neutral 명명 envelope로 *합성하는 분류*다. tier 계산(`resolvePolicyTier`)에 deny 판정 로직을 넣지 않는다. 명령 허용 여부는 `deny_reasons`가, 권한 envelope 이름은 `tier`가 책임진다.
- tier ladder는 `read_only` → `workspace_write` → `network_access` → `shell_exception` 순이며 most-privileged 차원이 이름을 정한다. 1회 승인이 세션 전체 등급을 올리는 YOLO/AUTO류 자동 승격 tier는 추가하지 않는다.
- tier를 추가/변경하면 `TestPolicyTierClassifiesEveryFlagCombination` table과 `command_policy` contract ResponseFields, response-contract golden을 함께 갱신한다.

## Doctor / lifecycle state conventions

- `issueops doctor`는 종합 진단 표면이고 기본 read-only다. 자동 수정은 별도 `--fix` 같은 명시 플래그가 있을 때만 추가한다.
- `issueops state doctor`는 checkpoint store 무결성 전용으로 유지한다. 사용자 안내와 troubleshooting 문서는 top-level doctor를 우선한다.
- `project bootstrap`은 target repo 문서와 별도로 user-state의 repo별 lifecycle namespace를 초기화한다. target repo에는 `.issueops/state/`나 schema 파일을 생성하지 않는다.

## State machine reducer contract

12-factor #12(stateless reducer)를 IssueOps 상태머신에 명문화한 계약이다. 코드에 이미 성립하는 불변식을 규율로 고정하는 것이지, 새 추상을 도입하는 것이 아니다.

- IssueOps phase 전이의 **판정(validation)은 균일하게 순수하지 않다**. readiness 게이트는 실측상 세 층으로 갈린다 — 이 분류가 계약이고, "전부 순수"라고 적지 않는다(#107, Codex 외부 검토로 확정).
  - **record-only 순수**: `IssueOpsProblemReadiness`(`internal/adapter/issueops/issueops_phase_ledger.go:27`)/`IssueOpsGrillReadiness`(`issueops_phase_ledger.go:35`)/`IssueOpsPlanReadiness`(`internal/adapter/issueops/issueops_readiness.go:13`)만 `IssueOpsRecord` 필드만 읽어 `Ready/Missing`을 돌려주며 clock·git·FS·network를 건드리지 않는다.
  - **FS 존재검사 수행**: `IssueOpsCompatibilityReviewReadiness`(`issueops_readiness.go:85`)/`IssueOpsImplementationReadiness`(`issueops_readiness.go:109`)/`IssueOpsAISlopCleanReadiness`(`issueops_readiness.go:73`)와 비-strict `IssueOpsPRReadiness`(`internal/adapter/issueops/issueops_pr_readiness.go:10`)는 `issueOpsWorktreePathValid`/`issueOpsPlanPathExists`/`issueOpsPlanPathInsideWorktree`(`issueops_readiness.go:193-205`)를 거쳐 `os.Stat`(`WorktreePathValid`/`PlanPathExists`, `internal/adapter/issueops/readinesspaths/paths.go:19-49`)과 `filepath.EvalSymlinks`(`PlanPathInsideWorktree`, 같은 파일)를 실행한다. 같은 record라도 디스크 상태가 바뀌면 결과가 바뀐다.
  - **git·network 수행**: `IssueOpsStrictPRReadiness`(`internal/adapter/issueops/issueops_pr_readiness_strict.go:12`)는 `rev-parse --is-inside-work-tree`(`:23`), `issueOpsCurrentHead`의 `rev-parse HEAD`(`issueops_readiness.go:261-270`), `branch --show-current`(`:28`), `status --porcelain=v1`(`:33`), upstream `rev-parse @{u}`(`:36`), 네트워크를 타는 `git fetch --quiet`(`:40`), `rev-list --left-right --count`(`:46`)를 직접 실행한다.
- **비결정·side-effect의 소유 경계는 게이트마다 다르다**(전부 wrapper 소유가 아니다). 전이 적용 함수 `applyIssueOpsPhaseTransition`(`internal/adapter/issueops/issueops_phase.go:143`) 밖에서 wrapper가 소유하는 것: wall-clock(`time.Now()`, `issueops_phase.go:144`), git read(`issueOpsCurrentHead`/`implementation.ChangeFingerprint`, `issueops_phase.go:155-156`), 디스크 write(`touchAndWriteIssueOps` 호출 `issueops_phase.go:75`, 정의 `internal/adapter/issueops/issueops_state.go:192`). 그러나 **판정 함수 `validateIssueOpsPhaseTransition`(`issueops_phase.go:78`) 자체가 IO를 실행한다**: PR 진입에서 strict 게이트를 직접 호출하고(`issueOpsStrictPRReadinessWithState`, `issueops_phase.go:125` → `issueops_pr_readiness_strict.go:97`) 그 안에서 git·network가 돈다. compatibility-review/implement/ai-slop-clean 진입 판정(`issueops_phase.go:104`, `:109`, `:114`)도 같은 이유로 FS를 읽는다. 규율은 "판정은 순수하다"가 아니라 **이 경계 안으로 새 IO를 더 밀어 넣지 않는다**이다.
- ledger stamp(`stampIssueOpsForwardTransition(ledger, prev, new, now)`, `issueops_phase_ledger.go`)는 `now`를 **주입받으면 순수**하다. 같은 `(record, to, now)`는 항상 같은 record를 낳아 replay/derive가 결정적이며, 이는 `internal/domain/issueopsstatus` projector의 결정성 테스트(`TestProjectorDerivesDeterministicLedger`, `projector_test.go`)로 보장된다.
- **신규 상태머신은 이 경계를 따른다**: 판정 로직에 clock/rand/uuid/IO를 섞지 않고, 비결정 입력은 값으로 주입한다(`nondeterministic-context-serialization` guard와 같은 정신). 결정성 검증이 필요해지고 두 번째 사용처가 생기면 그때 순수 함수를 *전체* 판정 블록 단위로 추출한다. 동작 무변화를 위해 `AdvanceIssueOpsPhase`를 선제적으로 리팩터하지 않는다(§28 게이트 파급이 큰 최고-민감 함수).

## Optional external orchestration adapter convention

- External orchestrators use one concrete adapter per verified boundary. The Orca adapter owns safe argv, bounded timeout/output, envelope decoding, and narrow DTO projection; it does not own IssueOps transitions and does not justify a registry/factory.
- Every external mutation follows `lock + persist pending -> unlock -> external call -> lock + compare-and-set result`. Never hold the cycle lock during an Orca/network call or mutating subprocess and never persist an observed identity against a stale attempt/epoch/context fence. A fixed read-only local Git checkpoint may run under the lock only when branch/HEAD/clean filesystem evidence must be sealed immediately before the same write.
- Completion projection is the narrow terminal-message variant: completed finish persists `submitted` result evidence plus a deterministic projection intent (or no-call diagnostic) in one cycle-lock write, releases the lock, and makes at most one argv-only `worker_done` call. Any persisted intent is a no-retry tombstone; post-call success/failure only annotates it and never rolls back submitted authority.
- Sole-writer attestation inventories every exact-worktree terminal plus server-filtered dispatched tasks immediately before each Orca create/dispatch boundary. Every connected or writable terminal, including baseline terminals, is a possible writer; only the designated active worker must be both connected and writable. Truncated, unparsable, incomplete, or duplicate identities persist `recovery_required` and never authorize replacement.
- Accepted publication is fenced to the full submitted `FinalHead`. Push requires the exact local branch ref at that SHA; PR/MR creation requires a durable provider-neutral receipt plus fresh local/remote ref verification for the same provider, remote, branch, and SHA. Provider adapters receive literal rendered body argv, never arbitrary local body-file paths.
- Failed/cancelled cleanup persists disposition before mutation and exact task/dispatch, terminal/PTY/worktree, then worktree-instance receipts in order afterward. Accepted handoffs cannot approve cleanup, and duplicate receipt writes are idempotent only for the same exact identity.
- `WorkerMailboxHandle` is the immutable dispatch assignee and completion sender. `WorkerTerminalHandle` is refreshable live control only. Runtime reconciliation may update the latter and runtime/PTY/tab/leaf evidence but never either sealed mailbox recipient.
- Timeout or transport error after invocation is ambiguous. Persist `recovery_required`; do not automatically repeat create/dispatch or switch to inline execution.
- Native worker identity is `(host, session_id, agent_id)` plus exact canonical worktree root. Host adapters forward that identity; common core decides ownership.
- CLI and MCP must use the same request/result DTOs. Keep the handoff MCP surface as one action-discriminated tool instead of multiplying near-identical lifecycle tools.
- **외부 시스템의 어휘는 그 시스템의 정의에서 인용하고 출처를 코드에 남긴다**(#171·#147·#180). CLI 출력에서 관측한 값의 집합은 어휘가 아니라 표본이다 — 그것으로 열거를 채우면 관측되지 않은 값이 "미지"로 오분류된다. Orca는 공개 저장소이므로 `DispatchStatus`/`GateStatus` 같은 union 정의와 `CHECK` 제약을 직접 읽고, provider API는 공식 문서의 필드 설명을 근거로 쓴다(GitLab `ref`는 "Branch name or commit SHA"). 인용 문구를 주석에 그대로 남겨 다음 독자가 다시 추측하지 않게 한다.
- **도구가 로컬에 없는 것은 계약을 확인할 수 없다는 뜻이 아니다**(#180). 설치 여부는 실측에만 영향을 준다 — 소스 읽기와 공식 문서 확인은 그대로 가능하다. `glab`이 없다는 이유로 GitLab 경로를 비범위로 둔 판단이 틀렸고, 상류 소스를 읽자 `ref` 문제와 **독립된 결함**이 나왔다(MCP 도구 인자가 실제 스키마와 달라 검증에서 실패하는 형태였다). 안내하는 MCP 도구의 인자 형태는 그 도구의 스키마를 만드는 코드에서 확정한다 — 이름만 맞고 형태가 틀린 안내는 따라 실행하면 실패한다.
- **분류 축을 섞지 않는다.** dispatch 수명주기와 task 수명주기는 다른 열거이고 종료 조건이 다르다 — dispatch가 `failed`여도 task는 `ready`로 남아 재시도를 기다린다(`db.ts` failDispatch). 한 축의 상태로 다른 축의 생존을 판정하면 재시도 가능한 사이클이 죽은 것으로 보인다(#147).
