# IssueOps child 도움말 actor 플래그 parity 설계

## 배경

IssueOps child 명령의 실제 parser는 `start`, `status`, `list`, `accept`, `reject`,
`drop` 모두 `addIssueOpsActorFlags`를 호출한다. 따라서 active execution에서 durable
record mutation을 수행하려면 `--host`, `--session-id`, 선택적 `--agent-id`, `--cwd`를
전달할 수 있다.

반면 child 전용 help는
`cmd/harness/issueopscli/issueops_cli_support.go`의 별도 상수 문자열이다. 이 문자열은
각 명령 줄의 `RECORD_ACTOR_FLAGS`와 그 축약 정의를 빠뜨린다. canonical catalog와 parser는
올바르지만, 사용자가 가장 가까운 `issueops child --help` 표면에서 계약을 발견할 수 없다.

선행 #188은 IssueOps 전체 usage 문장을
`internal/adapter/cli/issueops_catalog.go`의 단일 catalog로 통합했다. 이 결함은 그 뒤에도 남은
child 전용 부분 문자열 중복이다.

## 목표

- child 전용 help의 여섯 명령 줄을 canonical catalog에서 투영한다.
- 각 줄의 `RECORD_ACTOR_FLAGS`와 공용 `IssueOpsActorFlagLegend`를 함께 노출한다.
- parser와 help 사이의 actor 플래그 parity를 focused test로 고정한다.
- 기존 최상위 usage, lifecycle 상태 전이, cleanup 의미를 그대로 보존한다.

## 비목표

- flag 등록 정보로 모든 usage 문법을 자동 생성하지 않는다.
- actor 권한이나 write lease 검증 규칙을 변경하지 않는다.
- child accept의 indexed parent reference 복구 동작을 변경하지 않는다.
- `branch`, `execution` 등 다른 전용 help 표면을 함께 정리하지 않는다.

## 제안 설계

### 1. child help는 catalog 줄의 투영이다

`cmd/harness/issueopscli`에 child 명령 경로 여섯 개만 담은 key 집합을 둔다.

```text
child start
child status
child list
child accept
child reject
child drop
```

`cliadapter.IssueOpsUsageLines()`를 순회하고
`cliadapter.IssueOpsUsageKey(line)`가 이 집합에 포함된 줄만 catalog 순서로 선택한다. 이렇게
하면 usage 문장과 플래그 표기는 catalog 한 곳에만 존재한다. `link-child`는 key가
`link-child`이므로 선택되지 않는다.

이 projection은 `cmd/harness/issueopscli` 내부 함수로 유지한다. 다른 소비자가 없으므로
`internal/adapter/cli`에 새 공개 abstraction을 추가하지 않는다.

### 2. actor 범례도 공용 원본을 쓴다

선택한 명령 줄 아래에 `cliadapter.IssueOpsActorFlagLegend`를 붙인다. 별도 child 전용 범례를
만들지 않는다. 따라서 actor 축약 정의가 바뀌면 전체 help와 child help가 같은 내용을
렌더한다.

### 3. child 진입점은 매번 projection을 렌더한다

`runIssueOpsChild`의 help 분기는 기존 상수를 출력하는 대신 projection 함수를 호출한다.
catalog가 정적이므로 비용은 무시할 수 있고, 초기화 시 복사한 문자열보다 데이터 흐름이
명확하다.

```text
canonical usage catalog
        │
        ├─ 전체 issueops help
        └─ child key projection + 공용 actor legend
                         │
                         └─ issueops child --help
```

## 테스트 설계

### RED

현재 코드에서 child help를 호출하고 다음을 요구하는 focused test를 먼저 추가한다.

- child 여섯 줄이 canonical catalog의 대응 줄과 정확히 같다.
- 각 줄에 `RECORD_ACTOR_FLAGS`가 있다.
- 공용 actor 범례가 출력에 있다.
- `link-child`가 출력에 없다.

기존 상수는 actor 플래그와 범례가 없으므로 테스트가 실패해야 한다.

### GREEN

상수를 catalog projection 함수로 바꾼 뒤 같은 테스트를 통과시킨다.

### 회귀 검증

```bash
go test ./cmd/harness/issueopscli/... ./internal/adapter/cli/... -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness issueops child --help
```

로컬 전체 테스트와 전체 race는 실행하지 않는다. 범위를 벗어난 package 회귀는 원격 CI로
확인한다.

## 호환성과 rollback

- 명령 parser, JSON schema, persisted state는 변하지 않는다.
- 사용 가능한 플래그가 추가되는 것이 아니라 이미 가능한 플래그가 help에 드러난다.
- 기존 child help의 명령 순서는 canonical catalog 순서와 같아 유지된다.
- 문제가 생기면 이 단일 커밋을 revert하면 이전 help 문자열로 되돌아간다.

## 대안 검토

### 별도 상수에 actor 플래그를 수동 추가

가장 작은 diff지만 usage 문장을 두 곳에 유지해 #188이 제거한 drift 원인을 되살린다. 채택하지
않는다.

### parser flag 등록에서 usage를 자동 생성

선택 플래그, 반복 플래그, 배타 조합을 사람이 읽기 좋은 한 줄 문법으로 복원해야 한다. 이번
결함에 비해 범위와 위험이 크므로 채택하지 않는다.

### adapter package에 범용 projection API 공개

현재 추가 소비자는 child help 하나뿐이다. 단일 사용을 위해 공개 API를 늘릴 필요가 없으므로
채택하지 않는다.

## 자체 검토

- 미정 placeholder나 추후 결정 항목이 없다.
- #188의 catalog 단일 원본 결정과 모순되지 않는다.
- parser, lifecycle, cleanup을 비목표로 명시해 scope가 닫혀 있다.
- exact child key와 `link-child` 제외 조건으로 선택 경계가 모호하지 않다.
- success, 회귀, rollback 증거가 명령 단위로 정의돼 있다.
