# 130 — orca 모드 사이클의 task 종결 경로

이슈: https://github.com/m16khb/agent-harness/issues/130
사이클: io-26d62ec1a697
브랜치: `130-orca-task-settlement` (base `main` @ 404fb20)

## 문제

orca 모드 사이클이 orca task를 `dispatched`로 남기고 끝난다. 레코드가 삭제되면 그 task는
소유자 조회가 영구히 0건이 되어 `operational_task_residue`로 계속 보고된다.

agent-harness에는 task를 종결시키는 경로가 아예 없다. `UpdateTask`와 `SendWorkerDone`은
adapter에만 있고 core 배선이 없으며, #127이 그 보존 근거로 이 이슈를 지목했다.

#121은 **이미 종결된** task가 영구 residue가 되는 문제를 고쳤다. 남은 것은 **애초에
종결되지 않는** 문제다.

## 종결 시점: `execution complete`

세 후보 중 A를 택한다. 근거는 decision ledger에 있다. 요약:

- task는 자원이 아니라 오케스트레이션 이력이다(#121이 확립한 판단). 따라서 종결은 이력
  확정이고, 그 시점은 자원을 거두는 `cleanup finish`가 아니라 owner의 마지막 durable
  행위인 `execution complete`다.
- `cleanup finish`에 두면 complete~finish 구간(사용자 머지 대기 포함)에서 orca가 task를
  진행 중으로 오인한다.
- `SendWorkerDone` 배선은 owner 명령 카탈로그 확장을 요구해 "orca는 두 번째 workflow
  authority가 아니다"라는 계약과 충돌한다.
- 두 곳에 두는 안도 기각한다. AC-04가 요구하는 것은 시점의 명확성이고 중복은 그것을 흐린다.

## 설계

`CompleteExecution`에 외부 호출 주입점을 더한다.

```go
type ExecutionCompleteDeps struct {
    // SettleOrcaTask는 orca task를 terminal 상태로 옮긴다. nil이면 종결을
    // 건너뛴다(direct 모드 전용 호출자와 테스트).
    SettleOrcaTask func(ctx context.Context, taskID string) error
}
```

호출 순서가 계약의 핵심이다.

1. 상태 전이는 종전대로 **락 안에서 원자적으로** 커밋한다.
2. 그 뒤 **락 밖에서** orca task를 종결한다. orca 모드이고 `TaskID`가 있을 때만.
3. 실패는 완료를 되돌리지 않고 결과 필드로 표면화한다.

`cleanup remote-branch`의 `ReflectAudit` + `AuditError`가 같은 패턴의 선례다.

## 비범위

- `cleanup abandon` 경로. complete 없이 레코드를 지우므로 task가 `dispatched`로 남지만,
  AC-01이 정상 완료로 한정하며 abandon에 orca mutation을 더하는 것은 그 명령의 조회 전용
  계약을 바꾸는 별도 결정이다. 발견 사항으로 기록한다.
- `dispatched`를 residue 분류에서 면제하는 것. 실행 중일 수 있는 task를 무시하면 실제
  잔여물 검출력을 잃는다.

## 검증

```bash
go test ./internal/core/issueops/... -count=1
go test ./internal/core/operationalhealth/... -count=1
go test ./cmd/harness/issueopscli/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
메인 에이전트가 직접 반론을 수행했으므로 저자와 검토자가 분리되지 않았다.

활성 orca 사이클이 0건이므로 실환경 도그푸드는 불가능하다. fake orca 클라이언트로
계약을 고정한다.
