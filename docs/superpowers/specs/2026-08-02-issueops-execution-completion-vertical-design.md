# #198 — IssueOps execution completion vertical 설계

## 목표와 경계

`execution complete` 한 capability를 기존 `internal/core/issueops` orchestration에서
contract/domain/application/inbound/outbound 경계로 옮긴다. 공개 CLI/MCP 요청과
응답, schema v1 persisted bytes, 오류 의미, lifecycle hook authority는 바꾸지 않는다.

이번 범위는 다음 원자 전이만 소유한다.

```text
verified PR/MR + active exact holder + final evidence
  -> completion receipt 저장
  -> lease active -> released
  -> holder reverse index 삭제
  -> phase pr -> done + phase ledger 갱신
```

원격 issue completion reflection, issue close, `sync-base`, `switch-mode`, cleanup은
후속 capability다. provider mutation을 durable completion transaction에 포함하지 않는다.

## 현재 문제

`CompleteExecution`은 다음 책임을 한 함수에서 수행한다.

- 공개 요청 정규화와 오류 판정
- verified artifact, holder, generation, canonical CWD 검증
- Git HEAD와 Turing report 파일 관측
- completion receipt, lease, holder index, phase ledger 원자 갱신
- durable commit 이후 Orca task settle

또한 `ExecuteExecution`의 complete 분기는 다른 이전된 lease action과 달리 injected
handler 없이 legacy 함수에 직접 진입한다. CLI composition도 concrete Orca client를
직접 `executioncmd.Deps`에 넣는다. 이 구조는 public behavior는 맞지만 capability-local
dependency direction과 handler-only routing을 증명하지 못한다.

## 제안 구조

```text
CLI / MCP
  -> internal/core/issueops compatibility request
  -> inbound/issueopscompletion Handler
  -> application/issueopscompletion Service
       -> Repository (atomic snapshot/apply)
       -> EnvironmentVerifier (CWD, HEAD, report, artifact)
       -> Clock
       -> TaskSettler (post-commit best effort)
  -> outbound/issueopscompletion adapters
       -> SQLite/raw IssueOps persistence
       -> filesystem/Git/artifact verification
       -> Orca task gateway
```

### Contract

`internal/contract/issueopscompletion`은 transport-neutral command, actor/process,
record/lease/artifact snapshot, completion receipt와 result projection을 소유한다.
JSON tag와 schema persistence 구현은 소유하지 않는다. 기존 core DTO와 stable public
shape가 source of truth이며 inbound adapter가 양방향 mapping한다.

### Domain

`internal/domain/issueopscompletion`은 외부 I/O 없이 다음 순수 판정을 수행한다.

- apply / idempotent / deny 결정
- phase, verified artifact, prepared target branch invariant
- active holder, generation, canonical authority projection 일치
- final evidence 정규화와 기존 receipt equality
- completion receipt, released lease, phase ledger를 포함한 next snapshot 생성

시간은 입력으로 받는다. domain은 `time.Now`, filesystem, Git, SQLite, Orca를 알지 않는다.

### Application

application service의 순서는 legacy 오류 및 side-effect 순서를 보존한다.

1. command 구조와 confirm/verification/URL을 검증한다.
2. repository snapshot을 읽는다.
3. 이미 terminal이면 동일 evidence만 멱등 성공시킨다.
4. current holder authority와 canonical CWD를 검증한다.
5. worktree HEAD, report regular-file/in-root, artifact projection을 관측한다.
6. clock을 읽고 domain transition을 만든다.
7. record 갱신과 holder index 삭제를 한 repository apply로 커밋한다.
8. 커밋 뒤 lock 밖에서 Orca task settle를 best-effort로 실행한다.

repository failure 전후 snapshot이 같아야 한다. settler 실패는 durable completion을
롤백하지 않고 기존 `orca_task_error`로만 노출한다.

### Adapters와 composition

inbound adapter는 기존 `ExecutionCompleteRequest`와 `ExecutionResult`를 보존한다.
outbound persistence adapter는 기존 raw model과 SQLite CAS primitive를 재사용하되
application/domain에 core model을 노출하지 않는다. harnessapp만 concrete repository,
filesystem/Git verifier, clock, Orca gateway를 조립한다.

`ExecutionActionDependencies`에는 complete handler를 추가한다. production complete
분기는 handler가 없으면 typed unavailable error로 fail-closed하며 legacy orchestration에
fallback하지 않는다. CLI와 MCP는 동일 harnessapp handler를 소비한다.

## 호환성 oracle

첫 RED는 legacy/new에 같은 fixture와 deterministic clock을 넣어 다음을 비교한다.

- public result JSON과 normalized error string/classification
- persisted IssueOps raw JSON bytes
- holder reverse index 존재/삭제
- phase와 phase ledger timestamps
- Orca settler call count, args, error projection

성공, identical retry, different-evidence retry, wrong phase/artifact/target, non-holder,
generation/CWD/HEAD/report/verification/URL mismatch, persistence failure, direct/orca/nil
settler를 포함한다. legacy/new 비교가 불가능한 항목만 명시적 projection으로 고정하고
나머지는 byte-for-byte로 비교한다.

## 훅 활성 환경 계약

호스트 hook은 structured shell `workdir`을 canonical path base로 사용한다. 구현과
검증, commit, push, publication 명령은 active generation holder가 exact child worktree
CWD에서 수행한다. source checkout에서 child 파일을 수정하거나 holder 없는 remote
mutation을 전제로 하지 않는다. 새 vertical은 기존 lifecycle guard와 hook deny code를
변경하지 않는다.

## Rollback

schema version, persisted field와 data migration을 추가하지 않는다. 새 vertical이 쓴
bytes를 legacy implementation이 그대로 읽을 수 있어 child PR revert가 완전한 rollback이다.
caller-zero가 증명되지 않은 shared model, artifact verifier, persistence primitive는 유지한다.

## 완료 기준

- deterministic legacy/new differential 전 matrix 통과
- handler-only CLI/MCP production routing과 legacy fallback 0
- contract/domain/application dependency fitness 통과
- focused unit/race, CLI/MCP golden, scoped vet/build 통과
- final child head의 GitHub CI 전체 test/race/self-verify 통과
- PR을 `117-hexagonal-architecture-migration`에 병합하고 #198만 정리
