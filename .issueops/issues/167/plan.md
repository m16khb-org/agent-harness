# #167 실행 모드 전환

이슈: https://github.com/m16khb-org/issueops/issues/167
lifecycle: io-f571f26d660d
branch: 167-execution-mode-switch (base e0f2b8880b6911039a50dad43a762951978fa000)

## 결함

`internal/core/issueops/execution_prepare.go:82-90`

```go
if record.Execution != nil {
	if record.Execution.Pending != nil {
		...
	}
	return preparedExecutionResult(record, requested), nil
}
```

`record.Execution`이 있으면 `resolveExecutionPrepareMode`에 도달하지 않는다.
`preparedExecutionResultWithModes`(`:519-524`)가 `ResolvedMode`를 `record.Execution.Mode`에서
읽어 반환하고 요청 모드와 비교하지 않는다.

### 증상 ①: 모드 요청이 조용히 무시된다

```
$ issueops execution prepare --id io-b2d0c0f1daf2 --mode orca --owner-host claude --json
{ "ok": true, "requested_mode": "orca", "resolved_mode": "direct" }
```

`fallback_code`도 비어 있다 — 폴백이 아니라 요청이 평가되지 않았기 때문이다.

### 증상 ②: released lease를 재claim하지 않는다

이 사이클을 준비하는 중에 관측했다. lease를 `released`로 반납한 뒤 같은 모드로 `prepare
--confirm`을 다시 부르면, 새 lease를 잡지 않고 `released` 상태를 그대로 반환한다. 같은 82행
분기가 원인이다. 복구하려면 `replace --preview` → `--reseed` → `claim` 3단을 손으로 밟아야
한다.

### 되돌릴 경로가 없다

`record.Execution` 대입은 두 곳(`execution_prepare.go:154`, `execution_orca_intent.go:86`)이고
`nil`로 되돌리는 코드가 없다. `cleanup abandon`은 lifecycle 레코드 자체를 삭제한다
(`issueops_cleanup_abandon.go:157-166`).

## 설계

### ① prepare가 모드 불일치를 거부한다

`auto`는 기존 모드를 그대로 받아들인다 — 멱등성 유지. 명시적 `direct`/`orca`가 기존 모드와
다르면 `ok: false`로 거부하고 `next_command`에 전환 명령을 담는다.

### ② `execution switch-mode` 서브커맨드

`cleanup abandon`과 같은 preview → fingerprint → apply --confirm 3단.

- **preview**: 지워질 워크트리·브랜치를 나열하고 fingerprint를 준다
- **apply**: 게이트를 재검사하고 워크스페이스를 정리한 뒤 execution record를 교체한다

게이트:

| 게이트 | 기준 | 근거 |
|---|---|---|
| lease 홀더 부재 | `claimable` 또는 `released` | `cleanupAbandonLeaseHoldsWriter:273-278` 재사용 |
| pending intent 부재 | `record.Execution.Pending == nil` | 외부 mutation 미해소 상태에서 정리 금지 |
| 모드 실제 변경 | 요청 모드 ≠ 기존 모드 | 같은 모드 전환은 무의미 |
| 워크스페이스 청결 | 미커밋 변경·미푸시 커밋 없음 | 잃을 작업이 있으면 거부 |
| fingerprint 일치 | preview 이후 상태 불변 | TOCTOU |

### 전환이 워크스페이스 정리를 동반하는 이유

orca는 기존 브랜치·경로를 입양하지 않는다. 공개 소스가 근거다 —
`src/main/ipc/workspace-create-error-classifier.ts:25-33`이 "already exists locally",
"already exists. pick"을 `path_collision`으로 분류한다. `direct`가 만든 워크트리는 orca
목록에 없으므로 `prepareWorktree`(`internal/adapter/orca/execution.go:390-423`)가 재사용
후보를 찾지 못하고 `CreateWorktree`를 호출하는데, 브랜치가 이미 있으면 orca가 거부한다.

## 수용 기준

- AC-01 요청 모드가 기존 모드와 다르고 `auto`가 아니면 `ok: true`로 조용히 반환하지 않는다
- AC-02 lease가 `claimable`/`released`이고 pending이 없으면 전환이 가능하다
- AC-03 `active`/`revoking`이거나 pending이 있으면 거부하고 해소 명령을 안내한다
- AC-04 워크스페이스에 잃을 작업이 있으면 거부한다
- AC-05 전환은 preview → fingerprint → apply --confirm 3단을 요구한다
- AC-06 새 서브커맨드가 commandparse spec·가드 allowlist에 등록된다 (MCP는 제외 — 아래)
- AC-07 RED가 현재의 조용한 무시를 실증한다

AC-06은 #158의 선례가 근거다 — `decision add`가 allowlist만 고쳐서는 통과하지 못하고
`ParseExactIssueOpsCommand`의 두 단어 목록과 `IssueOpsCommandSpec`까지 필요했다.
`execution`은 이미 두 단어 목록에 있으므로 spec과 가드 두 곳을 등록했다.

### MCP 카탈로그는 등록하지 않는다

이슈 본문의 AC-06은 세 곳을 요구했지만 MCP는 제외한다. `sync-base`가 같은 판단의 선례다 —
`cmd/issueops/issueopscli/executioncmd/execution.go:283-285`가 "sync-base는 CLI 전용 표면이고
MCP action 카탈로그·mcp golden은 변경하지 않는다"고 명시하며, 실제로 `issueops_catalog.go`와
`mcp_tool_issueops_execution.go` 어디에도 없다.

`switch-mode`는 `sync-base`와 같은 성격이다: lease writer가 없을 때만 동작하고, 파괴적이며,
3단 확인을 요구한다. MCP 표면은 원격 호출자에게 열리므로 워크트리를 지우는 조작을 거기 두면
3단 확인의 의미가 약해진다. durable state에 결정으로 기록했다.

## 검증

```
go test ./internal/core/issueops/... -count=1
go test ./internal/core/commandparse/... -count=1
go test ./internal/core/lifecycle/... -count=1
go test ./... -count=1
```

## 비범위

- orca가 기존 git worktree를 입양하게 만드는 것. orca가 `path_collision`으로 거부한다
- `cleanup abandon`의 계약 변경
- 증상 ②(released lease 재claim)의 자동화. 같은 원인이지만 별개 계약이고, 이번 변경이
  모드 불일치를 거부하게 되면 그 경로가 더 드러난다. 후속 이슈로 낸다
