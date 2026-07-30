# IssueOps 정리된 자식 사후 승인 설계

## 문제

부모 IssueOps record는 자식 cycle의 인덱스를 보존하지만, 자식 record는
`cleanup finish`에서 삭제된다. 부모가 자식을 승인하기 전에 정리가 끝나면
`issueops child status`는 이를 orphan으로 표시하면서도 `issueops child accept`는
삭제된 자식 record를 먼저 읽다가 실패한다. 따라서 검증된 원격 병합 증거가
있어도 부모 인덱스를 정상 상태로 복구할 수 없다.

## 결정

`child accept`만 삭제된 자식 record를 위한 제한된 사후 승인 경로를 제공한다.

- 자식 record가 존재하면 기존처럼 `phase=done`과 부모 연결을 검증한다.
- 자식 record가 `fs.ErrNotExist`인 경우 부모의 `ChildCycles`에 같은 cycle ID가
  이미 있어야 한다.
- 사후 승인에도 하나 이상의 명시적 validation evidence가 필요하다.
- 승인 결과는 기존과 같은 `accepted` verdict, evidence, `validated_at` 영수증을
  부모 ref에 기록한다.
- 부모에 없는 ID, 잘못된 ID, 존재하지만 done이 아닌 record, 다른 오류는
  기존처럼 fail-closed한다.
- `reject`와 `drop`은 정리된 결과를 재분류하는 동작이 아니므로 변경하지 않는다.

## 안전성

사후 승인 경로는 삭제된 자식의 완료 상태를 추정하지 않는다. 부모가 이미
소유한 ref와 운영자가 제출한 검증 증거를 부모 소유의 최종 validation
receipt로 바꾸는 동작만 수행한다. 새 자식 ref를 만들지 않으며 IssueOps 자식
record를 복원하지 않는다.

## 검증

실제 state store와 기존 child API를 사용하는 회귀 테스트로 다음을 고정한다.

1. done 자식 record가 정리된 뒤에도, 부모 ref와 evidence가 있으면 승인된다.
2. record와 부모 ref가 모두 없는 cycle ID는 evidence가 있어도 거부된다.
3. 기존의 non-done 거부와 accepted cleanup receipt 테스트가 계속 통과한다.
