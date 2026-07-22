# IssueOps Handoff Intent Preservation and Coordinator Liveness

## 배경

`handoff owner에게 실행을 위임한다`는 사용자 목표가 다음 행동 선택지에서
`격리된 구현 worktree에서 실행한다`로 축약된 뒤, 숫자 응답 `1`이 그 선택지
문구만 원문 요청으로 승격했다. 그 결과 ownership transfer가 Git worktree
isolation으로 의미 축소됐고 source/main 세션이 구현을 직접 시작했다.

이 실패는 handoff engine의 dispatch 오류가 아니다. 선택지 작성과
numeric-choice relay 사이에서 실행 주체와 ownership mode가 손실된 뒤,
downstream이 손실된 입력을 정상 실행한 semantic downgrade다.

동시에 ownership handoff 직후 source/main 세션이 owner 진행을 동기적으로
기다리거나 terminal output을 무기한 tail하면 coordinator가 먹통처럼 보이는
liveness 회귀가 생길 수 있다. 기존 #65는 owner orientation, heartbeat,
fresh-shell authority, Stop 재진입 문제를 추적하며 coordinator가 계속
관측·조정 가능한 상태로 남아야 함을 보여준다.

## 확인된 원인

1. 선택지 문구가 실행 계약 전체가 아니라 사람이 읽는 축약 label이었다.
2. `StopNextActionRelayCandidate`는 `Index`, `Recommended`, `Text`만 저장한다.
3. `karpathyChoiceContextLine`은 선택지 text를 기존 목표에 대한 delta가 아니라
   `원문 요청`으로 승격한다.
4. 선택지 품질 gate는 형식, 언어, 중복, 파괴성만 검사하며 실행 주체,
   workspace, ownership transition의 보존 여부는 검사하지 않는다.
5. Git worktree/plan 실행 경로는 손실된 계약에 따라 main-agent direct execution을
   선택했다. 따라서 해당 경로는 원인이 아니라 semantic downgrade의 결과다.

## 선택한 최적해

두 층을 분리한다.

### 1. 선택지는 기존 목표를 대체하지 않는 next-action delta다

- 숫자 선택 복원 문구에서 `이 선택지를 원문 요청으로 삼아`를 제거한다.
- 선택된 text는 현재 대화에서 이미 확정된 사용자 목표, 금지사항, 실행 주체,
  workspace와 lifecycle mode를 유지한 채 적용하는 다음 행동으로 주입한다.
- 선택지 생성 계약은 실행 방식이 갈리는 경우 각 후보에 다음 항목을
  self-contained하게 명시하도록 한다.
  - 실행 주체: source/main coordinator, handoff owner, 또는 사용자
  - 실행 위치: source checkout, canonical worker root, 또는 외부 시스템
  - ownership 전환: 유지, 시작, 회수, 종료
  - 외부 mutation 경계: issue, push, PR/MR, merge, cleanup
- 선택지 label만으로 기존 사용자 목표를 축소하거나 덮어쓸 수 없다.

`action_kind`, `executor`, `workspace_mode`를 모든 일반 선택지에 추가하는 typed
relay schema는 지금 도입하지 않는다. Stop hook은 자유형 대화의 action type을
결정적으로 추론할 수 없고, 별도 machine-readable 선택지 protocol은 모든 host와
기존 choice parser를 확장한다. 현재 host는 원 대화 context를 이미 보존하므로
replacement 문구를 제거하고 delta 불변식을 강제하는 것이 더 작고 직접적인
수정이다. Durable IssueOps state가 존재하는 단계에서는 그 typed state가 항상
자유형 선택지보다 우선한다.

### 2. Handoff 이후 source/main은 non-blocking coordinator다

- `handoff start`는 dispatch receipt와 durable handoff identity를 반환한 뒤
  source turn을 해제한다. owner 완료까지 동기 실행하지 않는다.
- source/main은 worker 파일, owner shell, raw terminal input을 변경하지 않는다.
- source/main 관측은 `issueops status/resume`, bounded Orca terminal read/wait,
  heartbeat와 mailbox receipt로 제한한다.
- 한 번의 wait/poll은 30초 이내로 제한하고, 진행 중에는 60초 안에 사용자에게
  상태를 갱신한다. 무기한 terminal tail, 무기한 orchestration receive, blocking
  shell wait는 금지한다.
- owner claim/acknowledge/heartbeat가 제한 시간 안에 없으면 source가 구현을
  대신하지 않는다. durable state와 exact terminal/task/dispatch identity를
  읽고 recovery 또는 재-handoff 판단 지점으로 전환한다.
- owner가 `owner_active`가 된 뒤 구현, 테스트, commit, publish와 PR/MR lifecycle
  mutation은 exact owner session과 canonical worker root만 수행한다.
- source/main은 수정 요청 전달, 상태 관측, 사용자 decision relay만 수행한다.

## 이번 실행 계약

기존 bounded modification-request 구현을 먼저 완료한다.

1. 현재 detached worktree의 검증된 Task 1 변경을 atomic commit으로 보존한다.
2. 한글 GitHub issue를 만들고 IssueOps intent/design/plan/compatibility/execution
   decision을 기록한다.
3. issue-number branch와 canonical Orca worktree를 정식 prepare한다.
4. ownership handoff context에 기존 Task 1 commit SHA와 승인된 spec/plan을 넣는다.
5. owner는 identity를 확인한 뒤 Task 1 commit을 cherry-pick하고 계획의 남은
   task를 TDD로 완료한다.
6. source/main은 bounded status poll만 수행하며 owner 작업을 중복 실행하지 않는다.
7. push, PR 생성, merge, cleanup은 각각 기존 IssueOps 권한 경계를 유지한다.

## 회귀 검증

- 원 목표에 `handoff owner`가 있고 선택지 text에 `격리된 worktree`만 있어도,
  숫자 선택 주입은 원 목표를 대체하지 않고 실행 주체 보존을 요구한다.
- choice expansion context에 `원문 요청으로 삼아`가 존재하지 않는다.
- 선택지 품질 안내는 실행 주체, 작업 위치, ownership 전환을 요구한다.
- active IssueOps handoff의 durable owner/workspace state가 자유형 선택지보다
  우선한다.
- handoff start는 owner 완료를 기다리지 않고 bounded receipt를 반환한다.
- coordinator monitoring은 timeout 후 제어권을 돌려주며 source/main에서 구현
  mutation을 만들지 않는다.
- 기존 owner orientation, heartbeat, completed-owner, cleanup Stop 회귀 테스트가
  유지된다.

## 추적

- #65: handoff liveness, owner orientation, fresh-shell authority와 bounded Stop
  재진입의 body-of-record
- `docs/superpowers/specs/2026-07-22-issueops-handoff-modification-request-design.md`:
  기존 구현 설계
- `docs/superpowers/plans/2026-07-22-issueops-handoff-modification-request.md`:
  기존 구현 계획
