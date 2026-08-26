# IssueOps capability verticals and ownership

> Family index: [`../ARCHITECTURE.md`](../ARCHITECTURE.md). This module owns the
> IssueOps v1 execution state, schema authority, capability verticals, the Orca
> execution boundary, the execution threat model, and the actor model. General
> state topology and lock discipline live in [`runtime.md`](runtime.md);
> component boundaries live in [`hexagonal-core.md`](hexagonal-core.md).

## IssueOps v1 execution state and schema authority

- IssueOps 위치: `~/.local/state/agent-harness/issueops_v1/harness.db`, bucket `issueops_v1`. 한 row는 lifecycle evidence와 정확히 하나의 `Execution`을 저장한다. Execution은 canonical workspace, direct/Orca mode, generation-fenced lease, native process receipt, pending external intent, Orca resource identity, generation에 봉인된 issue-body/context-packet/owner-prompt digest identity, completion receipt를 가진다. terminal preparation intent가 삭제된 뒤에는 identity version 1과 이 세 digest가 resume의 trust root이며 현재 prompt template을 다시 렌더링하지 않는다. Version marker 없는 all-empty binding만 과거 record로 복구할 수 있고, versioned all-empty는 새 persistence invariant 위반으로 거부한다. 사용자 요청과 설계 검토 같은 freeform 값은 secret-like 패턴을 redaction한 뒤 저장한다.
- IssueOps v1의 현재 쓰기 버전은 `schema_version=1`이다. Missing/zero/future schema row와 legacy write-authority key(`execution_handoff`, `execution_workspace`, `ownership`, `remote_create_claim`)를 가진 row는 모두 generic `invalid state`로 byte-identical fail-closed다(`internal/adapter/issueops/execution_namespace_test.go`). Legacy namespace와 row/file은 자동 변환하지 않으며, 전용 reset 명령도 없다: 비현행 schema를 만나면 record를 다시 시작한다.

## Capability verticals

- 태스크 게이트 ledger는 PR readiness에 파일 존재 기반 opt-in으로 합성된다.
  worktree에 canonical `.agent-harness/gates/*.md` 또는 호환
  `GATES.md`/`gates/*.md`가 있으면 미충족 게이트(unchecked,
  또는 checked-but-EVIDENCE-pending)가 `gates_incomplete:<file>` missing으로 pr
  단계 진입을 막는다. ledger가 없으면 아무 요구도 추가하지 않는다. 게이트
  조회·평가 연산은 함수 변수로 주입되고 composition root만 배선한다(`loopgate`
  합성과 같은 구조). 상세 계약은
  [ADR 2026-08-22](../adr/decisions/2026-08-22-task-gate-ledger.md)를 참조.
  신규 IssueOps cycle은 root `GATES.md`나 `gates/`를 만들지 않고
  `.agent-harness/gates/issue-<provider-issue-number>.md`를 커밋한다. 기존
  root 경로는 unlazy ledger의 읽기 호환 경로로만 남는다.

- `execution release`는 첫 production vertical이다. CLI/MCP transport facade는 injected release handler만 호출하고, `internal/contract/issueopslease`의 stable v1 canonicalization → pure `internal/domain/issueopslease` → capability-local `internal/application/issueopslease` → inbound/outbound adapter 순서로 흐른다. `cmd/harness/harnessapp`만 SQLite store, process observation, clock, filesystem path matcher를 조립하며, 기존 two-argument `ReleaseExecution`은 외부 Go surface와 differential oracle을 위한 compatibility facade로만 남는다.
- `execution reconcile`의 Orca `worktree_create`·`owner_launch`·`dispatch` confirm도 같은 vertical 경계를 사용한다. kind-local router가 injected handler로 보내고, application은 호출당 현재 durable stage 하나만 inventory/adopt 또는 bounded retry/CAS한다. preview와 no-pending은 side effect가 없는 compatibility router에 남는다.
- 원격 PR/MR 생성과 `remote_pr_create` 복구는 `issueopspublication` capability vertical이다. `internal/contract/issueopspublication`의 stable mapping → pure domain decision → shared `CreateService`/`ReconcileService` → inbound/outbound adapter 순서로 흐르며, `cmd/harness/harnessapp`만 provider, raw schema v1 CAS bridge, live verifier를 조립한다. CLI create와 CLI/MCP reconcile은 같은 request-scoped handler pair를 사용하고, handler가 없으면 legacy full-flow로 우회하지 않고 fail closed한다.
- 최초 parent issue 생성은 record의 `IssueCreateIntent`가 권위다. Provider 호출
  전에 operation marker, provider, canonical project authority, title/body
  digest, labels/assignees를 원자 기록한다. 호출 시작 전 실패만 같은 sealed
  request의 재시도를 허용하고, 시작 후 timeout/error/malformed output/live
  verification 실패는 자동 재시도 없이 reconciliation을 요구한다. 정확히 한
  marker candidate만 title/body digest와 live label/assignee 검증 뒤
  `IssueURL`+completed receipt 한 CAS로 채택한다.

## IssueOps operational surface

- IssueOps 제공 표면: 기존 lifecycle/domain CLI와 함께 `issueops execution prepare/status/claim/release/replace/reconcile/switch-mode/complete`, generation-fenced `issueops remote create-pr`를 제공한다. 이원 구조 운영 표면으로 `issueops artifact stage/unstage`(prepare 전 스테이징·materialize·orca packet manifest 봉인), `issueops implementation-review record`(orca 모드 publication fail-closed 게이트, 변경 집합 fingerprint 바인딩), `issueops list`(read-only 다중 사이클 집계, 단일 SQLite snapshot의 물리 `scanned_records`, bounded invalid-row diagnostics와 pending/failure/cleanup-failure/issue-create projection), `issueops cleanup finish`(record-backed 머지 후 정리 — orca 회수→git worktree 제거→브랜치 CAS 삭제→감사 라인 멱등 반영→레코드 삭제, resumable), `issueops remote create-issue/reconcile-issue`(intent-first parent issue 생성과 zero/one/many marker reconciliation), `issueops remote reflect-completion/close-issue`(completion 섹션 보존·부모 이슈 close, 원격 readback fail-closed)를 제공한다. execution prepare는 `--owner-model` 미지정 시 host별 implementer 기본값(codex gpt-5.6-terra/xhigh, claude claude-sonnet-5/high)을 적용하고, owner 프롬프트에 planner급 reviewer 모델(codex gpt-5.6-sol/xhigh, claude claude-opus-5/high)을 렌더한다. Claude의 Fable 5는 자동 기본값이나 폴백으로 쓰지 않고 명시적 수동 지정으로만 사용한다. IssueOps MCP 표면은 정확히 하나인 `issueops_execution`이며 action으로 같은 execution state machine을 호출한다. `execution prepare`가 provider branch의 exact base SHA에서 fixed sibling worktree를 만들고, direct는 caller에게 generation 1을 부여하며 Orca는 sealed packet/prompt/token file과 claimable lease를 만든다. External mutation은 intent-first이고 ambiguity는 reconcile 전까지 fail closed다. `execution complete`는 phase `pr`, active generation, final HEAD, committed Turing report, verification, exact verified remote URL을 요구하며 `done` 전이와 lease release를 원자적으로 기록한다.

## Generated command authority

- IssueOps가 생성하는 `next_command`는 prepare/status/replace/resume/sync-base/switch-mode preview와 cleanup preview/finish를 포함해 첫 token을 생성 바이너리의 canonical executable literal로 렌더하고 같은 path·SHA-256·lease generation envelope를 포함한다. 실행 authority를 제거하는 switch-mode apply는 shell command를 만들지 않고 non-command `next_action`만 반환한다. 사람이 직접 입력하는 일반 `agent-harness` PATH UX는 바꾸지 않는다. CLI와 MCP composition은 outbound executable observer를 통해 같은 envelope를 결합하고, IssueOps root는 subcommand mutation 전에 현재 executable과 durable generation을 대조한다. Hook은 absolute token이 envelope와 일치하는 것만으로 신뢰하지 않고 durable worktree 또는 source root의 canonical `bin/agent-harness`인지 먼저 제한한다. 관측 실패·stale binary·generation drift에는 command를 내보내거나 fallback하지 않으며 structured error로 중단한다. Contract는 DTO·pure bind/validate만 소유하고 port는 contract를 import하지 않는 순수 observation receipt를 소유한다. 실제 observer 생성은 `harnessapp` composition root에만 두고 executable 관측 I/O는 outbound adapter에만 둔다.

## Actor model

- Actor model: main agent는 safety/reversibility/user-intent judgement와 child result acceptance를 소유한다. IssueOps의 active native holder는 exact lifecycle ID, generation, process receipt, canonical cwd 안에서만 쓴다. Hook은 관찰·차단·relay만 담당하고, phase 진행·workspace 준비·테스트·publication·merge·cleanup을 대신 실행하지 않는다.

## Optional Orca execution boundary

Orca integration is an optional execution adapter, not a native-install
dependency or second scheduler. `issueops execution prepare --mode auto`
probes readiness before mutation. `auto` resolves to direct only when Orca is
absent or unready at that pre-mutation boundary. After a possible Orca mutation,
the durable pending intent and explicit reconciliation path are authoritative.
Provider capability is part of that boundary. GitLab-linked execution accepts
one bounded, exact-identity issue snapshot observed through a host-configured
`glab_api` capability, or reads the same fields through the generic `glab api`
provider adapter when no MCP snapshot is supplied. MCP server namespace,
personal wrapper identity, credential profile, and token remain outside core;
only the normalized provider/source/URL/body/state DTO crosses the port. Exact
base SHA and branch upstream are separate identities; the SHA creates the
worktree and the remote issue branch is restored as upstream after namespace
canonicalization.
엄브렐라 자식 cycle은 branch prepare의 명시적 `parent_worktree`와
`base_branch`에서 canonical 부모 worktree 경로를 봉인한다. 기존 delegation
cycle은 명시값이 없을 때 같은 경로를 계산해 하위 호환한다. Orca adapter는 생성 시 그 경로를
`--parent-worktree`로 명시하고, 응답의 lineage가
`explicit-cli-flag`/`explicit`인지 검증한다. 독립 cycle만 `--no-parent`를
사용하므로 Orca UI 계층과 IssueOps의 provider-native 부모 관계가 일치한다.

## IssueOps execution v1 threat model and invariants

### Adversarial multi-session model

- One record has one `Execution`, one canonical worktree, and one active
  generation at a time.
- The trust boundary is the exact native actor: host, session/agent ID, process
  PID/start/executable receipt, canonical cwd, lifecycle ID, and generation.
- Branch names, source cwd, generic session bindings, terminal handles, and
  stable diffs are not write authority.
- Hooks are default-deny guards for mismatched mutation, not schedulers or lease
  grantors.

### Generation fence and sealed owner context

- Every mutating transition requires the active generation and matching native
  actor/cwd. Stale generations fail before CAS.
- Direct preparation grants generation 1 to the caller. Orca preparation stores
  a claimable generation and seals the remote issue digest, private context
  packet, fully rendered prompt, owner host/model/effort, and stable Orca
  resource IDs.
- Orca claim resolves and consumes the current-generation private token exactly once and requires both
  sealed SHA-256 values. Token contents never enter state, prompts, logs, or
  responses.

### External intent and lock discipline

- Workspace and remote PR/MR creation persist intent before calling the adapter.
  Timeout or error is ambiguity, not absence; retry and mode fallback remain
  blocked until `execution reconcile` proves one exact outcome.
- Parent issue creation follows the same intent-first rule independently of the
  execution lease. Only a proven `not_invoked` outcome is retryable.
  `invoked_unknown`, observed URL, verification failure, and receipt failure are
  Doctor findings until `remote reconcile-issue` adopts one exact candidate.
- sqlstore `BEGIN IMMEDIATE` spans serialize record CAS. No Git, provider, or
  Orca process call runs while the cycle lock is held.
- Remote intent stores generation and native actor. Finish/reconcile rejects a
  changed generation, holder, cwd, branch, or provider result.

### Replacement and completion

- Replacement is preview → revoke → finalize-preview → finalize. Inventory and
  quiescence fingerprints, expected generation, actor, cwd, and explicit
  confirm are required; there is no unsafe override.
- Completion requires phase `pr`, active generation, exact final HEAD, committed
  Turing report, verification evidence, and a durable verified remote artifact
  at the exact URL. The completion receipt, `done` transition, and lease release
  are one atomic state mutation.
- Completion never merges or deletes local/remote resources. Cleanup remains a
  separate human-authorized operation based on current merge and cleanliness
  evidence.
- A completed replacement first observes the fetched parent base through the
  `issueopsbasesync` port. Parent drift returns the typed
  `post_completion_sync_base_required` contract before owner inventory,
  artifact preparation, token creation, completion archival, or record CAS.
- Post-completion sync authority is the released lease plus its current stamped
  completion generation, canonical cwd, live native actor, and preview
  fingerprint. Completion history never restores current authority. The only
  recovery sequence is released current completion + drift → sync-base preview
  → generation/fingerprint-fenced sync-base apply → replacement preview →
  reseed/claim → verify and re-complete.
- `internal/port/issueopsbasesync` owns only request, receipt, and interface.
  The public typed error and exact next-command projection belong to
  `internal/contract/issueops`; Git/network observation belongs to the outbound
  adapter.
- The #303 provenance boundary binds typed-error `next_command` on the reachable
  CLI and MCP error paths. CLI conflict `next_command` and `abort_command` share
  one observed canonical executable/hash/generation receipt; observation
  failure exposes no unbound fallback. As #326 records, the public MCP action
  enum excludes sync-base and therefore has no sync-base success action/result;
  AC-06 host parity is the exact CLI command plus Codex/Claude hook classifier,
  while MCP binds only reachable resume/replace `BaseSyncRequiredError` output.

## Execution boundary

Workspace provisioning and lease grant are one execution transaction. The
source main worktree remains available before, during, and after direct or Orca
execution for unrelated work. A generic session binding is routing metadata
only. The fence selects the exact lifecycle ID, generation, native process
receipt, canonical worktree, and persisted Orca identity.

One active execution exists per record, not per source repository. Exact-ID
routing therefore keeps parallel cycles independent. The active holder performs
the remaining gates, implementation, publication, and completion in its
canonical worktree. Completion records `done` and releases the generation;
later merge and cleanup require separate current evidence and authority.

Post-merge cleanup ordering is a contract: `reflect-completion`(completion
섹션에 최종 head·PR URL·검증 요약·artifact 본문 보존) → `close-issue` →
`cleanup finish`. finish는 preview 게이트(원격 readback fail-closed·터미널
quiescence·head OID CAS·fingerprint) 뒤에만 파괴 단계를 수행하고 마지막에
레코드를 삭제한다 — 결정적 ID(`sha256(repo+branch)`) 재사용과 충돌하지 않는
유일한 수명 종료다. 각 파괴 단계는 멱등이며, 실패 시 레코드가 보존되고 재실행
전 preview 재발급이 요구된다. prune은 completion 미반영 + RemoteArtifact 보유
레코드를 나이와 무관하게 보존한다(보존 불변식). staged artifact의 수명은
레코드와 같다(deleteIssueOps가 스테이지 버킷을 동반 삭제).
