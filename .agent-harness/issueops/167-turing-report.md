# #167 Turing 리포트 — 실행 모드 전환

lifecycle: `io-f571f26d660d`
issue: https://github.com/m16khb/agent-harness/issues/167
branch: `167-execution-mode-switch` (base `e0f2b8880b6911039a50dad43a762951978fa000`)

## 목표와 판정 기준

한 번 `direct`로 준비된 lifecycle이 어떤 명령으로도 `orca`가 될 수 없고, 그 사실이 사용자에게
보이지 않는다. 명시적 `--mode orca`가 `ok: true`와 함께 `direct`를 돌려준다.

| AC | 판정 | 증거 |
|---|---|---|
| AC-01 모드 불일치를 조용히 성공시키지 않는다 | 충족 | `TestPrepareRejectsExplicitModeThatDiffersFromThePreparedOne` |
| AC-02 홀더 없는 lease에서 전환 가능 | 충족 | `TestSwitchModeApplyReplacesTheExecutionRecord` |
| AC-03 writer가 있으면 거부하고 안내 | 충족 | `TestSwitchModeRefusesWhileALeaseHoldsAWriter` (active·revoking) |
| AC-04 잃을 작업이 있으면 거부 | 충족 | `switchModeGates` ④ — `worktree_clean`, `worktree_commits_pushed` |
| AC-05 3단 확인 | 충족 | `TestSwitchModeApplyRequiresConfirm`, `TestSwitchModeApplyRejectsAStaleFingerprint` |
| AC-06 세 지점 등록 | **부분 충족** | commandparse·가드 등록. MCP는 의도적 제외(아래) |
| AC-07 RED 선행 | 충족 | 빌드 실패 → 게이트 구현 → GREEN |

## 관측한 결함

`internal/core/issueops/execution_prepare.go:82-90`이 `record.Execution != nil`일 때
`resolveExecutionPrepareMode`에 도달하지 않고 `preparedExecutionResult`를 반환한다.
`preparedExecutionResultWithModes`(`:519-524`)는 `ResolvedMode`를 `record.Execution.Mode`에서
읽고 요청 모드와 비교하지 않는다.

실측:

```
$ agent-harness issueops execution prepare --id io-b2d0c0f1daf2 --mode orca --owner-host claude --json
{ "ok": true, "requested_mode": "orca", "resolved_mode": "direct" }
```

`fallback_code`가 비어 있다 — 폴백이 아니라 요청이 평가되지 않았다.

되돌릴 경로도 없다. `record.Execution` 대입은 두 곳(`execution_prepare.go:154`,
`execution_orca_intent.go:86`)이고 `nil`로 되돌리는 코드가 없다. `cleanup abandon`은 lifecycle
레코드 자체를 삭제하고(`issueops_cleanup_abandon.go:157-166`), `reset-legacy`는 schema v0→v1
전용이라 `reset_required: false`를 반환한다.

이 분기는 `a875a16`(v1 write lease 최초 도입)부터 있었다. 의도는 prepare 멱등성이고 모드
불일치는 고려되지 않았다.

### 두 번째 증상

이 사이클을 준비하는 중에 관측했다. lease를 `released`로 반납한 뒤 같은 모드로
`prepare --confirm`을 다시 부르면 새 lease를 잡지 않고 `released`를 그대로 반환한다. 같은 82행
분기가 원인이다. 복구하려면 `replace --preview` → `--reseed` → `claim` 3단을 손으로 밟아야
한다. 이번 범위 밖으로 두고 후속 이슈로 낸다.

## 변경

| 파일 | 내용 |
|---|---|
| `internal/core/issueops/execution_prepare.go` | 모드 불일치를 `ok:false`로 거부하고 `switch-mode` 안내. `auto`는 통과 |
| `internal/core/issueops/execution_mode_switch.go` (신규) | `SwitchExecutionMode` — 게이트 5종, preview→fingerprint→apply |
| `internal/core/commandparse/issueops.go` | `execution switch-mode` spec |
| `internal/core/lifecycle/lifecycle_execution_guard.go` | typed control plane 등록 |
| `cmd/harness/issueopscli/executioncmd/execution.go` | CLI 서브커맨드 |

게이트 5종:

1. `mode_actually_changes` — 같은 모드 전환은 지울 이유가 없다
2. `lease_holds_no_writer` — `cleanupAbandonLeaseHoldsWriter` 재사용
3. `pending_intent_absent` — 외부 mutation 미해소 상태에서 정리 금지
4. `worktree_clean` / `worktree_commits_pushed` — 잃을 작업 없음
5. `orca_branch_name_free` — orca 대상일 때 원격에 이름이 비어 있음

## 설계에서 정정한 것

### `ensureOrcaBranchIsFree`를 재사용하지 못한다

design review의 refactor-plan은 판정 중복을 피하려고 재사용을 예정했다. 구현에서 막혔다 —
그 함수는 로컬 refs도 보는데(`execution_prepare.go:286`), 로컬 브랜치는 전환이 **지울 대상**이다.
게이트가 자기가 치울 것을 근거로 거부하는 모순이 된다.

질문이 다르다. prepare는 "지금 이름이 비어 있는가"를 묻고, switch-mode는 "정리 후에도 비어
있을 것인가"를 묻는다. 후자의 답은 원격에만 있다. 원격만 보는 검사를 따로 두고 그 차이를
주석으로 남겼다.

### MCP 카탈로그는 등록하지 않는다

AC-06 문면은 세 곳을 요구했다. `sync-base`가 선례다 —
`cmd/harness/issueopscli/executioncmd/execution.go:283-285`가 "CLI 전용 표면이고 MCP action
카탈로그·mcp golden은 변경하지 않는다"고 명시하며 실제로 `issueops_catalog.go`와
`mcp_tool_issueops_execution.go` 어디에도 없다.

`switch-mode`는 같은 성격이다 — lease writer가 없을 때만 동작하고, 파괴적이며, 3단 확인을
요구한다. MCP 표면은 원격 호출자에게 열리므로 워크트리를 지우는 조작을 거기 두면 3단 확인이
약해진다. durable state에 결정으로 기록했다.

## 외부 근거

orca가 기존 브랜치·경로를 입양하지 않는다는 것이 전환 설계의 전제다. 공개 소스가 근거다 —
`stablyai/orca` `src/main/ipc/workspace-create-error-classifier.ts:25-33`

```ts
if (
  text.includes('already exists locally') ||
  text.includes('already exists on a remote') ||
  text.includes('already exists. pick') ||
  ...
) {
  return 'path_collision'
}
```

## 검증

```
go build ./...                                  성공
go test ./internal/core/issueops/... -count=1   PASS
go test ./internal/core/commandparse/ -count=1  PASS
go test ./internal/core/lifecycle/... -count=1  PASS
go test ./... -count=1                          PASS (전 패키지)
```

## 남긴 것

- `classifier.go:271` — `dispatched`가 아닌 모든 dispatch를 `FindingInventoryUnknown`으로 분류.
  유효 어휘 5개 중 4개 오분류
- `classifier.go:486` — `knownGateStatus`에 `GateStatus`의 `timeout` 누락
- released lease를 prepare가 재claim하지 않음 (위 두 번째 증상)

세 건 모두 후속 이슈로 낸다.
