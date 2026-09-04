# #185 구현 계획 — cleanup 진단의 극성 통일

이슈: https://github.com/m16khb-org/issueops/issues/185

## 문제

`missing`은 충족되지 않은 요구의 목록이다. 그 안에 상태 차단을 그대로 적으면 극성이 뒤집혀 읽힌다.

`#181` 정리에서 실측했다. 원격 브랜치가 **존재하는** 상태에서:

```
cleanup status --merged  ->  missing: ["remote_branch_present"]
cleanup finish --preview ->  missing: ["remote_branch_absent"]
```

두 명령 모두 "원격 브랜치가 있어서 막혔다"를 말하고 있었지만 극성이 반대였다. `missing`에
`remote_branch_present`가 있으면 "원격 브랜치 존재라는 요구가 미충족" = 브랜치가 없다로 읽힌다.
실제 상태를 `git ls-remote`로 따로 확인해야 했다.

## 선례가 이미 요구 극성이다

- `#167`의 `switch-mode` 게이트: `worktree_clean`, `lease_holds_no_writer`, `orca_branch_name_free`
- `cleanup finish`: `worktree_clean`, `remote_branch_absent`

`cleanup status`와 `cleanup orphan`만 상태-차단 극성을 섞었다.

## 고칠 슬러그

| 파일 | 전 | 후 |
|---|---|---|
| `cleanupstatus/cleanup_status.go` | `remote_branch_present` | `remote_branch_absent` |
| 같은 파일 | `worktree_dirty` | `worktree_clean` |
| `orphancleanup/orphan_cleanup.go` | `worktree_dirty` | `worktree_clean` |
| 같은 파일 | `branch_mismatch` | `branch_match` |

`branch_match`는 `cleanup status`가 이미 쓰는 이름이다 — 두 표면이 같은 조건을 같은 이름으로
부르게 된다.

**건드리지 않는 것**: `execution_sync_base.go`의 `remote_branch_present`. 그것은 브랜치가 **있어야
한다**는 진짜 요구다(`plans/114`가 기록). `sync-base`와 `cleanup remote-branch`의
`RemoteBranchPresent` 결과 필드도 관측값이고 슬러그가 아니다.

## RED

기존 테스트 세 곳이 낡은 이름을 고정한다 — `cleanupstatus/cleanup_status_test.go`,
`issueops_cleanup_test.go`, `orphancleanup/orphan_cleanup_test.go`. 그것을 새 이름으로 바꾸면 RED가
된다.

그것만으로는 **이 이슈의 진짜 불변식**을 고정하지 못한다. 두 표면이 같은 물리 상태를 같은 슬러그로
부르는 것을 검사하는 테스트를 새로 만든다 — 원격 브랜치가 남은 하나의 레코드에 `cleanup status`와
`cleanup finish` 게이트를 모두 걸어 같은 슬러그가 나오는지 본다.

## GREEN

슬러그 문자열 네 곳을 바꾼다. `ready`/`ok` 판정은 그대로다 — 같은 조건에서 같은 개수의 `missing`이
나오고 이름만 바뀐다.

## 문서

`CONVENTIONS.md`의 Guard 컨벤션에 극성 축을 명문화한다. `#154`가 세운 "관측 불가와 조건 위반을 다른
슬러그로 구분한다"와 직교하는 축이다.

## 검증

```
go test ./internal/core/issueops/... -count=1
go test ./... -count=1
```

`sync-base`의 동명 슬러그가 그대로인지 전체 테스트로 확인한다.

## 비범위

- 게이트 추가·제거. 무엇이 cleanup을 막아야 하는지는 바꾸지 않는다.
- `worktree_git_status`의 이름. 관측 실패 축이고 극성 축과 섞으면 두 변경이 한 커밋에 들어간다.
- `cleanup status`와 `cleanup finish`를 합치는 것. 전자는 read-only 진단, 후자는 mutation 경계다.
