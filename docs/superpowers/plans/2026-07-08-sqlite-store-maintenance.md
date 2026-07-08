# sqlite 상태 저장소 유지보수 자동화 (2026-07-08)

sqlite 전환(ADR "State storage moves from JSON files + flock to SQLite", 커밋 f01ae08..b180106) 이후 남은 운영 결함 3건을 하나의 사이클로 해소한다. 구현은 태스크 단위 TDD로 진행하고, 태스크마다 정확히 하나의 커밋을 만든다.

## 문제 근거 (2026-07-08 실측)

| ID | 증상 | 증거 |
|----|------|------|
| M1 | WAL 파일이 checkpoint 후에도 truncate되지 않고 고수위로 유지 | `~/.local/state/agent-harness/issueops/harness.db-wal` 4,136,512B vs `harness.db` 200,704B (20배). sqlstore에 checkpoint/vacuum API 없음 (`internal/core/sqlstore/sqlstore.go`에 `wal_checkpoint` 미존재) |
| M2 | WAL/SHM 사이드카가 0644로 생성될 수 있음 (state는 0600 계약) | 실측 `harness.db-wal` `-rw-r--r--` (umask 022 환경, 데몬 생성분). `touchPrivate`는 `harness.db`/`harness.lock.db`만 커버 (`sqlstore.go` touchPrivate). 단 테스트 환경에서는 0600 상속이 관측됨 — 재현 조건 진단이 선행 단계 |
| M3 | 스테일 데이터가 수동 정리 외에 줄지 않음 | 전환 전 실측: 세션 바인딩 8,935행 상당 + done 사이클 잔존. `state prune`/`cleanup stale --prune-done`은 수동 전용. 세션 바인딩은 어떤 prune 표면에도 안 잡힘 (session bucket은 `StatePrune` 대상 아님) |

기존 재사용 패턴: 상각(amortized) 유지보수는 `MaybeDetectStuckWorkerJobs`(sentinel mtime, `internal/core/worker/store.go:199-`)가 선례이고, 호출 지점은 `cmd/harness/hookcli/hookcatalog/catalog.go:82`(6h 간격 hook 경로)다. 훅에서의 저장소 유지보수는 workflow 작업(이슈/PR/파일 편집)이 아니므로 hook boundary 계약과 충돌하지 않는다.

## 불변 제약 (전 태스크 공통)

1. TDD 순서 강제: 실패 테스트 → 실패 확인 → 최소 구현 → 통과 확인. 명령 출력 없이 완료 처리 금지.
2. sqlstore span 규율 유지: span 중첩 금지, 유지보수 작업은 span 밖(자체 커넥션)에서 수행하되 데이터 정합성은 sqlite 트랜잭션에 위임.
3. 응답 DTO는 additive-only. CLI/MCP 표면 추가 시 같은 태스크에서 contract/usage 골든 갱신.
4. 독립 실행: 외부 도구/키/네트워크 의존 금지.
5. 파괴적 동작(행 삭제, VACUUM)은 dry-run 기본 + `--confirm`/`--apply` 게이트. 삭제 전 개수·키를 결과에 보고.
6. 커밋은 태스크당 1개, Conventional Commit + Lore 형식(`.agent-harness/COMMIT_POLICY.md`).

## Task 1: sqlstore.Maintain — WAL truncate + 사이드카 권한 재보증

- Files: `internal/core/sqlstore/sqlstore.go`, `internal/core/sqlstore/maintain.go`(신규), `internal/core/sqlstore/maintain_test.go`(신규)
- Interfaces:
  - `type MaintainResult struct { Dir string; WALBytesBefore, WALBytesAfter int64; Checkpointed bool; PermissionsFixed []string }`
  - `func (d *DB) Maintain() (MaintainResult, error)` — ① `PRAGMA wal_checkpoint(TRUNCATE)` 실행, ② `harness.db*`/`harness.lock.db*` 전 사이드카(-wal/-shm/-journal) 존재 시 0600 재설정, ③ 전후 WAL 크기 보고
- Steps:
  - [ ] RED: `TestMaintainTruncatesWAL` — 수백 행 Put으로 WAL을 키운 뒤 Maintain 호출, `WALBytesAfter < WALBytesBefore`이고 truncate 후 WAL이 0 또는 32B 헤더 수준인지 단언. 실행: `go test ./internal/core/sqlstore -run TestMaintainTruncatesWAL -count=1` → FAIL(미구현) 확인
  - [ ] RED: `TestMaintainRestoresPrivateSidecarPermissions` — 사이드카를 의도적으로 0644로 chmod 후 Maintain이 0600으로 되돌리고 `PermissionsFixed`에 보고하는지 단언 → FAIL 확인
  - [ ] 구현: maintain.go 작성 (checkpoint는 `d.data.Exec`, 권한은 os.Chmod; 열린 span과 무관하게 안전 — checkpoint는 reader/writer와 동시 실행 가능, busy 시 `Checkpointed=false`로 보고하고 에러로 처리하지 않음)
  - [ ] GREEN: 위 두 테스트 + `go test ./internal/core/sqlstore -count=1 -race`
  - [ ] M2 진단 기록: umask 022에서 사이드카 생성 권한을 재현하는 테스트(`TestSidecarPermissionsUnderUmask`)를 추가하고, modernc가 db 파일 권한을 상속하는지/umask를 따르는지 관찰 결과를 테스트 주석에 남긴다(수정은 Maintain의 재보증으로 커버).
- Commit: `feat(sqlstore): add Maintain with WAL truncate and sidecar permission repair`

## Task 2: state maintain CLI + MCP 표면

- Files: `cmd/harness/statecli/state_cli_router.go`(서브커맨드 등록), `cmd/harness/statecli/state_cli_maintenance.go`(maintain 브랜치 추가), `internal/core/state_trace_facade.go`(facade), `internal/core/state/state_maintain.go`(신규: 알려진 root 4곳 — StateDir, issueops, workpool, worker — 순회), `cmd/harness/mcpcli/mcp_tool_policy_state.go`+`internal/adapter/mcp` catalog(도구 `state_maintain`), `cmd/harness/testdata/usage.golden.txt`, `cmd/harness/testdata/mcp_tools.golden.json`, `cmd/harness/testdata/response_contracts.golden.json`
- Interfaces:
  - `func StateMaintain() (StateMaintainResult, error)` — `{ OK bool; Roots []sqlstore.MaintainResult; Skipped []string }` (root 부재 시 Skipped)
  - CLI: `agent-harness state maintain [--json]`, MCP: `state_maintain` (read-mostly; 파괴적 아님 — VACUUM은 이번 사이클 비범위)
- Steps:
  - [ ] RED: `TestRunStateMaintainReportsRoots`(statecli) + `TestMCPStateMaintain`(mcpcli) → FAIL 확인
  - [ ] 구현 + 골든 갱신(`-update`), usage 골든 포함
  - [ ] GREEN: `go test ./cmd/harness/statecli ./cmd/harness/mcpcli ./cmd/harness/harnessapp -run 'Maintain|Golden' -count=1`
- Commit: `feat(state): add state maintain surface over sqlite store roots`

## Task 3: 스테일 세션 바인딩 정리 (issueops cleanup stale 확장)

- Files: `internal/core/issueops/session/session.go`(스캔 helper), `internal/core/issueops/issueops_stale_scan.go`, `internal/core/issueops/issueops_stale_scan_apply_test.go`, `cmd/harness/issueopscli/issueops.go`(결과 필드 노출), 관련 MCP 결과 DTO(additive)
- Interfaces:
  - `func StaleBindings(store Store, isCycleLive func(cycleID string) bool) ([]string, error)` — session bucket 전체를 훑어 ① 대응 사이클 레코드가 없거나 ② done 인 바인딩 키 목록 반환
  - `IssueOpsStaleScanResult`에 `PrunedBindings int` (omitempty, additive) 추가; `--apply`일 때만 삭제
- Steps:
  - [ ] RED: `TestScanStaleApplyPrunesOrphanSessionBindings` — done 사이클의 scoped 바인딩 + 레코드 없는 바인딩을 심고 `--apply` 후 삭제, 라이브 사이클 바인딩은 보존 단언 → FAIL 확인
  - [ ] RED: `TestScanStaleDryRunReportsBindingsWithoutDelete` → FAIL 확인
  - [ ] 구현: 삭제는 session span 안에서 read-재확인 후 수행(TOCTOU: 삭제 직전 바인딩이 다른 라이브 사이클로 재바인딩됐으면 skip)
  - [ ] GREEN: `go test ./internal/core/issueops/... ./cmd/harness/issueopscli/... -count=1 -race`
- Commit: `feat(issueops): prune orphan session bindings in stale cleanup`

## Task 4: 상각 자동 유지보수 트리거

- Files: `internal/core/state/state_maintain.go`(`MaybeMaintainStateStores(minInterval)` — sentinel `.last-store-maintain` mtime 게이트, StateDir 루트에 저장), `internal/core/workflow_facade.go`(facade), `cmd/harness/hookcli/hookcatalog/catalog.go`(기존 `MaybeDetectStuckWorkerJobs(6h)` 옆에 `MaybeMaintainStateStores(24h)` 배선)
- Steps:
  - [ ] RED: `TestMaybeMaintainStateStoresAmortizes` — 첫 호출 ran=true, 즉시 재호출 ran=false, sentinel mtime 갱신 단언 → FAIL 확인
  - [ ] 구현: worker sentinel 패턴 복제(에러여도 sentinel touch — 폭주 방지). 훅 경로 예산: Maintain은 checkpoint+chmod뿐이므로 ms 단위. 바인딩 정리는 여기 포함하지 않음(파괴적 삭제는 명시 `--apply` 전용 유지).
  - [ ] GREEN: `go test ./internal/core/state/... ./cmd/harness/hookcli -count=1` + hook 계약 골든 확인
- Commit: `feat(state): amortized store maintenance on the hook catalog path`

## Task 5: 문서 + 전체 검증 + 실측 dogfood

- Files: `.agent-harness/OPERATIONS.md`(유지보수 절차: `state maintain`, `issueops cleanup stale --apply --prune-done`, 자동 트리거 주기), `.agent-harness/CAUTIONS.md`(WAL 고수위·사이드카 권한 항목), `.agent-harness/ADR.md`(유지보수 정책 결정: VACUUM 비채택 사유 포함)
- Steps:
  - [ ] 문서 3건 갱신 (골든 영향 시 재생성)
  - [ ] 전체 배터리: `go test -p 1 -timeout 20m ./... -count=1` → exit 0; `go test -race -p 1 ./... -count=1` → FAIL 0; `go build -o bin/agent-harness ./cmd/harness`
  - [ ] 실측 dogfood: 실제 state 디렉토리에서 `./bin/agent-harness state maintain --json` 실행 → issueops WAL 4.1MB가 truncate되고 사이드카 0600 복구되는지 확인, `state doctor` healthy 유지 확인. 증거를 ADR/커밋 본문에 기록
  - [ ] 데몬 재시작(새 바이너리) + `claude mcp list` 재확인
- Commit: `docs(agent-harness): sqlite store maintenance policy and operations` (+ 검증에서 결함 발견 시 별도 fix 커밋)

## 비범위 (명시 거부)

- **VACUUM / auto_vacuum**: 현재 DB 200KB 수준에서 공간 회수 이득이 없고 전체 잠금 비용만 있음. DB가 수십 MB로 성장하면 재평가 (ADR에 기록).
- **sqlstore 핸들 eviction**: 열린 DB 핸들은 dir당 캐시되어 fd를 점유하지만 실측 사용 root는 ~10개 미만. 장수 데몬에서 projects/<id> 스팬이 수백 개 dir로 늘 때 재평가.
- **레거시 파일 자동 삭제 명령**: 2026-07-08 수동 정리 완료, 신규 설치에는 레거시가 없음.
- **일반 cron/스케줄러**: 데몬에 타이머 도입은 상각 sentinel 패턴보다 복잡도만 높음.

## 완료 기준

1. Task 1~5 커밋 5개(±fix), 전체/race 배터리 green.
2. 실측: issueops WAL ≤ 헤더 수준으로 truncate, 사이드카 전부 0600, doctor healthy.
3. 스테일 바인딩/done 사이클이 `cleanup stale --apply`로 감소하는 dogfood 증거.
4. OPERATIONS에 운영 절차, ADR에 정책 결정 기록.
