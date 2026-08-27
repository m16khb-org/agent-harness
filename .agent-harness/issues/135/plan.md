# 135 — 읽기 전용 진단 표면의 관찰 자격

이슈: https://github.com/m16khb/agent-harness/issues/135
사이클: io-6e469edb59a0
브랜치: `135-readonly-diagnostic-guard` (base `main` @ bec61e0)

## 문제

`executionObservation`의 관찰 allowlist에 읽기 전용 진단 명령 셋이 빠져 있다.

| 명령 | `IssueOpsCommandSpec` | `executionObservation` |
|---|---|---|
| `list` | 있음 | **없음** → default로 떨어짐 |
| `cleanup status` | 있음 | **없음** → default로 떨어짐 |
| `pr-readiness` | **없음** | **없음** → 앞 단계에서 실패 |

owner가 lease를 쥔 채 자기 사이클의 준비 상태를 점검할 수 없다.

## 판정 기준

가드를 넓히는 변경이므로 기준을 코드에 남긴다.

> **관찰로 인정하는 것: core 구현이 상태를 쓰지 않고, 파괴 작업의 preview 단계도 아닌 것.**

`execution replace --preview`와 `execution reconcile --preview`가 이미 이 기준으로 허용돼 있다. 셋 다 core가 읽기 전용임을 확인했다 — `ListIssueOpsCycles`, `IssueOpsCleanupStatusByID`, `IssueOpsPRReadiness`.

`cleanup status --merged`가 원격을 조회하지만 그것도 읽기다. `cleanup remote-branch --preview`가 같은 자격으로 원격 OID를 관측하는 선례가 있다.

## 변경

1. `executionObservation`의 switch에 `list`, `cleanup status`, `pr-readiness`를 더한다.
2. `pr-readiness`를 `IssueOpsCommandSpec`에 등록한다 (`--id --strict --json`).
3. 판정 기준을 주석으로 남긴다.
4. `cleanup` 하위 파괴 명령 4종(`finish`/`abandon`/`remote-branch`/`orphan`)이 관찰로 인정되지 않음을 회귀 테스트로 고정한다.

경로 문자열이 prefix를 공유하므로 4번이 이 변경의 안전판이다.

## 비범위

- 새 읽기 전용 명령 추가.
- `executionObservation`을 allowlist가 아닌 구조로 재설계. 명시적 열거가 fail-closed의 근거이며, 다른 구조는 누락이 조용히 통과하는 방향으로 실패한다.

## 검증

```bash
go test ./internal/core/lifecycle/... -count=1
go test ./internal/core/commandparse/... -count=1
go test ./... -count=1
```

## 게이트 한계 기록

devil's advocate를 격리 서브에이전트로 띄우지 못했다(세션 정책상 sub-agent dispatch 금지).
