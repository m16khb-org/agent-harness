# #185 검증 보고서 — cleanup 진단의 극성 통일

이슈: https://github.com/m16khb-org/issueops/issues/185

## 무엇이 문제였나

`missing`은 충족되지 않은 요구의 목록이다. 그 안에 상태 차단을 그대로 적으면 극성이 뒤집혀 읽힌다.

`#181` 정리에서 실측했다. 원격 브랜치가 **존재하는** 상태에서:

```
cleanup status --merged  ->  missing: ["remote_branch_present"]
cleanup finish --preview ->  missing: ["remote_branch_absent"]
```

둘 다 "원격 브랜치가 있어서 막혔다"를 말하고 있었지만, `missing` 안의 `remote_branch_present`는
"원격 브랜치 존재라는 요구가 미충족" = 브랜치가 없다로 읽힌다. `git ls-remote`로 따로 확인해야
했다.

`cleanup`은 워크트리·브랜치·레코드를 지우는 비가역 단계다. 그 직전 진단이 상태를 반대로 읽히게 하면
운영자는 `cleanup remote-branch`를 건너뛰고, `cleanup finish`가 막히고, 다시 추측한다.

## 요구 극성이 다수이고 선례다

- `#167`의 `switch-mode` 게이트: `worktree_clean`, `lease_holds_no_writer`, `orca_branch_name_free`
- `cleanup finish`: `worktree_clean`, `remote_branch_absent`

`cleanup status`와 `cleanup orphan`만 상태-차단 극성을 섞었다.

## 바꾼 슬러그

| 파일 | 전 | 후 |
|---|---|---|
| `cleanupstatus/cleanup_status.go` | `remote_branch_present` | `remote_branch_absent` |
| 같은 파일 | `worktree_dirty` | `worktree_clean` |
| `orphancleanup/orphan_cleanup.go` | `worktree_dirty` | `worktree_clean` |
| 같은 파일 | `branch_mismatch` | `branch_match` |

`branch_match`는 `cleanup status`가 이미 쓰는 이름이다 — 두 표면이 같은 조건을 같은 이름으로 부르게
됐다.

**건드리지 않은 것**: `execution sync-base`의 `remote_branch_present`. 그것은 브랜치가 **있어야**
한다는 진짜 요구다(`plans/114`가 기록). `sync-base`와 `cleanup remote-branch`의
`RemoteBranchPresent` 결과 필드도 관측값이고 슬러그가 아니다. 같은 문자열이 표면에 따라 요구일 수도
차단일 수도 있다는 것이 이 이슈의 미묘한 지점이다.

`worktree_git_status`는 관측 실패 축이라 그대로 뒀다 — `#154`가 세운 구분이며 극성 축과 직교한다.

## 테스트가 고정하는 것

이름의 의미는 문자열로 판정할 수 없다. 그래서 **두 표면이 같은 물리 상태를 같은 슬러그로 부른다**를
고정했다 — 그것이 이 이슈의 실제 불변식이다.

`TestRemoteBranchSurvivalBlocksBothCleanupSurfacesWithTheSameSlug`가 원격 소스 브랜치가 남은
머지된 사이클을 실제 Git 저장소로 만들어 `cleanup status`를 돌리고, 같은 상태를 fake로 재현해
`CleanupFinish` 게이트를 돌린 뒤 두 `missing`이 공유하는 원격 브랜치 슬러그를 확인한다. 공유가
없으면 실패한다.

RED가 정확히 그 모순을 출력했다:

```
--- FAIL: TestRemoteBranchSurvivalBlocksBothCleanupSurfacesWithTheSameSlug
    두 표면이 원격 브랜치 잔존을 다른 슬러그로 부른다:
    status=[remote_branch_present]
    finish=[remote_branch_absent]
```

## 폐기한 첫 시도

처음에 `cleanupStatusRemoteBranchSurvivalSlug()`라는 헬퍼를 두고 그것이 상수를 돌려주는 형태로 썼다.
**순환 검사였다** — 테스트가 검사한다고 주장하는 값을 테스트가 직접 적는 것이다. 실제 저장소 픽스처로
`cleanup status`를 돌리는 형태로 바꿨다.

## 검증

`go test ./... -count=1` 전 패키지 PASS. `sync-base`의 동명 슬러그가 그대로인지 전체 테스트로
확인했다 — `execution_sync_base_test.go`가 `remote_branch_present`를 계속 고정하며 통과한다.

`ready`/`ok` 판정은 바뀌지 않았다. 같은 조건에서 같은 개수의 `missing`이 나오고 이름만 바뀐다.

## 문서

`CONVENTIONS.md`의 Guard 컨벤션에 극성 축을 명문화했다. `#154`가 세운 관측/조건 축과 직교하며, 같은
이름이 표면에 따라 요구일 수 있으므로 표면별로 먼저 정해야 한다는 것을 함께 적었다.

## 남는 것

- **극성 규칙을 기계적으로 강제할 수 없다.** 슬러그 이름의 의미는 문자열로 판정되지 않는다. 이번
  테스트는 두 표면의 *불일치*만 잡는다. 새 표면이 혼자 잘못된 극성을 쓰면 잡히지 않는다.
- **`worktree_git_status`의 이름.** 관측 실패인데 요구 미충족처럼 읽힌다. 극성 축과 섞으면 두 변경이
  한 커밋에 들어가므로 남겼다.
