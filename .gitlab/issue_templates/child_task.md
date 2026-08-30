## 부모 이슈

parent issue URL 또는 번호를 적어 주세요.

## 실행 분류

제목은 `[p]` 또는 `[s]`로 시작해야 합니다. 기본은 `[p] parallelizable`이며,
`[s] sequential`은 이름 있는 hard dependency가 있을 때만 사용합니다.

- 실행 클래스: `[p] parallelizable` 또는 `[s] sequential`
- 선행 조건: `[p]`는 `none`, `[s]`는 필요한 child URL/산출물
- 실행 wave: 숫자

좋은 예: `[p] renderer 계약 검증`, 선행 조건 `none`, wave `1`.
나쁜 예: `task: renderer 수정`, 선행 조건과 검증이 없음.

## 작업 목표

이 child task가 독립적으로 끝내야 할 목표를 적어 주세요.

## 완료 기준

parent와 분리해 검증 가능한 완료 기준을 적어 주세요.

## 비목표

이 child task가 맡지 않을 범위를 적어 주세요.

## 검증

실행할 테스트, 빌드, 스모크 체크, 문서 검증을 적어 주세요.

## 부모 브랜치 병합 조건

child branch가 parent branch에 병합되기 전 필요한 검증과 리뷰 조건을 적어 주세요.

## child-only cleanup 규칙

child 완료 후 닫을 artifact와 parent에 남길 후속 상태를 적어 주세요.

<!-- #<parent>를 실제 부모 이슈 번호 또는 URL로 바꿔 저장하세요. -->
/set_parent #<parent>
/label ~"enhancement"
