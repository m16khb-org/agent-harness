# 128 cleanup audit의 completion 캐시 stamp 누락 구현 계획

- 이슈: https://github.com/m16khb-org/issueops/issues/128
- 부모 백로그: https://github.com/m16khb-org/issueops/issues/99
- IssueOps: io-654aec2b0fc0 / direct / generation 1
- 브랜치: 128-cleanup-audit-stamp (base main, base head 85b7e3baa004a1624719a0b3aec12c931e9e490c)

## 문제 요약

completion 섹션을 원격에 쓰는 경로가 둘인데 한쪽만 로컬 캐시를 갱신한다.

- `ReflectIssueCompletion`: 성공 시 `stampRemoteCompletion`으로 `ReflectedAt` 기록
- `ReflectCleanupAudit`: completion payload **전체**를 원격에 쓰지만 stamp 없음

`cleanup remote-branch`가 후자로 audit을 반영하고 **레코드를 유지**하므로, 직후 `issueops list`가 원격에 반영된 사이클을 거짓으로 미반영이라 보고한다. 118 사이클에서 관측됐다.

`cleanup finish`는 `completion_reflected`를 원격 readback으로 독립 판정하므로 파괴 경로는 안전하다. 문제는 진단 표면의 정확성이다.

## 설계

`ReflectCleanupAudit`에 `stateRoot`를 더하고 원격 반영 성공 직후 기존 `stampRemoteCompletion`을 재사용한다.

- 실패나 미확인 시에는 stamp하지 않는다. audit 반영은 best-effort이므로 실패가 캐시를 오염시켜서는 안 된다.
- 호출부는 `cleanup finish`와 `cleanup remote-branch`의 `ReflectAudit` 주입점 두 곳이며 둘 다 이미 `stateRoot`를 안다.
- 집계 로직과 finish의 원격 readback 게이트는 건드리지 않는다.
- 새 상태 필드와 새 표면을 만들지 않는다.

## RED 테스트

- audit 반영 성공 후 레코드의 `RemoteCompletion.ReflectedAt`이 채워지는지 → 현재는 비어 있어 실패
- 반영이 실패하면 stamp되지 않는지
- 진짜 미반영 사이클은 여전히 `completion_unreflected`로 보고되는지(회귀 금지)

## 수용 기준 매핑

| AC | 검증 |
| --- | --- |
| AC-01 stamp | audit 반영 후 캐시 확인 테스트 |
| AC-02 list 정확성 | 캐시가 채워지면 집계가 자동으로 정확해짐(같은 테스트로 확인) |
| AC-03 실패 시 미stamp | 실패 주입 테스트 |
| AC-04 finish 게이트 불변 | 기존 finish 테스트 통과 |
| AC-05 회귀 금지 | 미반영 사이클 보고 테스트 |
| AC-06 RED 선행 | 각 테스트를 구현 전 실행해 실패 확인 |

## 검증 명령

```bash
go -C /Users/m16khb/Workspace/issueops.worktrees/128-cleanup-audit-stamp test /Users/m16khb/Workspace/issueops.worktrees/128-cleanup-audit-stamp/internal/core/issueops
go -C /Users/m16khb/Workspace/issueops.worktrees/128-cleanup-audit-stamp test /Users/m16khb/Workspace/issueops.worktrees/128-cleanup-audit-stamp/...
```

## 비범위

- `list`에 원격 조회 추가. read-only 집계 표면의 성격을 바꾸고 오프라인에서 무용해진다.
- 필드 이름 변경. 캐시가 정확해지면 현재 이름이 의미와 일치한다.
