# 127 orca dead adapter 처분 구현 계획

- 이슈: https://github.com/m16khb-org/issueops/issues/127
- 부모 백로그: https://github.com/m16khb-org/issueops/issues/99
- 재배선 후보: https://github.com/m16khb-org/issueops/issues/130
- IssueOps: io-762a0cfeecab / direct / generation 1
- 브랜치: 127-dead-adapter-disposition (base main, base head 4a89d5de10155ef8b058199e6e84c8dd47e9688d)

## 처분 결정: 보존

#78의 dead code 3분류(순수 미참조는 삭제, 재배선 가능은 배선, 커버리지 보유는 계량 후 판단)를 적용한 결과 **재배선 가능** 분류에 해당한다.

### 실측 근거

1. `execution complete`와 `cleanup finish` 모두 orca task를 건드리지 않는다. finish에 TaskID 참조가 0건이고 complete에 task 갱신이 0건이다.
2. 따라서 orca 모드 사이클은 task를 `dispatched` 상태로 남기고 끝난다.
3. #121의 완화는 `completed`와 `failed`만 소유자 요구에서 면제한다. `dispatched`는 여전히 소유자를 요구하므로 레코드 삭제 후 영구 `operational_task_residue`가 된다.
4. 이 두 심볼이 그 공백을 메울 유일한 구현이며, worker done 전송은 응답 정체 검증과 payload 일치 검사를 갖춘 완성된 코드다.

삭제하면 #130 수정 시 그 검증 로직을 재작성해야 한다. 따라서 보존한다.

## 구현: 근거 주석만 추가

동작 변경이 없다. 심볼 이름·시그니처·구현을 건드리지 않으므로 회귀 표면이 없다.

주석을 두 지점에 남긴다. 다음 dead code 스윕이 어느 쪽을 먼저 보더라도 판단 근거를 만나게 한다.

1. `internal/port/orca.go`의 worker done 클라이언트 인터페이스 선언
2. `internal/adapter/orca/client.go`의 두 구현(worker done 전송, task 갱신)

주석 내용은 세 가지를 담는다.

- **왜 배선이 없는가**: IssueOps v1 owner 명령 카탈로그가 orca 메시지 전송을 요구하지 않는다. owner의 완료 보고는 durable state와 원격 artifact가 담당한다.
- **어떤 조건에서 배선되는가**: orca 모드 사이클의 task 종결 경로가 필요할 때. 현재 그 공백이 실측으로 확인됐다.
- **재배선 후보 이슈**: #130. 그 이슈가 닫히면 이 주석도 갱신 대상이다.

## 수용 기준 매핑

| AC | 검증 |
| --- | --- |
| AC-01 처분 결정과 근거 | 이 계획 문서와 커밋 메시지, 이슈 코멘트 |
| AC-02 주석 추가 | 두 지점 주석 |
| AC-03 재배선 후보 명시 | 주석에 #130 참조 |
| AC-04 회귀 없음 | 전체 테스트와 contract golden |

## 검증 명령

```bash
go -C /Users/m16khb/Workspace/issueops.worktrees/127-dead-adapter-disposition test /Users/m16khb/Workspace/issueops.worktrees/127-dead-adapter-disposition/...
go -C /Users/m16khb/Workspace/issueops.worktrees/127-dead-adapter-disposition test /Users/m16khb/Workspace/issueops.worktrees/127-dead-adapter-disposition/cmd/issueops/contractgolden
```

RED 테스트는 없다. 동작 변경이 없고 요구가 판단 기록이므로 테스트로 실증할 결함이 존재하지 않는다. 이 사실을 정직하게 기록한다.

## 비범위

- 두 심볼 삭제. 재배선 필요성이 실측으로 확인됐다.
- orca dispatch 수명주기 종결 설계. #130의 범위다.
