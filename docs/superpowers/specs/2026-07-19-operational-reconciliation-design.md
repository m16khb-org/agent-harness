# Operational Reconciliation and Residue Prevention Design

**Date:** 2026-07-19
**Status:** Design approved; implementation and cleanup not yet executed
**Scope:** agent-harness의 Git, IssueOps, Orca, user-state를 하나의 운영 정합성 모델로 진단하고 현재 누적된 terminal/task/worktree/branch/state residue와 stale IssueOps record를 안전하게 정리한다.

## 1. 문제와 배경

현재 각 저장소는 자기 내부 무결성만 부분적으로 확인한다. 그 결과 다음처럼 각 표면에서는 설명 가능한 상태가 전체 운영 관점에서는 서로 모순될 수 있다.

- IssueOps session binding은 cycle 소유 관계를 나타내지만 worker가 지금 살아 있다는 증거는 아니다.
- IssueOps cycle이 `done`이 아니어도 연결된 worktree, branch, task, terminal이 이미 사라졌거나 종료됐을 수 있다.
- 반대로 Git/Orca artifact가 남아 있어도 이를 소유하는 live IssueOps cycle이 없을 수 있다.
- Orca task의 status와 completion metadata가 서로 모순될 수 있다.
- top-level `doctor`는 state-root의 비정상 파일은 찾지만 IssueOps, Orca, Git 간 교차 정합성은 판정하지 않는다.
- 현재 stale scan은 안전하게 보수적이지만, session binding을 liveness로 오인하지 않는 공통 operational-health 판정은 없다.

이번 작업은 두 결과를 함께 달성한다.

1. 현재 누적 상태를 외부 복구 bundle과 journal로 고정한 뒤, 사용자가 승인한 전체 정리 범위에 맞게 실제 artifact를 제거하고 stale record를 release한다.
2. 같은 종류의 residue가 다시 쌓이면 `agent-harness doctor`와 stability audit가 즉시 unhealthy로 판정하도록 공통 분류기를 추가한다.

## 2. 검증된 감사 스냅샷

아래 숫자는 2026-07-19 설계 직전의 읽기 전용 감사 결과다. 실행 권한이나 삭제 대상 목록의 source of truth가 아니다. 실제 정리 직전에는 전체 inventory를 다시 만들고 그 manifest만 사용한다.

| 표면 | 감사 결과 |
|---|---|
| Git source checkout | `main` clean, `HEAD == origin/main == 752ba66741fd0436f879539950f59257fae9affc` |
| Git worktree | main 1개와 non-main 13개, 모두 clean |
| Local branch | non-main 15개 |
| Remote branch | non-main 18개; 설계 대기 중 branch 47이 새로 나타나 실행 시 재인벤토리가 필요함을 실증 |
| GitHub | open issue 0, open PR 0 |
| Orca terminal | 현재 operator terminal 1개와 stale/orphan terminal 8개 |
| Orca worktree | main 1개와 non-main 13개 |
| Orca orchestration | task 55개(`ready=12`, `completed=22`, `failed=21`), gate 0개; ready 8개는 completion metadata도 보유 |
| IssueOps | record 20개 중 `done=3`, non-done 17개; 현재 stale scan이 즉시 release 가능한 것은 2개뿐 |
| User state | top-level doctor가 state-root의 recovery artifact 5개를 warning으로 판정 |

추가로 설치된 Orca 공개 계약을 직접 확인했다.

- `orchestration task-list --brief --json`은 전체 task를 bounded spec projection과 `count`로 읽을 수 있다.
- `orchestration inbox`는 `--limit`은 제공하지만 total-count 또는 truncation 증거를 제공하지 않는다.
- 개별 task/message/gate 삭제는 없고 `orchestration reset --all`만 전체 orchestration state를 비운다.
- 공개 `export`, `import`, 조건부 reset, restore 명령은 없다.

따라서 Git과 SQLite backup은 복원 가능하게 만들지만 Orca snapshot은 감사용 archival evidence일 뿐 복원 수단이 아니다. 사용자는 이 비가역 한계를 명시적으로 승인했다.

## 3. 목표

- top-level `agent-harness doctor`를 Git, IssueOps, Orca, user-state 교차 정합성의 단일 공개 진단 표면으로 유지한다.
- ownership, liveness, invocation-scoped preservation, residue를 서로 다른 개념으로 판정한다.
- session binding만으로 cycle을 live로 판정하지 않는다.
- 모든 불완전, 중복, 미식별, truncated inventory는 healthy가 아니라 unknown/unhealthy로 판정한다.
- 같은 pure classifier를 doctor와 IssueOps stale scan이 소비하게 해 liveness 의미가 갈라지지 않게 한다.
- 현재 operator terminal만 보존하고 그 외 terminal, orchestration task/message/gate, non-main worktree, non-main local/remote branch를 제거한다.
- 실행 manifest에 고정된 모든 non-done IssueOps record를 fresh read 뒤 명시적으로 force-release한다. 감사 시점에는 17개였으며 전체 record는 20개였다. 실행 전 새 record가 발견되면 승인된 "모든 상태 정리" 범위 안에서 manifest에 포함하되, manifest seal 뒤 새 record가 생기면 중단한다.
- state-root recovery artifact 5개를 외부 bundle로 옮겨 top-level doctor를 healthy로 만든다.
- 모든 destructive mutation을 exact identity, manifest digest, expected Git OID, durable journal로 fence한다.
- crash 뒤 rollback을 추측하지 않고 동일 journal에서 idempotent forward recovery한다.
- 표준 test/race/build/golden, quick/full self-verify 95점 gate, daemon/process hygiene를 모두 통과한다.

## 4. 비목표

- Orca 자체를 설치, 수정, patch하거나 private storage를 직접 편집하지 않는다.
- Orca의 task/message/gate storage를 agent-harness에 복제하지 않는다.
- generic cleanup framework, scheduler, plugin registry, persistent exemption registry를 만들지 않는다.
- 새 `issueops cleanup reconcile` 명령이나 별도 operational-health CLI/MCP를 추가하지 않는다.
- heartbeat가 늦었다는 이유만으로 worker를 interrupt하거나 cycle을 자동 release하지 않는다.
- 현재 cleanup을 일반 테스트 fixture로 재현하기 위해 실제 global Orca reset을 실행하지 않는다.
- open issue/PR가 없는 현재 상태를 이용해 remote history의 의미를 재작성하거나 force-push하지 않는다.

## 5. 선택한 접근과 기각한 대안

### 5.1 선택: 공통 pure classifier + 기존 doctor + manifest 기반 일회 정리

`internal/core`에 host-neutral operational-health classifier를 둔다. 분류기는 clock과 외부 I/O를 직접 호출하지 않고 정규화된 snapshot, 명시적 current time, invocation-scoped preserve set만 입력받아 deterministic finding을 반환한다.

CLI adapter는 현재 저장소와 선택적 Orca adapter에서 complete inventory를 모아 core doctor에 주입한다. doctor는 기존 `checks`와 `issues`에 operational result를 합친다. IssueOps stale scan은 cycle별 operational finding을 같은 분류기에서 받아 기존 classification 설명과 revalidation에 사용한다.

현재 상태 정리는 제품에 범용 delete 명령을 추가하지 않고 operator 절차로 수행한다. 외부 0700 recovery bundle에 manifest, restorable backup, archival snapshot, append-only journal을 만든 뒤 exact target만 순서대로 제거한다.

이 방식은 새 public surface를 만들지 않으면서 진단 의미를 한곳에 두고, 현재의 대규모 cleanup은 명시적 사용자 승인 경계 안에서만 수행한다.

### 5.2 기각: session binding 또는 exact resource match만으로 live 판정

binding과 resource ID가 모두 일치해도 worker가 죽은 채로 순환 참조만 남을 수 있다. 이는 Brooks 검토에서 발견된 핵심 오류다. binding은 ownership 증거로만 사용하고 liveness는 fenced heartbeat와 현재 external state를 별도로 요구한다.

### 5.3 기각: 별도 reconcile/cleanup 제품 명령

top-level doctor, stale scan, 새 reconcile 명령이 각자 health 의미를 소유하면 세 번째 진실 원천이 생긴다. 현재 요청에 필요한 것은 판정과 이번 일회 정리이지 범용 orchestration garbage collector가 아니다.

### 5.4 기각: reset만 먼저 실행한 뒤 남은 상태를 맞춤

Orca reset은 복원할 수 없고, reset 뒤에는 어떤 task/message/gate가 있었는지 증거가 사라진다. Git ref와 IssueOps state도 동시에 변하면 crash recovery가 불가능하다. 먼저 manifest와 backup을 고정하고, quiescence와 동일 digest를 두 번 확인한 뒤 reset한다.

## 6. Brooks 적대 리뷰와 설계 수정

사용자 요청에 따라 구현 전에 독립 Brooks reviewer를 사용했다. reviewer verdict는 `stop`이었다.

### 6.1 핵심 blocker

초기 설계는 exact IssueOps binding과 Orca resource tuple이 일치하면 cycle을 보존할 수 있었다. 그러나 이는 liveness가 아니라 internally consistent dead cycle도 clean으로 통과시키는 ownership 증거뿐이었다.

reviewer가 요구한 가장 싼 반증 fixture는 다음과 같다.

> non-done bound cycle과 exact ready task는 존재하지만 active lease/heartbeat가 없다. 이 snapshot은 반드시 unhealthy여야 한다.

이 fixture를 첫 named RED test로 사용한다.

### 6.2 반영한 수정

- `owned`, `live`, `preserved`, `residue`를 별도 상태로 분리한다.
- claimed handoff는 exact resource tuple과 15분 이내 fenced heartbeat가 모두 있어야 live다.
- planning, dispatched, paused/recovery state는 영구 예외가 아니라 명시적 `--preserve-cycle`이 있는 이번 doctor invocation에서만 preserved다.
- 현재 operator terminal도 명시적 `--preserve-terminal`로만 preserved다.
- doctor가 단일 공개 health 표면이며 stale scan은 동일 finding을 소비한다.
- local/remote/integration branch를 여러 finding taxonomy로 나누지 않고 `non_main_branch_residue` 하나로 집계한다.
- CLI/MCP wiring보다 pure classifier fixture를 먼저 완성한다.
- cleanup manifest에 exact resource IDs, record digest, local/remote expected OID를 모두 고정한다.
- external 0700 journal에서 각 mutation을 idempotent forward recovery한다.
- Git과 SQLite는 restore test가 가능한 backup을 만들고 Orca snapshot은 archival-only로 명시한다.

### 6.3 명시적으로 수용한 잔여 위험

설치된 Orca에는 conditional reset, global mutation CAS, complete message export, import/restore가 없다. 다른 terminal과 coordinator run을 멈추고 동일 full digest를 두 번 읽더라도 별도 외부 Orca CLI actor가 그 직후 race할 가능성을 절대 제거할 수 없다.

이 한계는 숨기지 않는다. reset 직전 마지막 digest가 manifest와 다르면 중단하고, reset 이후에는 durable journal을 기준으로 forward recovery한다. 사용자는 이 비가역성과 residual race를 승인했다.

## 7. Operational-health 모델

### 7.1 입력 snapshot

분류기는 다음 normalized input만 받는다.

- canonical repo path와 source checkout branch/HEAD/clean status
- Git worktree 목록과 local/remote branch ref/OID 목록
- 모든 IssueOps record와 primary/scoped session binding
- 각 handoff의 attempt, ownership epoch, context hash, state, task/dispatch/terminal/worktree identity, last heartbeat
- Orca runtime ID와 complete repo worktree/terminal/task inventory, count-checked gate inventory
- Orca inbox의 bounded message-presence observation
- state doctor가 발견한 unexpected runtime/recovery artifact
- 호출자가 주입한 `now`
- invocation-scoped `preserve_cycle_ids`와 `preserve_terminal_handles`

Snapshot collection과 classification을 분리한다. collection error, duplicate ID, unsupported status, count mismatch, truncated row, incomplete identity는 빈 목록으로 축약하지 않고 explicit `unknown` finding을 만든다.

### 7.2 네 가지 의미

| 의미 | 정의 |
|---|---|
| `owned` | exact IssueOps record가 resource identity를 참조한다. 소유 관계이며 생존 증거가 아니다. |
| `live` | 현재 mutation authority가 exact하고 fenced heartbeat와 external resource state가 모두 유효하다. |
| `preserved` | live는 아니지만 이번 invocation에서 operator가 exact cycle/terminal을 명시적으로 보존했다. |
| `residue` | live/preserved owner가 없거나, terminal owner가 끝났거나, resource 상태가 owner 상태와 모순된다. |

한 resource가 여러 cycle에 owned되면 어느 cycle에도 귀속하지 않고 ambiguous/unhealthy로 판정한다. 알 수 없는 상태는 clean이 아니다.

### 7.3 Cycle liveness 규칙

1. `phase=done`은 terminal이다. session binding이나 external artifact가 남아 있어도 live가 아니며 남은 것은 residue다.
2. `execution_handoff.state=claimed`인 cycle만 heartbeat로 자동 live가 될 수 있다.
3. claimed cycle은 다음을 모두 만족해야 한다.
   - durable attempt, epoch, context hash, native worker identity가 완전하다.
   - exact Orca worktree, terminal, task, dispatch가 한 번씩만 존재하고 record tuple과 일치한다.
   - designated worker terminal은 connected와 writable이 모두 true다.
   - task/dispatch 상태가 claimed worker와 모순되지 않는다.
   - `last_heartbeat_at`이 주입된 `now` 기준 15분 이내다.
4. 15분은 문서화된 5분 heartbeat cadence의 세 번 연속 누락이다. heartbeat age만으로 자동 interrupt/delete/release하지 않고 unhealthy diagnostic만 만든다.
5. `coordinator_preparing`, `dispatched`, `submitted`, `recovery_required` 또는 handoff가 없는 non-done planning cycle은 `--preserve-cycle <exact-id>`가 있을 때만 preserved다.
6. preserve 요청도 record/resource identity가 불완전하거나 서로 충돌하면 healthy로 바꾸지 못한다.
7. session binding은 exact cycle ownership을 보조하지만 2~6의 liveness 규칙을 대체하지 않는다.
8. unknown phase/handoff/task status는 fail closed다.

### 7.4 Resource 규칙

- Source checkout의 현재 branch를 canonical branch로 삼고, 그 외 local/remote branch가 live/preserved cycle에 속하지 않으면 하나의 `non_main_branch_residue` finding으로 집계한다.
- canonical source worktree 이외의 Git/Orca worktree는 live/preserved exact owner가 없으면 residue다.
- terminal은 live cycle worker terminal 또는 exact `--preserve-terminal`만 허용한다. preserved terminal도 inventory에 유일하게 존재해야 한다.
- task는 live/preserved cycle의 exact task만 허용한다. status가 ready인데 completion timestamp/result가 있는 것처럼 필드 의미가 모순되면 owner가 있어도 unhealthy다.
- gate는 returned rows와 `count`가 일치해야 하며 exact active owner를 증명할 수 없으면 residue다. Inbox의 `count`는 total이 아니라 이번 호출에서 반환한 row 수이고 truncation field가 없다. 따라서 message가 하나라도 반환되면 residue로 판정하고, `--limit 1`에서 `count=0`과 empty array가 함께 나온 경우만 absence 증거로 인정한다. Nonzero message store를 active-cycle ownership으로 healthy 처리하지 않는다.
- state-root의 허용되지 않은 recovery artifact는 residue다. 이를 whitelist에 추가하지 않는다.

### 7.5 Finding과 공개 표면

기존 doctor result shape를 유지한다. `checks`에 `operational_state` 하나를 추가하고, 기존 `issues`에 필요한 만큼 다음 안정적인 code를 사용한다.

- `operational_inventory_unknown`
- `operational_dead_owner`
- `operational_worktree_residue`
- `operational_terminal_residue`
- `operational_task_residue`
- `operational_gate_residue`
- `operational_message_residue`
- `operational_non_main_branch_residue`
- `operational_state_artifact_residue`

Branch는 local/remote별 새 code를 만들지 않는다. summary의 bounded metadata에 location과 expected/current OID를 넣는다. doctor는 계속 read-only이며 destructive fix command를 자동 실행하거나 `--fix`를 추가하지 않는다.

Public invocation은 기존 명령 하나다.

```bash
agent-harness doctor --repo . \
  --preserve-terminal "$ORCA_TERMINAL_HANDLE" \
  --preserve-cycle <cycle-id> \
  --json
```

두 preserve flag는 반복 가능하고 invocation에만 적용된다. state에 예외를 저장하지 않는다. 현재 doctor의 MCP surface가 없으므로 이 작업을 이유로 새 MCP tool을 만들지 않는다. Codex, Claude Code, GJC는 모두 공통 CLI를 호출한다.

## 8. Doctor, stale scan, stability audit 통합

### 8.1 Core 경계

- 새 작은 core unit은 normalized snapshot을 받아 findings를 반환하는 pure classifier만 소유한다.
- current time, filesystem, Git, SQLite, Orca process read는 wrapper가 수행한다.
- Orca adapter는 complete all-task `--brief` inventory, task semantic metadata, gate list, bounded message presence를 host-neutral DTO로 projection한다.
- collection wrapper는 existing worktree/terminal completeness 규율처럼 count/truncation을 검증한다.
- top-level doctor가 classifier 결과를 기존 check/issue에 투영한다.

Doctor와 stale scan이라는 두 소비자가 있으므로 이 shared pure unit은 단일-use speculative abstraction이 아니다. 외부 orchestrator registry나 generic collector interface는 만들지 않는다.

### 8.2 Stale scan

Stale scan은 자체 liveness 규칙을 다시 구현하지 않고 cycle별 operational finding을 받는다. 다만 health와 destructive eligibility는 분리한다.

- dead-but-bound/no-heartbeat cycle은 즉시 doctor unhealthy다.
- heartbeat 부재만으로는 기존 `needs-review`를 `confirmed-stale`로 올리거나 `--apply` auto-release하지 않는다.
- release 직전에는 현재처럼 fresh record와 external evidence를 다시 읽고 재분류한다.
- exact worktree absence, terminal task terminality, remote closure 같은 기존 강한 신호가 있을 때만 기존 releasable class를 유지한다.

이렇게 하면 liveness 의미는 하나지만 destructive policy는 보수적으로 유지된다.

### 8.3 Stability audit

stability audit script는 별도 정합성 계산을 하지 않고 마지막 binary의 top-level doctor를 호출한다. 현재 session을 보존해야 하면 singular optional `--preserve-terminal`을 받는다. 명시값은 exact `term_*`, 최대 256 bytes여야 하고 inherited environment보다 우선하며, 옵션이 없을 때만 기존 `ORCA_TERMINAL_HANDLE` fallback을 쓴다. sealed reconciliation은 `manifest.current_terminal.handle`을 명시 argv로 전달하고 환경 변수를 덮어쓰지 않는다. doctor가 unhealthy이거나 operational inventory가 unknown이면 audit도 실패한다. 이 doctor 호출만 상위 live harness 환경을 상속한다. audit 내부 ordinary/race Go regression은 `HARNESS_ROOT`를 exact audited source checkout으로 고정하고 `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`, `HARNESS_WORKER_DIR`는 audit 전용 임시 루트로 격리해 live IssueOps session projection을 바꾸지 않아야 한다.

## 9. 일회 전체 정리 프로토콜

### 9.1 외부 recovery bundle

Bundle root는 repo 밖의 다음 위치다.

```text
~/.local/state/agent-harness-backups/<repo-fingerprint>/<UTC-timestamp>/
```

root와 하위 디렉터리는 mode `0700`, 파일은 `0600`으로 만든다. bundle은 최소 다음을 포함한다.

```text
manifest.json
manifest.sha256
journal.jsonl
git/all-refs.bundle
git/bundle-verify.txt
git/refs.json
issueops/harness.sqlite
issueops/integrity-check.txt
state/artifacts.json
state/backup/
state/relocated/
orca/status.json
orca/worktrees.json
orca/terminals.json
orca/tasks-brief.json
orca/gates.json
orca/inbox-bounded.json
orca/limitations.json
```

- Git bundle은 cleanup 대상 ref를 포함하고 `git bundle verify`를 통과해야 한다.
- IssueOps SQLite는 SQLite online backup으로 만들고 backup DB의 `PRAGMA integrity_check`가 `ok`여야 한다. raw DB/WAL 파일을 독립 copy해 복원 가능하다고 주장하지 않는다.
- state artifact는 SHA-256, mode, size, original path를 manifest에 넣고 먼저 `state/backup/` copy hash를 확인한다. 정리 단계에서는 original을 비어 있는 `state/relocated/` target으로 rename한 뒤 두 copy의 hash를 다시 확인하며 기존 backup을 overwrite하지 않는다.
- Orca JSON은 bounded/redacted projection이다. complete message export와 restore가 없다는 제한을 `limitations.json`에 기록한다.
- command stdout에 raw message body나 secret-like value를 노출하지 않는다.

### 9.2 Manifest fence

`manifest.json`은 다음 exact target을 pin한다.

- repo fingerprint, canonical path, canonical branch와 HEAD
- 모든 Git worktree path/branch/HEAD/clean state
- 삭제할 local/remote ref와 expected full OID
- 모든 IssueOps record ID, phase, handoff identity, raw/canonical record digest, session binding
- Orca runtime ID, worktree ID/instance/path, terminal handle/PTY/tab/leaf, task/dispatch ID/status, gate ID, bounded message observation
- 보존할 현재 terminal의 runtime/handle/PTY/tab/leaf/worktree ID/path/connected/writable exact tuple
- 이동할 state artifact path와 SHA-256
- 전체 stable projection digest

Generated timestamp와 read latency 같은 volatile field는 stable digest에서 제외한다. destructive 단계는 target 하나마다 현재 identity가 manifest와 exact 일치하는지 다시 확인한다. 매 operation 전후에는 journal order로 계산한 exact phase projection을 적용해 Orca terminal/worktree/task/dispatch/gate/inbox, Git worktree와 local/remote ref, IssueOps record/session/other row, state artifact를 모두 다시 읽는다. 이미 journaled된 제거·repair·release만 허용하며 신규 ID/key/path/ref, 남아 있는 resource의 digest/OID/hash drift, incomplete inventory가 있으면 다음 mutation을 호출하기 전에 중단한다. `started` recovery는 해당 operation의 exact before/after projection 중 하나만 허용한다.

Raw digest는 SQLite에 저장된 JSON bytes 그대로의 SHA-256이다. Canonical digest는 key-sorted compact UTF-8 JSON의 SHA-256이며 `<`, `>`, `&`를 HTML escape하지 않는다. runner와 locked core CAS가 같은 규칙을 사용해야 한다.

### 9.3 Quiescence와 Orca reset 경계

1. observation을 시작하기 전에 official no-selector `orca terminal show`로 current row를 resolve한다. 그 runtime/handle/PTY/tab/leaf/worktree ID/path/connected/writable tuple이 complete terminal list의 단 하나의 row와 unique source worktree에 exact match해야 한다. 명시적으로 읽은 `ORCA_TAB_ID`, `ORCA_PANE_KEY`, `ORCA_WORKTREE_ID` composite도 같아야 하며, `ORCA_TERMINAL_HANDLE` explicit probe는 같은 current row이거나 exact structured `terminal_handle_stale`만 허용한다. caller argument는 resolved current handle에 대한 assertion일 뿐이다. 같은 resolve-and-compare를 collection의 두 관측, `validate-live`/각 `apply`/`verify-final` 진입, 모든 mutation fence 전후에 반복하고 sealed current tuple/runtime과 다르면 mutation/readback 전에 중단한다.
2. 다른 terminal을 exact handle로 close/stop하고 absence 또는 disconnected/quiescent 상태를 재확인한다.
3. active Orca coordinator run을 `orchestration run-stop`으로 멈춘다.
4. non-main Orca worktree를 exact ID로 제거하고 Git/Orca 양쪽 inventory에서 absence를 확인한다.
5. 전체 stable projection을 연속 두 번 읽어 동일 digest인지 확인한다.
6. 두 번째 digest가 manifest의 remaining-state projection과 일치할 때만 `orca orchestration reset --all --json`을 한 번 실행한다.
7. reset 뒤 task/gate/message/dispatched가 모두 0이고 current terminal과 canonical Git/Orca source worktree가 각각 정확히 하나인 sealed projection과 완전히 같은지 확인한다. 이후 모든 mutation fence도 이 exact projection을 요구한다.

Orca reset은 rollback하지 않는다. invocation timeout/transport ambiguity가 발생하면 reset을 자동 재호출하지 않고 current zero/nonzero observations와 journal을 읽어 forward recovery한다.

### 9.4 Mutation 순서

정리 순서는 다음과 같다.

1. source checkout이 clean이고 canonical HEAD가 expected origin ref와 같은지 확인한다.
2. external bundle 생성, hash 검증, Git bundle verify, SQLite restore/integrity 검증을 완료한다.
3. current terminal 외 terminal을 닫고 coordinator run을 멈춘다.
4. non-main Orca/Git worktree를 exact identity로 제거한다.
5. Orca full digest 두 번 일치 후 orchestration reset을 실행한다.
6. manifest에 고정된 primary/scoped binding만 SQLite exact transaction CAS로 제거한다.
7. collection 단계에서 clean sealed HEAD로 빌드해 hash/VCS seal한 bundle-private executor를 사용해, 각 non-done IssueOps record의 raw/canonical digest와 repository binding 0을 같은 state-root lock 안에서 재검증하는 CAS mode로 force-release한다. live executor 환경의 state/root/daemon/worker 경로는 명시적으로 고정한다.
8. non-canonical local ref를 `git update-ref -d <ref> <expected-old-oid>`로 삭제한다.
9. fetch/push URL이 각각 singleton이며 동일 canonical authority인지 다시 확인한 뒤 non-canonical remote ref를 `git push --force-with-lease=<ref>:<expected-oid> <sealed-explicit-push-url> :<ref>`로 삭제한다.
10. state artifact의 검증된 backup copy를 유지한 채 original을 bundle의 별도 `state/relocated/` target으로 move하고 original path absence와 양쪽 hash를 확인한다.
11. Git/IssueOps/Orca/state/doctor 전체 inventory를 처음부터 다시 검증한다.

Remote delete 전에 server ref OID를 다시 읽는다. expected OID가 바뀌었으면 해당 ref를 삭제하지 않는다. broad glob, unresolved env, `git branch -D`의 moving-name 판단, plain `git push --delete`를 사용하지 않는다.

### 9.5 Durable journal과 crash recovery

`journal.jsonl`은 mutation마다 다음 상태를 append하고 파일과 parent directory를 동기화한다.

```text
planned -> started -> completed -> verified
```

각 row는 operation ID, resource kind, exact identity, expected digest/OID, command class, result digest, timestamp를 포함한다. secret/raw message body는 포함하지 않는다.

재개 규칙:

- `verified` operation은 다시 실행하지 않는다.
- `started`에서 멈춘 operation은 current state를 읽는다. exact target이 이미 없고 manifest identity와 journal이 일치하면 `completed/verified`를 append한다.
- target이 남아 있고 expected identity가 그대로면 같은 idempotent operation만 재개한다.
- target identity가 바뀌었거나 새 owner가 생겼으면 중단한다.
- Orca reset은 이미 zero면 verified로 전진하고, nonzero/unknown이면 자동 재호출하지 않는다.
- rollback 가능한 Git/SQLite 증거는 bundle에 남기지만 정상 recovery 전략은 부분 rollback이 아니라 forward completion이다.

## 10. 구현 범위

최소 변경은 다음 경계로 제한한다.

- pure operational-health model과 table-driven tests
- 현재 Orca adapter의 complete task/gate/message-presence read projection과 completeness tests
- top-level doctor request의 invocation-scoped preserve flags와 existing result projection
- IssueOps stale scan이 같은 cycle finding을 소비하는 wiring
- stability audit가 top-level doctor 결과를 gate로 사용하는 wiring
- response contract/golden과 운영 문서의 필요한 최소 갱신

새 public cleanup command, persistent preserve state, background reaper, auto-fix, generic orchestrator interface는 추가하지 않는다. 구현 중 50줄짜리 pure classifier가 registry/factory 계층으로 불어나면 설계 위반으로 간주하고 단순화한다.

## 11. TDD와 검증 설계

### 11.1 첫 RED

첫 production edit 전에 다음 exact named regression을 추가하고 의도한 assertion으로 실패하는 terminal exit를 보존한다.

```text
TestClassifyOperationalHealthRejectsBoundCycleWithoutFreshHeartbeat
```

Fixture:

- non-done IssueOps cycle
- exact primary/scoped binding
- exact ready Orca task
- active lease/fresh heartbeat 없음

Expected: `operational_dead_owner`, overall unhealthy.

### 11.2 Pure classifier table

최소 cases:

- fresh claimed heartbeat + exact worktree/terminal/task/dispatch -> live/healthy
- stale/missing heartbeat with exact binding/resources -> dead owner/unhealthy
- exact invocation-preserved planning cycle -> preserved/healthy
- persistent binding without preserve flag -> unhealthy
- duplicate owner or resource ID -> unknown/unhealthy
- incomplete/truncated task/terminal/worktree inventory -> unknown/unhealthy
- ready task with completion metadata -> task residue/unhealthy
- done cycle with remaining resource -> residue/unhealthy
- one unmatched terminal/worktree/task/gate/message -> corresponding residue
- local and remote branch residue -> one aggregated branch finding
- unknown phase/status -> fail closed
- injected time boundary at exactly/just beyond 15 minutes

### 11.3 Wiring tests

- doctor stays read-only and returns the same core finding in JSON/text projection.
- repeated `--preserve-cycle` and `--preserve-terminal` parse exact nonempty IDs and do not write state.
- no new MCP tool or cleanup command appears in command/tool catalog.
- stale scan uses the same dead-owner finding but does not promote heartbeat-only evidence to auto-release.
- fake Orca runner proves all-task count equality, task semantic projection, gate projection, and bounded inbox zero/nonzero behavior.
- collector failures are unhealthy, not empty-success.
- response-contract and usage goldens change only for the intended doctor fields/flags.

### 11.4 Cleanup verification

실제 global reset/delete는 unit test에서 실행하지 않는다. Cleanup 전후에는 manifest와 journal validator를 사용하고 모든 외부 command의 exit code와 readback을 기록한다. 부분 package output이나 yielded process는 성공 증거가 아니다.

최종 all-or-nothing verification run은 다음을 포함한다.

```bash
go mod tidy
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

마지막 stability audit는 새 binary로 실행하고 daemon, hooks, MCP, worker/state, stale socket, zombie/orphan process를 다시 검사한다.

## 12. 완료 기준

같은 마지막 verification window에서 다음을 모두 증명해야 한다.

- source checkout 하나만 남고 branch는 canonical `main`이다.
- Git worktree와 Orca worktree는 source/main 하나만 남는다.
- local branch와 `origin` remote branch는 canonical `main`만 남는다.
- current operator terminal만 남고 그 exact handle이 유지된다.
- Orca task, message, gate, dispatched task가 모두 0이다.
- 실행 manifest에 고정된 전체 IssueOps record(감사 시점 20개)가 모두 `done`이고 non-done 0, 새 record 0, stale session binding 0이다.
- state-root recovery artifact 5개는 original path에서 사라지고 external bundle에서 hash가 일치한다.
- external bundle mode, manifest digest, Git bundle verify, SQLite integrity/restore evidence가 유효하다.
- `git status`가 clean이고 `HEAD == refs/remotes/origin/main`이다.
- `agent-harness doctor --repo . --preserve-terminal <current> --json`이 `healthy=true`이며 operational issue가 없다.
- stability audit가 새 doctor gate를 포함해 통과한다.
- full test, race, vet, build, contract golden, quick/full self-verify 95점 gate가 한 번의 최종 all-or-nothing run에서 통과한다.
- 임시 script/artifact, 살아 있는 test process, stale daemon/socket, zombie/orphan process가 없다.

이 중 하나라도 indirect evidence이거나 확인 불가능하면 전체 목표는 완료로 판정하지 않는다.
