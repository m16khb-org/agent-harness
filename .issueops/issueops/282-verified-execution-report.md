# #282 검증 보고서 — native hook stable runtime

이슈: https://github.com/m16khb-org/issueops/issues/282

## 목표와 경계

Codex와 Claude native hook가 lifecycle worktree의 실행 파일을 영구 target으로
사용하지 않게 하고, 이미 시작된 host session이 완료된 worktree 실행 파일을
캐시했을 때 exact observed/expected 경로와 재시작 지침을 정상 hook 응답으로
전달한다. 설치 binary 갱신은 실행 중 inode를 제자리 덮어쓰지 않고 stable source
runtime에 staged-file fsync, atomic rename, directory fsync 순서로 활성화한다.

#261의 기존 regular command-file adoption과 PATH-link transaction은 이 변경에
포함하지 않는다. #282는 stable hook target authority, cached executable 진단,
atomic stable runtime activation만 소유한다.

## 구현 결과

- linked worktree의 `gitdir`와 `commondir`를 해석해 physical source checkout을
  canonical native root로 결정한다. relative metadata와 malformed metadata를
  각각 허용·fail-closed하도록 고정했다.
- Codex와 Claude installer가 기존 managed hook command의 observed target을 읽고
  exact stable target과 다르면 normal/dry-run 양쪽에서 동일한 drift warning을
  낸다. third-party hook group은 보존한다.
- strict activation readback은 `.worktrees` target을 exact observed/expected
  evidence와 함께 거부한다.
- PreToolUse와 SessionStart가 현재 executable을 기준으로 stale cached runtime을
  판별한다. host별 기존 response adapter를 사용하며 오류 상태로 사라지지 않는다.
- `scripts/install-native.sh`는 invoking checkout을 `BUILD_ROOT`, Git common-dir의
  source checkout을 stable `ROOT`로 분리한다. stable target 옆 임시 파일에
  build하고 file fsync 후 `os.replace`, directory fsync로 활성화한다.
- 새 runtime use case는 `internal/core/utility_facade.go`에 compatibility alias를
  추가하지 않고 hook CLI가 `internal/core/install`을 직접 사용한다.

## RED/GREEN 증거

각 작업은 production 변경 전에 resolver, drift diagnosis, strict readback,
cached executable, atomic activation 테스트를 먼저 실패시켰다. 최종 focused
GREEN은 다음 명령으로 고정했다.

```text
go test ./internal/core/install ./internal/adapter/installutil ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookcatalog ./cmd/issueops/installcli -count=1
```

## macOS native smoke

2026-08-03에 실제 macOS source runtime과 격리된 임시 HOME/CODEX_HOME/state를
사용해 worktree의 `scripts/install-native.sh --json --path-mode skip`을 실행했다.
사용자 host 설정은 변경하지 않았다.

```text
stable binary: /Users/m16khb/Workspace/issueops/bin/issueops
old inode: 180233675
new inode: 180715415
inode changed: true
old sha256: 92b7054f225191b058abcdf4477bc017adadf2bef54e1e82046e60203591d5db
new sha256: a0cb88b3593917d0ce09a9ab23ec64036d1e950311d88cb2d9e140d61db20385
install result: ok=true
Codex/Claude configured .worktrees targets: 0
Codex PreToolUse: exit 0
Codex PostToolUse: exit 0
Claude PreToolUse: exit 0
Claude PostToolUse: exit 0
```

Codex config와 Claude settings의 모든 installer-owned hook target은 stable source
binary를 가리켰다. 네 실제 hook payload 모두 JSON `{}`를 출력하고 status 0으로
종료해 `hook exited without a status code`가 재현되지 않았다. smoke 임시 HOME은
검증 후 휴지통으로 정리했다.

## fresh implementation review

첫 리뷰는 Critical 0건, Important 1건으로 `REVISE`였다. 구현과 unit/contract
검증은 통과했지만 issue 완료 조건의 old-inode/native-smoke 증거가 아직 보고서에
없다는 지적이었다. 위 실제 macOS native smoke로 inode 교체, stable target,
양 host PreToolUse/PostToolUse 정상 status를 증명했다. 추가로 clean-break 방향에
맞춰 새 `internal/core` facade alias를 제거했다.

재리뷰는 Critical/Important/Minor 모두 0건으로 `PASS`였다. reviewer는 모든
modified/untracked Go 파일의 진단, focused native/runtime tests, focused vet,
contract/response golden, cached-runtime race 반복을 통과시키고 현재 stable source
binary inode `180715415`와 SHA
`a0cb88b3593917d0ce09a9ab23ec64036d1e950311d88cb2d9e140d61db20385`가 위 smoke
증거와 일치함을 독립 확인했다.

## 검증

다음 검증을 변경 worktree에서 순차 실행해 모두 통과했다.

```text
git diff --check
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o /tmp/issueops-282 ./cmd/issueops
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
bash -n scripts/install-native.sh
```

첫 fresh reviewer의 full-suite 실행에서는 webfetch baseline comparator가 한 번
`signal: killed`로 종료됐고 exact test 재실행은 통과했다. branch-local 변경과
무관한 transient인지 최종 full verification에서 다시 확인했으며 같은 test와
전체 normal/race suite 모두 재발 없이 통과했다.

## 남은 publication 단계

fresh 재리뷰 PASS, IssueOps implementation review와 AI-slop-clean 기록, atomic
commit/push, PR CI, merge 후 stable activation 재확인, lifecycle cleanup을 수행한다.
