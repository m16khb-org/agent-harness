# SQLite Store Maintenance Discovery Design

## 목적

`state maintain`은 현재 state root, issueops, worker 세 곳만 유지보수한다. 이후 추가된 loop store와 project-scoped `WithKeyLock` store는 같은 `sqlstore`를 사용하지만 이 목록 밖에 있어 WAL checkpoint와 권한 재보증을 받지 못한다.

이 변경은 이미 존재하는 loop 및 project SQLite store를 유지보수 대상에 포함한다. 새 store를 만들지 않고, project 디렉터리의 JSON/JSONL 파일을 수정하지 않으며, 기존 CLI/MCP DTO와 24시간 session-start 상각 주기를 보존한다.

## 관찰된 현재 상태

- `knownStoreRoots`는 state root, issueops, worker만 반환한다.
- loop는 `$ISSUEOPS_STATE_DIR/loop/issueops.db`에 독립 store를 둔다.
- kubectl live-access approval은 project namespace에서 `state.WithKeyLock`을 사용한다. 이 호출은 `$ISSUEOPS_STATE_DIR/projects/<repo-id>/issueops.db`를 생성하지만 approval record 자체는 JSON 파일이므로 `records` table은 비어 있을 수 있다.
- 실측된 16개 project store는 각각 8,272-byte WAL 2 frames, loop store는 16,512-byte WAL 4 frames였다. 모든 data/lock DB의 `integrity_check`는 `ok`였고 파일 권한은 `0600`이었다.

따라서 긴급한 손상 복구가 아니라, 새 store topology에 맞게 기존 유지보수 계약을 확장하는 작업이다.

## 접근 방식

### 채택: bounded discovery

고정 root 목록에 loop를 추가하고, `projects/`의 직계 하위 디렉터리만 조회한다. 각 project 디렉터리 안에 regular file인 `issueops.db`가 이미 있을 때만 유지보수 대상으로 추가한다.

이 방식은 현재 directory contract를 직접 반영하면서도 탐색 범위를 한 단계로 제한한다. `os.ReadDir`의 정렬 순서를 사용해 결과 순서를 결정적으로 유지한다.

### 거부: state tree 전체 재귀 탐색

미래의 임시·audit·benchmark 디렉터리까지 암묵적으로 포함하고 session-start 작업량을 디렉터리 깊이에 비례하게 만든다. `sqlstore` 소유 root라는 의미도 흐려진다.

### 거부: 별도 store registry

store 생성 시 registry를 갱신하면 discovery는 빨라지지만 registry 자체의 atomicity, migration, stale-entry 정리가 필요하다. 현재 24시간마다 한 번 실행하는 bounded scan에는 불필요한 두 번째 상태 계층이다.

## Discovery contract

유지보수 후보 순서는 다음과 같다.

1. state root
2. issueops
3. worker (`ISSUEOPS_WORKER_DIR` override 유지)
4. loop
5. `projects/<repo-id>` 중 existing `issueops.db` store, repo ID 오름차순

고정 root는 기존처럼 `issueops.db`가 없으면 `Skipped`에 보고한다. 따라서 loop가 아직 생성되지 않았다면 skipped root가 된다.

Project discovery에는 다음 제약을 적용한다.

- `projects/`가 없으면 project 후보는 0개이며 오류가 아니다.
- 직계 하위 real directory만 검사하고 symlink directory는 따라가지 않는다.
- `issueops.db`가 regular file일 때만 후보로 추가한다. DB가 없는 project namespace는 `Skipped`에 추가하지 않는다. 수천 개의 lifecycle-only namespace가 응답을 팽창시키지 않게 하기 위해서다.
- `projects/`가 존재하지만 읽을 수 없으면 성공으로 가장하지 않고 오류를 반환한다.
- 후보를 발견하는 과정에서는 `sqlstore.Open`을 호출하지 않는다. 유지보수 대상으로 확정된 existing DB만 기존 `StateMaintain` loop에서 연다.

## 유지보수 동작과 오류 처리

발견된 store에는 기존 `sqlstore.Maintain`을 그대로 적용한다.

- `PRAGMA wal_checkpoint(TRUNCATE)`로 WAL을 checkpoint한다.
- `issueops.db*`와 `issueops.lock.db*` 파일 권한을 `0600`으로 재보증한다.
- busy writer가 있으면 기존처럼 `Checkpointed=false`로 보고하고 전체 작업을 실패시키지 않는다.
- discovery 또는 store open/maintenance 오류는 기존 fail-fast 방식으로 반환한다.

행 삭제, `VACUUM`, project JSON/queue cleanup, empty project namespace 삭제는 범위 밖이다.

## 응답과 문서 계약

`StateMaintainResult` 필드는 변경하지 않는다.

- `Roots`에는 실제로 유지보수한 loop/project store가 추가된다.
- `Skipped`에는 missing loop가 추가될 수 있다.
- DB가 없는 project namespace는 `Skipped`에 포함하지 않는다.

MCP 설명의 "every existing harness store root"는 실제 동작과 다시 일치한다. ADR의 "4 known store roots" 결정은 고정 root와 bounded project discovery를 설명하도록 갱신한다. CLI/MCP schema에는 필드 추가나 golden shape 변경이 없어야 한다.

## 테스트 전략

TDD는 다음 순서로 진행한다.

1. loop와 두 project store를 materialize한 뒤 `StateMaintain` 결과에 세 root가 포함되고 WAL이 truncate되는 실패 테스트를 작성한다.
2. DB가 없는 project namespace와 symlink namespace가 store로 생성되거나 결과에 포함되지 않는 실패 테스트를 작성한다.
3. missing `projects/`는 정상이고, 읽을 수 없는 `projects/`는 오류인 discovery 테스트를 작성한다.
4. 최소 discovery 구현으로 RED 테스트를 GREEN으로 만든다.
5. 기존 state CLI/MCP 테스트의 loop skipped 기대값을 갱신하고 응답 계약 회귀를 확인한다.

검증 명령은 다음과 같다.

```bash
go test ./internal/core/state -run 'Maintain|Discover' -count=1
go test ./cmd/issueops/statecli ./cmd/issueops/mcpcli -run Maintain -count=1
go test -p 1 -timeout 20m ./... -count=1
go test -race -p 1 -timeout 20m ./... -count=1
go build -o bin/issueops ./cmd/issueops
```

## 완료 기준

- 새 테스트가 기존 구현에서 의도한 이유로 실패하고 변경 후 통과한다.
- 기존 DB가 있는 loop와 project store만 유지보수된다.
- 유지보수 실행이 DB 없는 project namespace에 SQLite 파일을 만들지 않는다.
- 기존 네 root의 결과 순서와 worker override 동작이 보존된다.
- CLI/MCP DTO, session-start 24시간 gate, busy checkpoint 의미가 유지된다.
- 전체 및 race 테스트와 build가 통과한다.
