# 125 봉인된 이슈 편집 선제 차단 구현 계획

- 이슈: https://github.com/m16khb-org/issueops/issues/125
- 부모 백로그: https://github.com/m16khb-org/issueops/issues/99
- IssueOps: io-dc336927dc60 / direct / generation 1
- 브랜치: 125-sealed-issue-edit-guard (base main, base head 72e547b9ca09f1dce0c9bff73f4c5a9acc2788a1)
- 선행 조건: #123이 재봉인 경로를 만들어 차단 시 안내할 정식 복구 절차가 성립했다

## 문제 요약

훅의 원격 artifact 파서는 이슈 편집 명령을 이미 인식하고 제목·본문을 추출해 한국어 게이트에 넘긴다. 그러나 **편집 대상 식별자를 저장하지 않으므로** 그 이슈가 sealed owner context에 봉인되어 있는지 판정할 근거가 없다. 결과적으로 planner가 봉인 이슈를 수정해도 통과하고, digest drift는 owner가 claim할 때에야 발견된다.

## 설계: 계층을 지키는 최소 확장

### 파싱 계층 (remoteartifact)

- `remoteArtifactCommand`에 편집 대상 필드를 추가한다. 첫 비플래그 positional 인자를 대상으로 저장한다.
- 번호와 URL 두 형태를 정규화한다.
- exported 조회 함수로 대상을 노출한다. 이 계층은 상태 저장소를 알지 못한다.

### lifecycle 계층

- 새 가드가 활성 orca 사이클을 스캔한다. 기존 레코드 스캔 패턴을 재사용한다.
- 차단 조건 세 가지를 모두 만족해야 한다.
  1. 사이클이 orca 모드이고 lease가 활성 계열이다.
  2. 세대별 sealed packet 경로가 실제로 존재한다.
  3. 그 사이클의 연결 이슈가 편집 대상과 일치한다.
- 차단 메시지는 봉인 lifecycle ID와 재봉인 명령을 함께 안내한다.
- 원격 조회는 하지 않는다. 훅은 로컬 durable 레코드만 읽어 빠르고 결정적이어야 한다.

### 통과 규칙 (의도된 설계)

식별자를 해석하지 못하면 통과시킨다. 이 가드의 목적은 봉인 보호이며, 미지의 명령 형태를 fail-closed로 막으면 봉인과 무관한 일상 작업이 깨진다. 놓쳐도 기존 동작보다 나빠지지 않는다.

## RED 테스트

- 파싱: 번호 형태와 URL 형태의 편집 대상이 추출되는지
- 차단: 봉인된 이슈 편집이 차단되고 메시지에 lifecycle ID와 재봉인 명령이 들어가는지
- 오탐 금지: 무관한 이슈, packet 없는 direct 사이클, 종료된 사이클의 이슈 편집이 통과하는지
- 통과 규칙: 식별자를 해석할 수 없는 편집이 통과하는지

## 수용 기준 매핑

| AC | 검증 |
| --- | --- |
| AC-01 식별자 추출 | 파싱 테스트(번호·URL) |
| AC-02 차단 | 봉인 이슈 편집 차단 테스트 |
| AC-03 안내 | 차단 메시지 내용 테스트 |
| AC-04 오탐 금지 | 무관·direct·종료 사이클 통과 테스트 |
| AC-05 통과 규칙 | 미해석 편집 통과 테스트 |
| AC-06 RED 선행 | 각 테스트를 구현 전 실행해 실패 확인 |

## 검증 명령

```bash
go -C /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard test /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard/internal/core/remoteartifact
go -C /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard test /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard/internal/core/lifecycle
go -C /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard test /Users/m16khb/Workspace/issueops.worktrees/125-sealed-issue-edit-guard/...
```

## 비범위

- 이슈 편집 내용을 자동 재봉인하는 것. 개정 정당성은 사람의 판단이며 훅은 워크플로 작업을 수행하지 않는다.
- 봉인 이슈의 라벨·assignee 변경 차단. 본문 digest에 영향이 없다.
- 이슈 생성 경로 변경.
