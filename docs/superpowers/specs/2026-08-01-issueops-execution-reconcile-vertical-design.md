# IssueOps execution reconcile vertical 설계

## 범위

`execution reconcile`의 `worktree_create`, `owner_launch`, `dispatch` confirm만 기존 core orchestration에서 capability-local vertical로 이전한다. `remote_pr_create`, preview, no-pending, unsupported kind는 기존 경로를 유지한다.

공개 `ExecutionReconcileResult`, JSON/text/error, CLI/MCP 입력, schema v1, record와 external-intent raw bytes는 변경하지 않는다. 새 계층은 기존 raw-byte CAS와 receipt primitive를 harnessapp compatibility bridge 뒤에서 재사용한다.

## 구조

- `internal/contract/issueopslease`: snapshot, inventory, receipt, application result projection
- `internal/domain/issueopslease`: candidate/authority/invocation/attempts의 순수 `adopt|retry|preserve` 판정
- `internal/application/issueopslease`: 호출당 현재 stage 하나만 조정하는 service
- inbound adapter: core request/result mapping과 request-scoped issue reader 전달
- outbound adapter: `ReconcileEffects` port와 Orca inspect/invoke projection. core import 금지
- harnessapp: SQLite, `coreReconcileEffects`, Orca adapter composition
- core API: confirm+세 kind만 injected handler로 보내는 compatibility router. nil handler는 fail-closed

새 wiring은 `reconcileOrcaExecutionIntent`와 `executeOrcaIntentStage`를 호출하지 않는다. 전자는 production caller 0 뒤 제거하고, 후자는 prepare/resume shared caller 때문에 보존한다. core compatibility wrapper는 canonicalize/read/request/mark-invoking/record-failure/apply-receipt처럼 purpose-bound surface만 노출한다.

## 전이 계약

1. repository가 record/pending/payload raw bytes를 읽고 canonical marker를 검증한다.
2. exact legacy marker와 기존 안전 조건이 모두 맞을 때만 동일 CAS transition으로 승격한다.
3. 이 단계까지 실패하면 `external_state_inspected=false`, `legacy_intent_upgrade_unsafe`다.
4. inspection port는 `(inventory, attempted, error)`를 반환한다. local request/validation 실패는 attempted=false, transport 시도 이후는 성공/실패 모두 true다.
5. candidate 1개는 exact receipt를 적용한다.
6. multiple, non-authoritative zero, 생성 mutation의 unknown invocation, attempts 2는 pending과 raw bytes를 보존한다.
7. authoritative zero + `attempts < 2`에서 생성 mutation은 `not_invoked_proven`일 때만 invoke한다. 멱등 `run_bind`는 기존 호환 계약대로 unknown outcome도 같은 상한 안에서 수렴시킬 수 있다. attempts 0은 최초 시도, attempts 1은 한 번의 bounded retry다.

| pending kind | 현재 stage | 다음 stage | 공개 code |
|---|---|---|---|
| `worktree_create` | `worktree_create` | `terminal_create` | `orca_reconcile_advanced_terminal_create` |
| `owner_launch` | `terminal_create` | `run_create` | `orca_reconcile_advanced_run_create` |
| `owner_launch` | `run_create` | `run_bind` | `orca_reconcile_advanced_run_bind` |
| `owner_launch` | `run_bind` | `task_create` | `orca_reconcile_advanced_task_create` |
| `owner_launch` | `task_create` | `dispatch` | `orca_reconcile_advanced_dispatch` |
| `dispatch` | `dispatch` | pending 제거 | `orca_reconcile_completed` |

`worktree_create` receipt가 owner artifact를 만들 때만 `ExecutionActionDependencies.ReadIssue`에서 파생된 request-scoped reader를 쓴다.

## 검증

- attempts 0/1/2, 여섯 stage, candidate 1/0/multiple, authority/invocation matrix
- legacy/new JSON/error/text/raw-record/raw-intent differential
- inspection attempted truth table와 external mutation call count
- CLI/MCP production entrypoint와 request-scoped reader
- architecture forbidden imports, production fallback, legacy helper source ratchet
- focused unit/race, scoped vet, contract golden, build
- full test/full race는 GitHub Actions에서만 실행
