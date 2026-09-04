---
name: cautions/runtime.md
description: Cautions for process, daemon, worker, lock, state store, and install/temp-artifact hygiene.
---

# Runtime, daemon, worker, lock, and state cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for process,
daemon, worker, lock, SQLite state store, and build/install temp-artifact
hygiene. Dated incident lessons live under [lessons/](lessons/).

## 5. Worker lifecycle 문제

현재 worker는 state-first one-shot job record와 policy-gated `run --read-only`만 제공한다. 장기 상주 worker를 추가하면 stale lock, orphan process, socket 권한, 오래된 binary 문제가 생긴다.

주의:
- 현재 daemon은 shared MCP backend이지 background job runner가 아니다.
- persistent worker를 도입하기 전에 health/version handshake, graceful shutdown, stale lock cleanup, timeout/cancellation을 고정한다.
- socket path와 permission을 문서화하고 테스트한다.

## 6. State 위치 혼동

프로젝트 지식과 런타임 state가 섞이면 repo가 오염되고 secret이 커밋될 수 있다.

주의:
- 추적할 지식은 `.issueops/`에 둔다.
- cache/log/runtime state는 user state dir 또는 ignored `.issueops-runtime/`에 둔다.
- `.issueops-runtime/`를 도입하면 `.gitignore`에 추가한다.
- **S2 (current boundary)**: state, IssueOps, worker, loop store는 SQLite `issueops.db`를 사용하며, root별 `issueops.lock.db`에서 `BEGIN IMMEDIATE` span으로 직렬화한다. 한 store root의 multi-record invariant는 하나의 `WithSpan`/transaction 안에서 갱신하며, 서로 다른 store root 사이의 cross-database atomicity는 가정하지 않는다.

## 11. 과도한 초기 추상화

처음부터 remote server, distributed queue, plugin marketplace packaging을 만들면 개인 하네스 MVP가 늦어진다.

주의:
- 1단계는 `issueops inspect`와 state/checkpoint 같은 작은 기능으로 시작한다.
- 반복 사용으로 필요가 확인된 기능만 worker/plugin layer로 승격한다.

## 13. Daemon lifecycle drift

`issueops mcp`가 daemon을 자동 시작하므로 오래된 binary가 이미 떠 있으면 새 코드 검증과 실제 MCP 동작이 갈라질 수 있다. `issueops update`와 `issueops bootstrap`은 실행 중인 daemon을 post-install 단계에서 재시작하지만, 수동 `go build`나 `install-native`만 실행한 경우에는 daemon이 그대로 남을 수 있다.

주의:
- 수동 설치/빌드 후 MCP smoke 전에는 필요하면 `issueops daemon stop --json`으로 기존 daemon을 내린다.
- 테스트는 `ISSUEOPS_DAEMON_DIR=$(mktemp -d)/daemon`으로 실제 user daemon과 분리한다.
- macOS actual-socket QA는 Unix-domain socket 경로 길이 제한을 피하도록 `/tmp/ahd-*`처럼 짧은 임시 root를 사용한다. 기본 `t.TempDir()`의 긴 `/var/folders/...` 경로는 구현과 무관한 `bind: invalid argument`를 만들 수 있다.
- QA launcher가 daemon child의 parent라면 `daemon stop`과 parent의 `Wait`를 동시에 진행해 SIGTERM 종료 자식을 즉시 reap한다. unreaped zombie는 `kill(pid, 0)`에 살아 있는 것으로 보여 fail-closed forced-stop 검증을 오탐할 수 있다. 모든 QA는 `defer`/`finally` 정리 후 임시 binary, state root, PID, socket이 0개인지 확인한다.
- daemon socket/pid/log는 user state dir에 두고 repo나 wiki vault에 쓰지 않는다.
- accept 루프를 검증하려고 연결을 몰아서 여는 테스트는 대기 중인 연결이 커널 백로그 한도(`kern.ipc.somaxconn`, macOS 기본 128)를 넘지 않게 dial과 수용 확인을 번갈아 수행한다. `maxConnections`(256)만큼 먼저 dial하면 129번째 connect가 `ECONNREFUSED`로 거절되어 부하에 따라 흔들린다 ([2026-08-27 lesson](lessons/2026-08-27-daemon-accept-loop-burst-dial-backlog.md)).
- **D2 (NFS caveat, accepted)**: daemon single-instance locking은 `daemonlock/lock.go`의 `O_EXCL` create + stale(30s)/PID-liveness 감지로 막는다. lock 파일은 startup handoff 후 child가 삭제하므로(transient) flock fallback은 부적합하다(flock은 inode에 묶여 삭제 시 깨짐). `O_EXCL`은 NFS/FUSE에서 원자성이 보장되지 않으니 **daemon state는 로컬 FS에 둔다**; 네트워크 마운트 home에서는 이론상 두 daemon이 뜰 수 있으나 두 번째는 동일 unix socket bind에서 실패한다.

## 20. /tmp/issueops-* build artifact cleanup

Manual builds, smoke tests, and ad-hoc verification runs can leave stale binaries and log files under `/tmp/issueops-*`. Self-verify temp directories are properly cleaned (`t.TempDir()`), but one-off commands like `go build -o /tmp/issueops-test ./cmd/issueops` and output captures (`... >/tmp/issueops-*.txt`) are manual artifacts that accumulate.

주의:
- Harness Go code never writes to `/tmp/issueops-*` — these are always manual developer artifacts.
- Self-verify temp directories (`/tmp/issueops-self-verify-*`, `/tmp/ahd-*`) are cleaned on normal completion paths, but SIGKILL or host crash can still leave them behind. No automatic `/tmp` reaper is implemented; cleanup remains explicit hygiene.
- To clean up stale manual artifacts: `rm -f /tmp/issueops-*`. Add this to a periodic workspace hygiene routine.
- CI and automated scripts should prefer `mktemp -d` or Go `t.TempDir()` / `os.MkdirTemp` over hardcoded `/tmp/` paths.
- Build scripts (`scripts/install-native.sh`) build to `$ROOT/bin/issueops`, not `/tmp`.
- 기존 `~/.local/bin/issueops` regular file을 symlink처럼 일반 교체하지 않는다. `--adopt-command-file` 승인 뒤에도 두 binary의 정적 Go build identity, current euid·executable·single-link·size 경계를 모두 먼저 확인하고, Seal 전 실패는 private backup에서 복원한 뒤 같은 transition ID만 Abort한다. Seal 후 backup cleanup 실패는 committed activation을 rollback하지 않는다.
- native install wrapper는 실제 실행과 같은 인자의 dry-run path preflight를 activation Begin보다 먼저 끝낸다. 승인되지 않은 regular command path가 Begin 뒤에 거부되면 기존 sealed receipt를 잃고 불필요한 pending transition을 남기므로, 실패 fixture는 호출 순서가 `preflight` 하나뿐이고 prior receipt와 pending 부재가 그대로인지 검증한다.
- staged install의 managed-path preflight는 eventual canonical symlink target이 아니라 현재 실행 중인 `.issueops.activate-*` candidate의 build/file identity를 검사한다. command path 교체와 rollback은 atomic exchange로 displaced 객체를 먼저 보존한 뒤 승인된 identity 또는 exact symlink인지 확인하며, 경합이 감지되면 동시 교체 파일을 canonical path에 복원하고 private backup을 recovery evidence로 유지한다.
- macOS 기본 Bash 3.2에서 `set -u`와 빈 array의 `"${values[@]}"` 조합은 `unbound variable`로 종료된다. optional install/activation args는 길이를 먼저 확인하고 빈 경우 array expansion 자체를 생략하며, `bash -n`만으로는 잡히지 않으므로 zero-flag wrapper runtime fixture를 둔다.

## 26. SQLite WAL 고수위 및 사이드카 권한

SQLite 전환 후 checkpoint 뒤에도 WAL이 truncate되지 않고 고수위로 유지되는 문제(M1)와 사이드카가 0600이 아닌 권한으로 생성되는 문제(M2)가 관측되었다.

주의:
- `sqlstore.Maintain`은 `PRAGMA wal_checkpoint(TRUNCATE)`와 사이드카 권한을 0600으로 재보증한다. context hook은 SQLite를 유지보수하지 않으므로 필요할 때 `issueops state maintain --json`을 명시적으로 실행한다.
- 오래된 IssueOps record는 `issueops prune --max-age DURATION`으로 먼저 dry-run한 뒤 `--confirm`으로 삭제한다. cutoff보다 오래된 `done` record 중 lease가 released 상태이고 issue-create intent와 remote completion reflection이 정리된 record만 대상이다.
- VACUUM은 DB가 수십 MB로 성장하기 전에는 비용만 있고 이득이 없어 비범위다(ADR 참조).

## 레코드 삭제는 related write span과 같은 게이트를 지날 것

`sqlstore`의 직렬화 게이트(`spanGate`)는 `WithSpan`만 획득한다. `Apply`/`CompareAndApply`는 게이트 밖에서 커밋하므로, 게이트를 지나지 않는 삭제는 열려 있는 related-update span과 순서를 맺지 못하고 삭제 뒤 related row가 되살아난다. CAS는 대상 레코드의 drift만 막는다.

- 레코드와 related bucket을 함께 지우는 경로는 `WithSpan` 안에서 실행한다 ([2026-08-27 lesson](lessons/2026-08-27-record-delete-bypassed-the-span-gate.md)).
- 게이트를 `Apply` 계열 안으로 내리지 않는다. span 안에서 호출되는 write가 자기 자신과 교착한다.

## SQLite state root 최초 초기화도 cross-process 경합으로 취급할 것

같은 state root를 두 process가 처음 열 때 process-local handle mutex는 data/span DB 파일 생성과 schema pragma를 직렬화하지 못한다. transaction lock만 재시도해도 open/schema 단계의 `SQLITE_BUSY`는 그대로 노출된다.

- `sqlstore.Open`의 초기화 재시도는 typed `SQLITE_BUSY`/`SQLITE_LOCKED`에만 적용하고 명명된 짧은 상한을 둔다. permission, symlink, schema, path 오류를 retry로 숨기지 않는다.
- cross-process 회귀는 양쪽 helper가 준비됐다는 barrier 뒤 동시에 진입시킨다. 단순히 process 두 개를 순서대로 `Start`한 것만 actual contention 증거로 삼지 않는다.
- expected loser error를 exact allowlist로 분류한다. 이미 artifact가 확정된 뒤의 phase exclusion과 live claim exclusion 외 오류는 `blocked`로 축약하지 말고 helper stderr와 nonzero exit로 남긴다.
- parent는 첫 `Wait` 실패에서 즉시 종료하지 말고 시작된 모든 helper를 회수해 orphan과 TempDir cleanup race를 방지한다.
