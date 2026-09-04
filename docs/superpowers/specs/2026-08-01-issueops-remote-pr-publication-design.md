# IssueOps 원격 PR/MR publication vertical 설계

## 상태와 결정

이 문서는 GitHub 이슈 [#195](https://github.com/m16khb-org/issueops/issues/195)의 승인된 구현 전 설계다. 기준 parent integration HEAD는 `667e5d15b0773e2550cfbf5bc2780506e9eb2896`, child branch는 `195-issueops-remote-pr-publication-vertical`이다.

기존 FS/HTTP/host/provider 수평 정리 범위는 폐기한다. `remote_pr_create`의 최초 생성과 그 durable recovery를 함께 소유하는 publication capability 하나만 수직 이전한다. 생성과 복구는 같은 schema v1 intent payload와 receipt CAS를 공유하므로 별도 child로 나누지 않는다.

이 문서를 원자 커밋한 뒤 사용자가 검토·승인하기 전에는 implementation plan을 작성하거나 구현 코드를 변경하지 않는다.

## 문제와 현재 근거

현재 원격 PR/MR 발행 lifecycle은 `internal/core/issueops/execution_remote.go`와 `execution_reconcile.go`에 걸쳐 있다. CLI와 MCP는 각자 provider create/reconcile closure를 조립하므로 production composition이 중복되고, 생성과 복구가 헥사고날 capability 경계 밖에서 SQLite, provider, artifact verification 세부사항을 함께 조정한다.

선행 이슈 #194는 Orca `worktree_create`, `owner_launch`, `dispatch` pending reconcile을 `issueopslease` vertical로 옮겼지만 `remote_pr_create`는 기존 router에 의도적으로 남겼다. 따라서 #195는 execution v1에서 남은 원격 publication lifecycle을 다음 production vertical로 완결한다.

CodeGraph와 exact parent ref에서 확인한 주요 경계는 다음과 같다.

- 공개 create DTO와 intent lifecycle: `internal/core/issueops/execution_remote.go:31-330`
- raw payload read와 exact candidate 검증: `internal/core/issueops/execution_remote.go:332-450`
- public reconcile router와 remote state machine: `internal/core/issueops/execution_reconcile.go:17-268`
- 중복 production wiring: `cmd/issueops/issueopscli/issueops_execution_cli.go:24-46`, `cmd/issueops/mcpcli/mcp_tool_issueops_execution.go:27-54`, `cmd/issueops/issueopscli/remotecmd/remote.go:709-786`
- provider DTO와 concrete adapter port: `internal/port/provider.go:33-98,220-230`
- #194 composition precedent: `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go`

## 목표

새 vertical은 다음을 하나의 consumer-owned capability로 제공한다.

- 기존 CLI/MCP 요청과 결과를 안정된 publication contract로 투영하는 inbound adapter
- 생성 자격, exact candidate 채택, authoritative zero와 단일 retry를 순수 판정하는 domain
- 동일 repository/provider/verification port를 공유하는 `CreateService`와 `ReconcileService`
- schema v1 raw bytes와 기존 transaction 의미를 보존하는 persistence adapter
- 기존 GitHub/GitLab concrete adapter를 호출하는 provider gateway
- 모든 production create/reconcile 진입점을 동일 service에 연결하는 issueopsapp composition

완료 후 core는 공개 facade와 아직 공유되는 raw persistence·artifact projection primitive만 보존한다. publication 전용 orchestration은 production caller가 0임을 architecture test로 증명한 것만 제거한다.

## 비목표

다음은 #195에 포함하지 않는다.

- issue/child 생성·close, completion reflection, execution complete 재설계
- branch preparation, push, Git config authority, provider 인증 정책 변경
- 일반 FS/HTTP/host/provider 수평 정리
- GitHub/GitLab concrete adapter의 native semantics 변경 또는 두 provider의 통합 abstraction 추가
- 새 CLI/MCP flag, response field, error code, IssueOps schema version 추가
- caller-zero 증거가 없는 shared helper나 package 제거
- OpenWiki 자동 갱신

## 패키지와 의존 방향

```text
CLI / MCP
    │
    ▼
inbound/issueopspublication
    │
    ▼
application/issueopspublication ──▶ domain/issueopspublication
    │                                │
    ├── consumer-owned ports         └── contract/issueopspublication
    │
    ▼
outbound/issueopspublication
    │
    ├── raw persistence compatibility bridge
    ├── existing GitHub/GitLab provider adapters
    └── existing live artifact verification

cmd/issueops/issueopsapp = 유일한 production composition root
```

패키지 책임은 다음과 같다.

- `internal/contract/issueopspublication`: service가 주고받는 stable snapshot, intent, candidate, receipt, result projection. core·port·concrete adapter를 import하지 않는다.
- `internal/domain/issueopspublication`: create eligibility와 `adopt|retry|preserve|terminal-not-invoked` 판정. 시간, filesystem, SQLite, provider 호출이 없는 순수 함수만 둔다.
- `internal/application/issueopspublication`: `CreateService`, `ReconcileService`, consumer-owned `Repository`, `Provider`, `Verifier` port. domain 결정을 실행하되 concrete package를 import하지 않는다.
- `internal/adapter/inbound/issueopspublication`: 기존 `RemotePullRequestRequest`, `ExecutionReconcileRequest/Result`와 새 contract 사이를 매핑하는 compatibility adapter. legacy core DTO import가 허용되는 유일한 새 inbound 경계다.
- `internal/adapter/outbound/issueopspublication`: application port 구현. raw persistence client와 기존 provider/verification 경계를 얇게 투영하고 정책을 재구현하지 않는다.
- `cmd/issueops/issueopsapp`: 실제 SQLite compatibility bridge, GitHub/GitLab gateway, verifier를 조립해 CLI와 MCP에 같은 handler를 제공한다.
- `internal/core/issueops`: 기존 public facade와 purpose-bound raw CAS primitive를 유지하고 새 handler로 forward한다.

contract, domain, application은 `internal/core`, `internal/port`, CLI/MCP, SQLite, concrete provider를 import하지 않는다. outbound는 application이 소유한 port를 구현하며, concrete 생성과 lifecycle 정책을 섞지 않는다.

## 공개 호환성 계약

다음 표면은 field 이름, JSON omission, text, error 의미까지 그대로 유지한다.

- `RemotePullRequestRequest`
- `port.IssueProviderCreatePullRequestRequest/Result`
- `ExecutionReconcileRequest/Result`
- CLI 성공 출력 `created: <URL>`과 preview 출력
- MCP success/error content와 `isError` 매핑
- 기존 `remote_reconcile_*`, `remote_reconcile_required`, `no_pending_external_intent`, `unsupported_external_intent` code
- `external_state_inspected`의 항상 존재하는 boolean 의미

preview는 provider inventory를 조회하거나 intent를 기록하지 않는다. pending kind가 `remote_pr_create`이면 기존처럼 `remote_reconcile_required`, `external_state_inspected=false`를 반환한다.

confirm의 phase, generation, active holder, actor/process ancestry, canonical CWD, head/base/SHA, linked issue, label, assignee authority 검증은 약화하지 않는다. handler가 없으면 기존 provider-unavailable 의미로 fail-closed하며 migrated remote PR 경로를 legacy orchestration으로 되돌리는 fallback은 두지 않는다.

#194 router의 다음 경로는 변경하지 않는다.

- Orca `worktree_create`, `owner_launch`, `dispatch`
- no-pending
- preview
- unsupported pending kind

## 영속 데이터와 byte 계약

schema version은 1로 유지한다. 새 persisted field, bucket, migration을 추가하지 않는다.

- bucket: `external_intent_v1`
- pending kind: `remote_pr_create`
- raw payload: 현행 `externalRemotePRPayload`와 동일한 field 순서·JSON tag·omitempty·값
- receipt: 현행 `RemoteArtifact` projection과 동일한 field 및 timestamp 의미
- failure: 현행 `ExecutionFailure` code와 bounded/redacted message 의미

새 vertical이 기록한 raw row를 legacy implementation이 그대로 decode하고 처리할 수 있어야 한다. legacy/new differential은 decode 결과만이 아니라 저장된 payload byte slice와 최종 record JSON을 비교한다.

payload는 다음 authority를 봉인한다.

- `schema_version`, `operation_id`, `generation`, `provider`, `kind`
- canonical provider create request 전체
- `invocation_state`, `retry_count`, optional `known_url`

`operation_id` marker는 기존 body suffix 형식을 유지하고 body hash 계산에 포함된다. GitLab draft title prefix 정규화, merged/closed 후보의 draft 일치 예외, canonical label/assignee set 비교도 현행 의미를 그대로 옮긴다.

## CreateService 데이터 흐름

1. inbound adapter가 기존 요청을 stable contract로 투영한다.
2. service가 provider availability와 public request validation을 수행한다. preview는 이 검증 결과로 기존 provider preview를 호출하되 영속 상태를 만들지 않는다.
3. confirm은 repository의 intent-begin CAS를 호출한다. CAS 안에서 최신 record와 generation/holder/CWD/phase/branch authority를 다시 검증하고, pending과 raw payload를 한 transaction으로 기록한다.
4. transaction과 SQLite cycle lock을 모두 해제한 뒤 provider create를 호출한다.
5. provider가 URL을 반환하면 existing live verifier로 project/kind/target/labels/assignees를 검증한다.
6. repository의 receipt CAS가 같은 operation과 payload를 다시 확인하고 `RemoteArtifact` 기록, pending/failure 제거, raw payload 삭제를 한 transaction으로 처리한다.
7. 성공 결과는 inbound adapter가 기존 CLI/MCP/public result로 되돌린다.

provider/network 호출이나 live verification 동안 SQLite lock을 보유하지 않는다. lock은 intent-begin, failure receipt, success receipt처럼 짧은 record/payload CAS에만 사용한다.

최초 create 호출의 timeout, nonzero, empty URL, verification 실패, success receipt CAS 실패는 pending을 보존한다. `IssueProviderCreateError.Invoked=false`이면 payload의 `invocation_state`를 `not_invoked_proven`으로 기록하고, 그 밖은 `unknown`으로 기록한다. 관측된 canonical URL은 별도 failure receipt CAS에서 `known_url`로 봉인한다. success receipt CAS 뒤 failure receipt CAS도 authority 변경 때문에 실패하면 오류를 그대로 반환하고 외부 mutation을 재시도하지 않는다. 최초 create 실패는 자동 재시도하지 않고 execution reconcile을 요구한다.

## ReconcileService 데이터 흐름

1. public router가 preview, no-pending, Orca kind, unsupported kind를 기존 의미대로 먼저 분기한다.
2. `remote_pr_create` confirm만 inbound adapter를 통해 `ReconcileService`로 보낸다.
3. repository가 pending과 raw payload를 읽고 schema, operation, generation, provider, kind를 검증한다.
4. provider inventory는 lock 밖에서 정확히 한 번 조회한다. 조회를 실제 시도한 모든 반환 경로는 `external_state_inspected=true`다.
5. domain이 candidate 수, authoritative-zero, invocation state, retry count로 `adopt|retry|preserve`를 판정한다.
6. mutation이 필요하면 repository CAS가 operation과 raw payload가 아직 동일한지 확인한 뒤 적용한다.

### Exact candidate 채택

candidate가 정확히 하나일 때만 채택 후보가 된다. 다음 값이 모두 봉인된 intent와 일치해야 한다.

- target project와 source project
- head branch, base branch, expected head SHA
- normalized title, body SHA-256
- draft와 live state
- canonical labels와 assignees
- `known_url`이 있으면 exact URL
- provider/kind와 linked issue authority에 맞는 artifact URL

일치 후에도 live verifier가 통과해야 receipt CAS를 적용한다. mismatch, verification failure, receipt CAS failure는 새 mutation을 호출하지 않고 pending을 보존하며 기존 `remote_reconcile_*` code를 반환한다.

### Zero, multiple, retry

- candidate 복수: `remote_reconcile_multiple`, pending 보존
- non-authoritative zero: `remote_reconcile_zero_ambiguous`, pending 보존
- authoritative zero지만 absence가 증명되지 않음: `remote_reconcile_zero_unproven`, pending 보존
- retry가 이미 소비됐거나 create gateway가 없음: `remote_reconcile_retry_exhausted`, pending 보존
- authoritative zero + `not_invoked_proven` + `retry_count=0`: retry marker CAS를 먼저 적용한 뒤 create를 정확히 한 번 호출

retry marker CAS는 `invocation_state=unknown`, `retry_count=1`을 외부 호출 전에 저장한다. CAS가 실패하면 provider를 호출하지 않는다. retry 성공은 live verify 후 success receipt CAS로 끝난다.

retry 호출이 다시 pre-invocation-proven으로 실패하면 pending과 payload를 원자 삭제하고 failure code `external_operation_not_invoked`를 기록해 `remote_reconcile_retry_not_invoked`로 terminal 수렴한다. timeout/nonzero/unknown invocation, empty URL, verification 실패, receipt CAS 실패는 pending을 보존하며 두 번째 retry를 허용하지 않는다.

## 오류와 원자성 표

| 사건 | provider 호출 | durable 결과 | 재호출 정책 |
|---|---:|---|---|
| intent-begin CAS 실패 | 0 | 기존 상태 유지 | authority 재확인 |
| 최초 create pre-invocation failure | 1회 시도, invoked=false | pending + `not_invoked_proven` | reconcile에서만 1회 retry 가능 |
| 최초 create ambiguous failure | 1회 시도 | pending + `unknown` | 자동 retry 금지 |
| URL 없음 또는 verify 실패 | 1회 시도 | pending, URL이 있으면 `known_url` | inventory reconcile |
| success receipt CAS 실패 | 1회 시도 | pending 보존, authority가 유지되면 `known_url` 봉인 | inventory reconcile |
| exact candidate 1 + verify 성공 | 0 | artifact 기록, pending/payload 삭제 | 완료 |
| candidate multiple/ambiguous zero | 0 | pending 보존 | operator 재확인 |
| authoritative zero + retry 조건 충족 | 최대 1회 | retry marker를 먼저 CAS | 두 번째 retry 금지 |
| retry pre-invocation failure | 1회 시도, invoked=false | pending/payload 삭제 + terminal failure | 완료 |
| retry ambiguous/verify/receipt 실패 | 1회 시도 | pending + retry_count=1 | 추가 mutation 금지 |

어떤 경로도 provider mutation과 SQLite transaction을 겹치지 않는다. CAS 실패를 성공으로 투영하지 않으며, known URL이 있는 상태에서 다른 URL candidate를 채택하지 않는다.

## Routing과 composition

모든 production surface는 다음 한 경로를 사용한다.

```text
CLI remote create-pr ─┐
                     ├─▶ issueopsapp publication handler ─▶ CreateService
MCP remote create-pr ─┘

CLI execution reconcile ─┐
                         ├─▶ core public router ─▶ ReconcileService
MCP execution reconcile ─┘
```

CLI/MCP는 concrete publication provider closure를 직접 조립하지 않는다. issueopsapp만 기존 provider resolver, live verifier, raw persistence compatibility bridge를 조립한다. core facade는 함수명과 public DTO를 유지하면서 injected handler로 forward한다.

migrated `remote_pr_create` handler가 nil이면 legacy helper를 호출하지 않는다. 반면 #194의 Orca handler, preview/no-pending/unsupported routing은 그대로 유지해 두 vertical이 서로의 pending kind를 소유하지 않게 한다.

## Architecture fitness

정적 architecture test는 다음을 거부한다.

- contract/domain/application의 core, port, adapter, SQLite, CLI/MCP import
- domain의 filesystem, clock, network, provider 호출
- CLI/MCP의 concrete publication provider wiring
- migrated production path의 legacy remote publication fallback
- provider/network 호출을 감싼 SQLite cycle lock
- caller가 남은 publication orchestration 삭제

production caller-zero ratchet은 제거 대상 helper의 이름만 세는 것이 아니라 CLI, MCP, issueopsapp, core router에서 새 handler가 실제로 사용되는지를 함께 확인한다.

## 테스트 계약

TDD는 production code보다 named RED를 먼저 기록한다. 최소 matrix는 다음과 같다.

- public DTO/JSON/text/error/MCP `isError` differential
- raw record, pending, payload byte slice, receipt JSON legacy/new differential
- create preview/no-write/no-inventory와 confirm authority validation
- provider mutation 전 intent 존재 및 호출 중 lock 미보유
- create timeout/nonzero/empty URL/verify/receipt CAS failure와 known URL
- candidate 1/0/multiple, exact field mismatch 각각
- authoritative/non-authoritative zero × invocation unknown/not-invoked × retry_count 0/1
- retry marker CAS failure 시 create call count 0
- retry 성공, pre-invocation terminal failure, ambiguous failure의 call count와 pending 결과
- GitHub/GitLab draft title/state와 candidate inventory parity
- CLI/MCP production entrypoint가 같은 handler instance를 사용하는지
- architecture forbidden imports, zero fallback, caller-zero ratchet

로컬 focused 검증은 다음 명령으로 고정한다.

```bash
go test ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
go test -race ./internal/domain/issueopspublication ./internal/application/issueopspublication ./internal/adapter/inbound/issueopspublication ./internal/adapter/outbound/issueopspublication -count=1
go test ./internal/core/issueops -run 'RemotePullRequest|RemoteReconcile' -count=1
go test ./internal/adapter/provider/github ./internal/adapter/provider/gitlab -run 'CreatePullRequest|ReconcilePullRequest' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'RemotePullRequest|CreatePR|Reconcile|ExecutionHandler' -count=1
go test ./internal/architecture -run Dependency -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go vet ./internal/domain/issueopspublication/... ./internal/application/issueopspublication/... ./internal/adapter/inbound/issueopspublication/... ./internal/adapter/outbound/issueopspublication/...
go build -o bin/issueops ./cmd/issueops
```

로컬에서 전체 `go test ./...`와 전체 race는 실행하지 않는다. parent #117 정책에 따라 GitHub Actions의 마지막 완전한 run에서 확인한다.

## 완료 기준

- AC-195-01: 공개 CLI/MCP·JSON/text/error와 schema v1 raw bytes 호환성을 유지한다.
- AC-195-02: create가 intent-first, lock-free external call, atomic receipt/failure CAS를 지킨다.
- AC-195-03: reconcile exact candidate/zero/multiple/invocation/retry/known URL matrix를 유지한다.
- AC-195-04: 모든 production create/reconcile caller가 새 vertical을 사용하고 legacy fallback이 0이다.
- AC-195-05: focused unit/race, architecture, provider, CLI/MCP golden, scoped vet/build와 원격 CI를 통과한다.

## Rollback과 문서 영향

schema/data migration과 새 persisted field가 없으므로 child PR revert가 유일한 rollback이다. 새 vertical이 기록한 bytes는 legacy implementation이 그대로 읽을 수 있어야 하며, rollback 때문에 별도 cleanup command나 state migration을 요구하지 않는다.

구현 증거가 기존 운영 설명을 바꾸면 `.issueops/ARCHITECTURE.md`, `CONVENTIONS.md`, `OPERATIONS.md`, `TESTING.md`를 surgical하게 갱신한다. 현재 단계에서는 설계 문서와 #195 원격 계약만 source of truth로 추가하며 OpenWiki는 수정하지 않는다.

## 열린 질문

범위, package 경계, data flow, 오류 원자성, 호환성, routing, 검증, rollback, no-split 결정은 모두 사용자 승인으로 닫혔다. 구현 전 남은 gate는 이 문서 자체에 대한 사용자 검토 승인뿐이다.
