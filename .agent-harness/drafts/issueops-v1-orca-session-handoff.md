# IssueOps v1: Orca 선택형 실행 인계와 교체 가능한 격리 worktree write lease 재설계

## 한 줄 결론

IssueOps 실행 계층을 schema v1으로 다시 만들고, `lifecycle ID ↔ canonical isolated worktree ↔ logical owner slot`을 1:1로 고정하되 native session은 영구 소유자가 아니라 원자적으로 교체 가능한 write lease holder로 취급한다. Orca가 준비되면 coordinator가 별도 owner session으로 한 번만 인계하고 종료하며, Orca가 없으면 같은 main session이 새 격리 worktree의 direct holder가 되어 구현한다. 어느 모드에서도 source checkout이나 다른 worktree에 쓰지 않지만, read-only 관찰은 허용한다.

## 배경과 문제

현재 구현은 IssueOps 계획, worktree 준비, Orca 외부 mutation journal, coordinator/owner session 봉인, context acknowledgement, heartbeat, publication, cleanup, runtime rebind, owner restart, WIP seal, legacy migration을 하나의 누적 상태기계에 계속 추가했다. 그 결과 안전 장치가 서로를 선행 차단하면서 다음과 같은 실제 교착이 반복됐다.

- 다중 cycle의 source 공유가 safe control-plane/read까지 막았다(#47). pre-dispatch 취소는 session binding과 taskless cleanup 결합으로 복구되지 않았다(#22).
- worktree가 있어도 sealed coordinator와 terminal inventory 불일치가 handoff를 막았다(#26). owner는 acknowledgement 부재로 `owner_orienting`에 고착되고 Stop cleanup은 무한 재진입했다(#65).
- fresh session의 modification/rebind/closed-handoff 인수도 exact-session 봉인이 복구 경로를 먼저 차단했다(#68).

이번 변경은 위 증상을 하나씩 예외 처리하지 않는다. coordinator 생존, 최초 session identity, append-only attempt history를 mutation authority의 전제로 삼는 모델을 제거하고, 단일 현재 lease와 항상 실행 가능한 교체 경로로 다시 설계한다.

## 검증된 현재 상태

재감사 기준은 [`main@5515347`](https://github.com/m16khb/agent-harness/tree/5515347546babc1a5ae3b561cad6ba3066565a71)이며 구현 branch의 HEAD와 `origin/main`이 모두 이 commit과 일치한다. 이전 감사 기준 `1623a9d`부터 현재 HEAD까지 production code diff는 0건이고, 추가된 파일은 이 설계 초안·remote score 입력·owner prompt 3개뿐이다. 아래 `file:line`과 blast radius는 현재 HEAD에서 CodeGraph와 `rg`로 다시 확인했다.

### 현재 코드가 요구와 어긋나는 지점

1. Orca `auto` probe 실패 시 fallback이 mode/code/warning/workspace/handoff를 지우고, CLI는 worktree를 만들지 않은 채 `git worktree add ...` 문구만 출력한다 (`internal/core/issueops/issueops_handoff_prepare.go:348-421`, `cmd/harness/issueopscli/worktreecmd/worktree.go:174-227`).
2. session binding은 native session identity 없이 repo 또는 repo+cycle key만 저장하며 primary row를 overwrite한다 (`internal/core/issueops/session/session.go:1-204`).
3. 준비·owner mutation은 최초 sealed `host + session_id + optional agent_id` 완전 일치를 요구해 같은 canonical worktree의 fresh session도 거부한다 (`internal/core/issueops/issueops_actor.go:37-106`, `internal/core/issueops/handoff/state.go:97-124`).
4. claim은 `owner_orienting`의 다른 session을 거부하지만 교체용 최소 CAS가 없다 (`internal/core/issueops/handoff/state.go:110-119`).
5. lifecycle guard는 exact-session 불일치 시 mutation을 막고 다시 read-only `resume`을 안내해 봉인을 바꾸지 못한다 (`internal/core/lifecycle/lifecycle_handoff_guard.go:138-313`).
6. cross-worktree observation grammar는 `cat/head/tail/ls/find/stat/file` 같은 일반 reader도 허용하지 않는다 (`internal/core/commandparse/issueops.go:186-230`).
7. owner 모델은 Codex `gpt-5.6-terra/high`, Claude `opus`로 하드코딩되고 GJC도 launch path에 있다 (`internal/port/orca.go:8-13`, `internal/core/issueops/handoff/launch_profile.go:11-22`, `internal/adapter/orca/client.go:844-880`).
8. schema v9 record가 legacy workspace/handoff와 ownership ledger를 함께 보유한다 (`internal/core/issueops/model/types.go:401-707`).

### 정량 근거

- 감사 범위 합계는 Go 60,086 LOC(제품 31,203+테스트 28,883), lint baseline 202건(`cyclop` 169, `gocognit` 28, `funlen` 3, `gocyclo` 2)이다.
- 대표 cognitive complexity는 `ValidateOwnershipLedger` 82, `PrepareIssueOpsHandoffWorktree` 66, `requireCancellationQuiescence` 105, `reconcileIssueOpsHandoff` 70이다.
- 현재 user state readback: IssueOps row 31건(schema 7: 4, schema 8: 24, schema 9: 3), session row 35,984건. schema 9에는 이 작업의 control cycle, 기존 #65 cycle, branch 없는 active row가 각각 1건 있다. 이 수치는 제품 결함 건수가 아니라 legacy/runtime residue 규모이며, 기존 namespace를 v1 authority로 재사용하지 않아야 하는 근거다.
- GJC/Reasonix 문자열은 제품 Go 파일 30개와 테스트 Go 파일 18개에 걸쳐 있고, install/update/inspect/hook/conformance/Orca launch/golden까지 first-party 표면에 남아 있다.
- 현재 HEAD의 focused baseline은 production edit 전부터 실패한다. 대표 실패는 `TestWorktreePrepareDefinitiveStartFailureClearsJournalAndAutoFallsBack`, `TestSupervisedHandoffCyclesRetainsOnlyCurrentNonterminalMissingWorktreeAuthority`, `TestIssueOpsContractChangeFeedbackBlocksPRUntilIssueUpdateRecorded`, `TestRunHookStopBoundsOwnershipCleanupRelay`이며, schema v9 ownership ledger 도입 뒤 legacy fixture의 missing `cycle_state` 정규화와 active/paused/done 검증이 충돌한다. 이 실패를 새 v1 RED로 계산하지 않고 Task 0 evidence에 별도 보존하며, 최종 full gate는 모두 GREEN이어야 한다.

### 실제 도구 확인

- Orca `/Users/m16khb/.orca-relay/bin/orca` handshake `0.1.0+15e8b8ca277b`; runtime/graph/repo/worktree base ready.
- Codex `0.145.0`의 model/reasoning/cd/resume, Claude `2.1.217`의 model/effort/session/resume/background surface를 installed CLI에서 확인했다.
- `orca --version`은 값을 주지 않으므로 readiness는 executable+JSON envelope+runtime/graph/capability/repo resolution으로 판정한다.

## IssueOps 분할 gate 결정

단일 이슈 SSOT를 유지한다. schema/store, lease, hook, CLI/MCP, native docs/golden은 함께 바뀌어야 하는 하나의 authority contract라 분할하면 v1 writer와 legacy guard의 dual authority를 다시 만든다. Task 0~10을 한 canonical worktree/owner slot의 순차 RED/GREEN·commit gate로 사용하고, 별개로 ship 가능한 발견만 사용자 승인 후 후속 cycle로 보낸다.

## 확정 요구사항

### 공통

- 원격 이슈 하나가 문제, 설계, 구현 순서, acceptance criteria, 검증 명령의 SSOT다. provider-native child issue로 계획을 분산하지 않는다.
- 한 lifecycle은 정확히 하나의 issue branch와 하나의 canonical isolated worktree를 가진다.
- 한 lifecycle의 logical owner slot에는 동시에 한 write lease holder만 존재한다.
- 한 session은 동시에 하나의 lifecycle/worktree write lease만 가질 수 있다.
- session은 source checkout과 다른 worktree를 읽을 수 있다. write/build/test/format/generate/commit/push 같은 mutation은 lease의 canonical worktree에만 가능하다.
- source checkout은 planning/remote/provision control-plane일 뿐 구현 대상이 아니며, 구현은 record에 link한 새 sibling worktree에서만 한다.
- owner는 구현, Turing 증거, AI-slop 정리, 검증, draft PR/MR 생성까지 수행한 뒤 execution lease를 release하고 종료한다.
- PR/MR 이후 검토와 추가 지시는 human-in-the-loop다.
- hook은 빠른 관찰·분류·allow/block·정확한 escape 안내만 수행한다. workflow 전이, retry, polling, 외부 도구 호출, cleanup, 메시지 전송을 수행하지 않는다.

### Orca가 준비된 경우

- `1..N` coordinator가 N개 lifecycle/branch/worktree를 병렬 준비하며 cycle 간 coordinator/mailbox/terminal/task/dispatch를 재사용하지 않는다.
- coordinator는 probe, provision/link, packet seal, owner launch와 dispatch receipt persist를 한 번 수행한다. verified dispatch 뒤에는 기다리거나 monitor하지 않고 종료할 수 있다.
- owner session은 prompt에 노출되지 않는 one-time claim-token file과 exact lifecycle/worktree/generation으로 lease를 claim한다.
- Orca identity는 launch/reconcile locator일 뿐 mutation authority가 아니다. 권위는 현재 lease generation + native session + canonical mutation root다.

### Orca가 없거나 준비되지 않은 경우

- pre-mutation probe 실패는 `direct`이며 handoff/worker/Orca identity 없이 같은 main session이 새 linked worktree의 generation-1 holder가 된다.
- source에는 쓰지 않고 모든 구현 target/cwd를 canonical root로 고정한다. active direct lease가 있으면 두 번째 lease나 다른 cycle control-plane write를 거부하며 병렬 cycle은 별도 main session이 담당한다.

### session 종료·교체

- 1:1은 최초 session이 아니라 `lifecycle ↔ canonical worktree ↔ logical owner slot`이며 holder는 교체 가능한 현재 값이다. coordinator 종료는 lease/claimability에 영향이 없다.
- holder가 clean release하면 같은 worktree에서 다음 session이 새 generation을 claim한다.
- crash takeover는 exact lifecycle/worktree/branch/HEAD/index/content와 generation을 preview해 `revoking`으로 CAS한다. old 신규 mutation을 막고 process/in-flight 종료와 최종 fingerprint를 검증한 뒤에만 `claimable`로 finalize한다.
- 새 session은 quiescence 이후의 같은 dirty worktree를 그대로 이어받으므로 WIP 복사, 새 worktree, hidden Git ref, append-only attempt ledger가 필요 없다.
- claim/replace/reconcile/release escape는 현재 holder가 아니어도 exact context에서 실행할 수 있어야 한다. “권한이 없어서 권한을 얻는 명령도 거부”하는 circular deny를 금지한다.

## 용어와 불변식

| 용어 | 의미 |
|---|---|
| lifecycle ID | IssueOps cycle의 유일하고 안정적인 ID. 모든 control-plane/mutation 명령이 exact ID를 요구한다. |
| source checkout | main coordinator가 계획·remote issue·branch/worktree control-plane을 수행하는 checkout. 구현 파일 mutation 금지. |
| canonical worktree | lifecycle에 1:1로 link된 유일한 구현 root. 동일 branch의 임의 checkout은 권위가 아니다. |
| logical owner slot | lifecycle이 가진 단 하나의 writer 자리. 별도 영구 session ID가 아니다. |
| lease generation | holder 교체 때 증가하는 fencing token. schema version과 무관하다. |
| holder | 현재 generation에서 mutation 가능한 Codex/Claude native session. |
| observation | repository bytes, metadata, status, diff, logs를 바꾸지 않는 읽기. owner 선택 전에도 허용한다. |
| mutation | 파일/인덱스/ref/state/remote/process output tree를 변경할 수 있는 작업. build/test/generate/format도 assigned worktree에서만 허용한다. |

다음 불변식은 코드와 property/regression test로 고정한다.

1. `cycle.lifecycle_id`, linked `worktree`, logical `owner slot`은 1:1이다.
2. `active lease count per lifecycle <= 1`이다.
3. `active lease count per native session <= 1`이다.
4. holder mutation의 모든 resolved target과 execution cwd는 canonical worktree 내부다.
5. observation은 active cycle 수, coordinator 생존, holder identity와 무관하게 수행할 수 있다.
6. session identity는 현재 lease holder 비교에만 사용하며 coordinator 생존 증명이나 historical ownership key로 사용하지 않는다.
7. 외부 mutation 가능성이 생긴 뒤에는 Orca→direct fallback을 하지 않는다.
8. 모든 nonterminal deny에는 실제로 allow되는 exact next command가 하나 존재한다.
9. Stop/hook 한 번은 유한 시간에 반환하고 같은 event를 재주입해 상태 변화 없는 loop를 만들지 않는다.
10. source/main에 active cycle이 여러 개 있어도 서로 다른 exact lifecycle의 control-plane은 논리적으로 직렬화하지 않는다.

## 선택한 아키텍처

### 비교한 접근

| 접근 | 결과 | 이유 |
|---|---|---|
| A. v9 append-only attempt ledger를 계속 확장 | 기각 | 기존 session 봉인과 coordinator recovery 전제를 유지해 교착 표면이 남고, migration/WIP seal/attempt 소비자까지 계속 증가한다. |
| B. v9에 `restart-owner` 예외만 추가 | 기각 | 같은 worktree의 holder 교체 문제를 새 worktree/WIP 복사 문제로 바꾸며, 기존 authority 함수마다 historical/current attempt 분기를 추가한다. |
| C. v1 current-lease 모델로 교체 | 선택 | authority를 단 하나의 현재 generation으로 축소하고, same-worktree takeover와 source-independent claim을 직접 표현한다. |

### 컴포넌트 경계

```text
IssueOps planning/remote/phase
        |
        v
Execution v1 core  <----> SQLite v1 namespace (cycle row + current lease)
   |          |
   |          +----> Hook policy projection (pure/read-only)
   |
   +---- WorktreeProvisioner port
   |       |- Git provisioner (direct)
   |       `- Orca provisioner (orca)
   |
   `---- OwnerLauncher port
           `- Orca Codex/Claude launcher only
```

- core는 Orca JSON, terminal handle, Codex/Claude argv를 알지 않는다.
- Git과 Orca adapter는 동일한 canonical `WorkspaceReceiptV1`을 반환한다.
- Orca adapter는 설치/업데이트를 하지 않고 실제 CLI capability를 probe한 뒤 literal argv만 실행한다.
- host adapter는 Codex/Claude hook payload를 공통 `NativeActorV1`로 정규화한다.
- hook은 state projection만 읽고 side effect를 만들지 않는다.

## schema v1 상태 모델

아래는 구현자가 임의로 필드를 늘리지 않도록 고정한 최소 논리 모델이다. JSON field 이름과 enum은 CLI/MCP golden에서 고정한다.

```go
const IssueOpsSchemaVersion = 1

type ExecutionModeV1 string // direct | orca
type LeaseStatusV1 string   // claimable | active | revoking | released

type ExecutionV1 struct {
    Mode       ExecutionModeV1        `json:"mode"`
    Workspace  WorkspaceV1            `json:"workspace"`
    Lease      WriteLeaseV1            `json:"lease"`
    Orca       *OrcaBindingV1          `json:"orca,omitempty"`
    Pending    *ExternalIntentV1       `json:"pending,omitempty"`
    Completion *ExecutionCompletionV1  `json:"completion,omitempty"`
    Failure    *ExecutionFailureV1     `json:"failure,omitempty"`
}

type WorkspaceV1 struct {
    SourceRoot   string `json:"source_root"`
    Root         string `json:"root"`
    Branch       string `json:"branch"`
    BaseHead     string `json:"base_head"`
    Driver       string `json:"driver"` // git | orca
    LinkedAt     string `json:"linked_at"`
}

type WriteLeaseV1 struct {
    Generation       uint64          `json:"generation"` // 최초 1, 교체 때 +1
    Status           LeaseStatusV1   `json:"status"`
    Holder           *NativeActorV1  `json:"holder,omitempty"`
    ClaimTokenSHA256 string          `json:"claim_token_sha256,omitempty"`
    ClaimedAt        string          `json:"claimed_at,omitempty"`
    ReleasedAt       string          `json:"released_at,omitempty"`
    ReplacedAt       string          `json:"replaced_at,omitempty"`
    ReplacementReason string         `json:"replacement_reason,omitempty"`
}

type NativeActorV1 struct {
    Host           string                    `json:"host"`       // codex | claude
    SessionID      string                    `json:"session_id"`
    AgentID        string                    `json:"agent_id,omitempty"`
    SessionProcess *NativeProcessReceiptV1   `json:"session_process,omitempty"`
}

type NativeProcessReceiptV1 struct {
    PID              int    `json:"pid"`
    StartedAt        string `json:"started_at"` // PID reuse fence
    Executable       string `json:"executable"`
}

type OrcaBindingV1 struct {
    RuntimeID         string `json:"runtime_id"`
    RepoID            string `json:"repo_id"`
    WorktreeID        string `json:"worktree_id"`
    WorktreeInstanceID string `json:"worktree_instance_id,omitempty"`
    OwnerHost         string `json:"owner_host"`  // codex | claude
    OwnerModel        string `json:"owner_model"`
    OwnerEffort       string `json:"owner_effort,omitempty"`
    TaskID            string `json:"task_id"`
    DispatchID        string `json:"dispatch_id"`
    TerminalPTYID     string `json:"terminal_pty_id,omitempty"`
}

type ExternalIntentV1 struct {
    OperationID string `json:"operation_id"`
    Kind        string `json:"kind"` // worktree_create | owner_launch | dispatch
    Marker      string `json:"marker"`
    StartedAt   string `json:"started_at"`
}

type ExecutionCompletionV1 struct {
    FinalHead         string   `json:"final_head"`
    TuringReportPath  string   `json:"turing_report_path"`
    Verification      []string `json:"verification"`
    RemoteArtifactURL string   `json:"remote_artifact_url"`
    CompletedAt       string   `json:"completed_at"`
}

type ExecutionFailureV1 struct {
    OperationID string `json:"operation_id,omitempty"`
    Code        string `json:"code"`
    Message     string `json:"message,omitempty"` // bounded + redacted
    At          string `json:"at"`
}
```

추가 규칙:

- lifecycle ID 자체가 logical owner slot ID이므로 중복 `owner_id`를 만들지 않는다.
- current holder만 저장한다. historical attempt 배열, ownership epoch, coordinator mailbox/session, orientation, heartbeat, WIP seal, cleanup receipt, worker-done projection을 새 모델에 넣지 않는다.
- `claimable`은 holder 없음 + 64-hex token hash, `active`는 holder 있음 + token hash 없음, `released`는 holder/token hash 모두 없음이어야 한다. `revoking`은 권한이 제거된 이전 holder를 quiescence 확인용으로만 보존하고 token hash는 없어야 한다. `status == active`만 mutation authority를 가지며 다른 조합은 invalid다.
- `session_process`는 CLI/hook parent chain에서 검증한 PID, process start identity, executable receipt다. PID 단독으로 liveness를 판단하지 않으며 authority도 아니다. first-party adapter가 receipt를 만들 수 없으면 lease 획득을 실패시켜 종료 후 안전 takeover를 증명할 수 없는 holder를 만들지 않는다.
- claim token 원문은 상태·prompt·Orca task/message·로그·hook output에 남기지 않는다. worktree의 generation별 deterministic ignored runtime 경로에 mode `0600` file로 한 번 전달하고, 상태에는 SHA-256만 저장하며 claim 성공 시 hash를 비우고 file을 소비·삭제한다. CAS 뒤 file 삭제 전에 crash가 나서 inert file이 남아도 비워진 hash와 active generation 때문에 재사용할 수 없어야 한다.
- terminal handle은 runtime-scoped이므로 durable authority field가 아니다. 필요할 때 worktree/PTY로 다시 resolve한다.
- external intent는 동시에 하나만 허용한다. intent를 먼저 저장하고 외부 mutation을 최대 한 번 호출한 뒤 receipt를 CAS 저장한다.
- completion은 final HEAD, Turing report path, verification, remote artifact URL만 저장한다. cleanup은 일반 IssueOps/human 단계이며 execution Stop gate가 아니다.
- 별도 `lease_holder_v1` reverse index는 `host + session_id + optional agent_id → lifecycle_id + generation` 하나만 저장한다. lifecycle row와 같은 SQLite transaction에서 설정/삭제해 한 native session의 active lease가 둘이 되지 않게 한다. 이것은 current active holder index이며 repo-primary/scoped session binding이나 historical session registry가 아니다.

## 상태 전이

| 현재 | 명령/사건 | 다음 | writer |
|---|---|---|---|
| execution 없음 | `execution prepare --mode direct` confirm | active generation 1 | 같은 main native session |
| execution 없음 | `execution prepare --mode orca` intent | pending worktree/launch | 없음 |
| pending | exact external receipt CAS | claimable generation 1 | 없음 |
| claimable | exact token `execution claim` | active same generation | claim session |
| active | `execution release` | released same generation | 없음 |
| active | `execution replace --revoke` preview+confirm | revoking generation + 1 | 없음; 이전 generation 즉시 stale |
| revoking | `execution replace --finalize` preview+confirm + quiescence proof | claimable same generation | 없음 |
| released/claimable | `execution replace --reseed` preview+confirm | claimable generation + 1 | 없음; 기존 token은 폐기 |
| claimable generation N | 새 token `execution claim` | active generation N | 새 session |
| active | verified PR/MR + `execution complete` | released + completion | 없음 |
| pending/ambiguous external result | `execution reconcile` | pending, claimable 또는 recovery failure | 없음 |

금지 전이:

- active holder가 둘이 되는 전이.
- revoking에서 quiescence proof 없이 claimable/active로 가는 전이.
- lifecycle/worktree/branch를 바꾸는 replace.
- coordinator session identity를 요구하는 claim/replace/reconcile.
- external mutation 이후 direct fallback.
- completed execution에서 자동 merge/cleanup.
- source checkout 또는 foreign worktree를 mutation root로 선택하는 전이.

## 두 실행 시퀀스

### 1. Orca ready

1. coordinator가 plan/design을 확정하고 `prepare --mode auto`의 read-only probe로 Orca ready를 확인한다.
2. adapter가 issue branch+sibling worktree를 marker로 한 번 create/adopt하고, core가 path/branch/base/common-dir/issue link를 검증·저장한다.
3. issue digest, lifecycle/worktree/generation/AC/docs/verification을 bounded packet으로 봉인한다.
4. explicit `codex|claude` model/effort로 owner를 launch하고 dispatch receipt+token hash를 CAS 저장한다. token 원문은 deterministic file에만 두며 여기서 coordinator 역할은 끝난다.
5. owner가 packet/lease를 검증·claim하고 그 worktree에서 TDD/Turing/AI-slop clean/full verification과 draft PR/MR을 수행한다.
6. completion이 lease를 release하면 owner도 종료하고 human review로 넘긴다.

### 2. Orca missing/unready

1. mutation 전 auto probe의 missing/unready를 diagnostic과 함께 `direct`로 결정한다.
2. Git adapter가 issue branch+sibling worktree를 생성·link하고 같은 main session을 generation-1 holder로 기록한다. handoff/worker/Orca identity는 없다.
3. main session은 canonical root에서 구현·검증·draft PR/MR·completion까지 수행하고 source/다른 worktree는 observation-only로 둔다.

## Orca fallback과 복구 경계

- `auto`는 mutation 전 probe에서만 `direct`를 선택한다. 이후 외부 mutation 가능성이 생기면 `orca`로 고정한다.
- timeout/invalid JSON/persist failure로 결과가 모호하면 `pending.operation_id + marker`로 exact inventory를 reconcile한다.
- 후보 1개는 검증 후 adopt, 0개는 adapter가 `invoked=false`를 증명한 경우에만 동일 operation을 한 번 재시도, 복수 후보는 recovery failure로 종료한다.
- reconcile은 한 단계만 처리하고 반환하며 polling하지 않는다. fresh source session도 exact lifecycle ID로 실행할 수 있다.

## lease claim·release·교체 계약

### 정상 claim

- lifecycle ID, canonical worktree cwd, branch, generation, mode `0600` one-time token file의 내용, native host/session을 모두 검증한다.
- token digest가 일치하면 holder를 CAS 설정하고 token을 지운다.
- claim 성공 후 token file을 삭제한다. 상태 CAS 성공과 file 삭제 사이에 crash가 나도 같은 holder의 retry만 idempotent success가 되고 다른 session은 token 재사용에 실패해야 한다.
- 같은 session의 동일 요청은 idempotent success, 다른 session의 동일 generation claim은 conflict다.

### clean release

- 현재 holder/session/generation/cwd가 모두 일치해야 한다.
- dirty worktree를 허용한다. release는 파일을 이동·commit·stash하지 않고 holder만 비운다.
- release는 같은 generation을 `released`로 바꾸고 holder/token hash를 비운다. 이후 session은 released lease를 직접 claim하지 않고 holder가 없음을 재검증하는 `replace --reseed` preview/confirm으로 generation을 정확히 1 증가시킨 뒤 claim한다. 아직 claim하지 않은 terminal/token을 잃은 `claimable`도 같은 reseed 경로를 쓰며 기존 token을 원자적으로 폐기한다. generation 증가는 오직 replace revoke/reseed confirm에서만 일어난다.

### crash takeover

교체는 **revoke와 finalize를 분리**한다. generation fencing만으로 hook을 이미 통과한 old command를 중단할 수 없으므로, 즉시 새 holder를 claimable로 만드는 강제 takeover는 금지한다.

1. 새 session은 source 또는 canonical worktree에서 `execution replace --preview`를 실행한다.
2. preview는 lifecycle/worktree/branch/HEAD/index/working-tree content digest/current generation/current holder와 session process receipt, mode별 Orca liveness를 읽어 revocation fingerprint를 반환한다.
3. `execution replace --revoke --confirm --expected-generation --inventory-fingerprint --reason`은 lock 안에서 같은 값을 재검증하고 generation을 정확히 1 증가시키며 status를 `revoking`으로 바꾼다. token은 만들지 않고, old holder identity는 quiescence 진단용으로만 남긴다.
4. 이 CAS 이후 old generation의 새 mutation은 stale이고, `revoking`에는 writer가 없으므로 new session도 claim/mutation할 수 없다. 이미 hook을 통과한 old command는 끝날 수 있지만 동시에 새 writer가 생기지 않는다.
5. 사용자는 old session을 정상 종료한다. 종료가 불가능하면 exact PID/start identity와 Orca terminal/task inventory를 보고 해당 process/task만 명시적으로 중단한다. timeout 경과나 “아마 죽었음”만으로 다음 단계로 가지 않는다.
6. `execution replace --finalize-preview`는 다음을 모두 확인하고 quiescence fingerprint를 반환한다.
   - old native session process의 PID+start identity가 더 이상 live가 아님.
   - 그 process의 descendant, Orca owner terminal/task, canonical worktree를 cwd로 쓰거나 그 안의 writable file을 연 process가 없음.
   - background/detached mutation 명령이 허용된 기록이 없음. mutation shell의 `&`, `nohup`, detached runner는 평상시 guard에서 금지한다.
   - canonical root/common-dir/branch는 동일하고, 현재 HEAD, index bytes, tracked binary diff, sorted untracked path·mode·content digest를 새로 계산할 수 있음.
7. `execution replace --finalize --confirm --expected-generation --quiescence-fingerprint`는 lock 안에서 liveness/process inventory와 worktree fingerprint를 다시 계산한다. 모두 같을 때만 old holder를 비우고 one-time token hash를 만들며 같은 generation을 `claimable`로 바꾼다.
8. 새 session이 token으로 claim한다. revoke 뒤 old in-flight command가 끝내 남긴 변경은 finalize fingerprint에 포함되고 그대로 인계된다. revoke/finalize 명령 자체는 worktree bytes/index/HEAD/branch를 변경하지 않는다.

live holder를 교체하려면 clean release가 우선이다. release를 할 수 없을 때도 revoke까지는 명시적 human confirm으로 가능하지만, live/unknown process를 무시하는 unsafe finalize override는 제공하지 않는다. `revoking`을 만든 session이나 최초 coordinator가 종료돼도 authority는 그 session에 묶이지 않는다. 다른 fresh session이 exact lifecycle ID로 `status → replace --finalize-preview → replace --finalize`를 실행할 수 있다. 각 deny는 현재 남아 있는 exact process/task와 다음 허용 명령을 반환하므로 “기존 owner가 없어서 교체 명령도 deny”하는 순환은 없다.

## read/write 권한 판정

권한 판정 순서는 다음으로 고정한다.

1. 요청의 side-effect class를 먼저 판정한다.
2. observation이면 owner/cycle 선택 전에 허용한다.
3. typed `status/claim/release/replace/reconcile`이면 exact lifecycle ID로 한 record를 선택하고 해당 operation의 좁은 조건만 검사한다.
4. mutation이면 모든 cwd/source/destination/path/ref/remote target을 resolve한다.
5. exact active holder/session/generation이며 모든 mutation target이 canonical worktree 내부일 때만 허용한다.
6. 불명확한 shell은 mutation 가능으로 취급해 차단하되, 동일 목적의 허용 reader 또는 exact escape 명령을 안내한다.

### 반드시 허용할 observation

- host-native Read/Glob/Grep/Search/List 및 CodeGraph.
- `pwd`, `cat`, `head`, `tail`, `wc`, read-only `sed`, `rg`, `ls`, mutation flag 없는 bounded `find`, `stat`, `file`.
- `git -C <source-or-any-linked-worktree> status|diff|log|show|rev-parse|branch --show-current`에서 output/exec hook을 유발하지 않는 형식.
- exact-ID `issueops status`, `issueops resume --bind=false`, `execution status`, `execution replace --preview`, `execution replace --finalize-preview`, `execution reconcile --preview`.
- 다른 worktree 파일·status·diff·log를 읽는 위 형식.

### 반드시 차단할 foreign/source mutation

- Write/Edit/ApplyPatch/NotebookEdit 및 filesystem write MCP.
- `rm`, `mv`, `cp` destination, `mkdir`, `touch`, chmod/chown, redirect, in-place edit.
- `git add|commit|push|checkout|switch|reset|rebase|merge|worktree`.
- formatter, generator, package install, build, test, benchmark처럼 worktree 또는 cache/artifact를 변경할 수 있는 실행.
- assigned root 안이라도 `&`, `nohup`, daemonize/detach처럼 holder session 종료 뒤 mutation을 계속할 수 있는 background 실행. 장기 검증이 필요하면 holder가 foreground에서 완료를 관찰한다.
- shell substitution/unknown wrapper로 target을 정적으로 확정할 수 없는 명령.

source coordinator의 exact IssueOps/remote/worktree control-plane은 repository content mutation과 별도 분류한다. 다른 active cycle이 존재한다는 이유만으로 별도 source session의 새 exact cycle scoring, issue create, branch/worktree provisioning을 막지 않는다. 단, 이미 active direct lease를 가진 session은 자기 cycle 밖 control-plane write를 할 수 없고 observation만 가능하다.

## hook liveness 설계

- PreToolUse는 state read + pure decision만 수행하고 5초 hook timeout보다 훨씬 작은 bounded path를 유지한다.
- PostToolUse/Stop을 lease heartbeat나 workflow 전이의 필수 조건으로 사용하지 않는다.
- revoke/finalize의 state CAS와 process/worktree inventory는 사용자가 명시적으로 실행한 typed CLI/MCP control-plane command가 담당한다. hook이 reservation을 쓰거나 polling하지 않는다.
- Stop은 execution cleanup이나 owner 선택을 요구하지 않는다. PR/MR 이후 owner는 정상 종료할 수 있다.
- `stop_hook_active=true`, 사용자의 종료 의사, 동일 fingerprint 재진입은 항상 no-op이다.
- deny reason은 `code`, `lifecycle_id`, `expected_root`, `current_generation`, `next_command`를 구조화해 반환한다.
- `next_command`는 해당 state에서 실제 allowlist를 통과하는지 table/property test로 검증한다.
- 여러 cycle이 source를 공유해도 observation/control-plane은 record 선택 전에 허용하고, mutation만 exact cycle/target을 요구한다.
- `revoking` deny는 최초 coordinator나 revoke initiator identity를 요구하지 않고, 남아 있는 process/task와 exact `replace --finalize-preview` next command를 반환한다. quiescence가 아직 아니면 같은 명령을 자동 재시도하지 않고 human이 표시된 session/process를 종료한 뒤 다시 실행한다.

## host와 model 계약

- first-party host enum은 `codex|claude`뿐이다.
- Orca owner launch는 `--owner-host`, `--owner-model`, host가 지원하는 `--owner-effort`를 명시적으로 요구한다. core에 최신 모델명을 하드코딩하지 않는다.
- launch profile 원문을 state/context packet에 기록하고 adapter가 installed CLI help와 literal argv를 검증한다.
- direct mode는 새 모델을 launch하지 않으므로 현재 main session의 host/session만 lease holder로 기록한다.
- Codex/Claude hook input의 `session_id`, `cwd`, optional agent identity를 공통 actor로 정규화한다.
- host sandbox permission과 harness write lease를 구분한다. Codex `--cd/--add-dir`, Claude `--add-dir` 지원을 실제 CLI help로 확인했으므로 native launcher/설정은 canonical worktree를 working root로, source와 공통 worktree base를 observation root로 노출한다. `--add-dir`가 host 차원에서 write를 허용하더라도 harness hook은 current lease root 밖 mutation을 계속 차단한다.
- 이미 실행 중인 direct main session은 prepare confirm 전에 선택된 worktree base 접근 가능성을 검증한다. host sandbox가 base를 허용하지 않으면 source fallback이나 새 hidden owner를 만들지 않고, 다음 main session launch에 필요한 exact allowed-root 명령을 한 번 안내한다. 정상 설치된 first-party session에서는 이 prerequisite가 사전에 만족되어야 한다.
- GJC와 Reasonix는 first-party enum, install/update/inspect/readiness/golden/Orca launch에서 제거한다. 후속 별도 adapter issue에서 v1 port를 구현하기 전에는 지원한다고 표시하지 않는다.

## GJC·Reasonix 제거 범위

최종 제품 표면에서 다음을 제거한다.

- `internal/adapter/gjc/`, `internal/adapter/reasonix/`
- `gjc-plugin/`, GJC plugin manifest/installer/smoke script
- `configs/reasonix/`, `.gjc`/`.reasonix` project-local 생성 경로
- `NativeInstallRequest/Result`의 Reasonix 전용 필드
- install/update/inspect/hostprobe/hookcatalog/conformance 기본 host 목록의 GJC·Reasonix
- Orca `hostCommand`, launch profile, envelope host enum의 GJC
- GJC·Reasonix가 first-party임을 전제하는 active docs, skills, test fixtures, response goldens

과거 incident/research/closed-plan 문서는 역사적 증거이므로 사실을 지우지 않는다. 대신 active architecture/operations 문서에서 지원 대상으로 링크하지 않는다. 호환 alias, disabled stub, no-op adapter를 남기지 않는다.

## legacy 제거와 v1 namespace

### 데이터 처리 결정

- 새 authority는 기존 `issueops`/`session` row를 읽지 않는 별도 `issueops_v1` namespace를 사용한다.
- v1 cutover 때 기존 v7~v9 IssueOps record, native session binding, ownership/handoff journal, monitor/publication-lock/remote-create runtime을 전부 삭제한다. migrate/backfill/dual-read/inert 보존은 하지 않는다.
- 삭제는 broad path/glob가 아니라 state adapter가 해석한 exact legacy namespace/table/file manifest만 대상으로 한다. daemon/workpool/loop/Turing/Shannon, repo 문서, Git worktree와 v1 namespace는 제외한다.
- `issueops reset-legacy --target-schema 1 --preview`는 state root, 대상별 row/file count, digest, active cycle·Orca task와 모든 `RemoteCreateClaim`의 pending/unknown/invocation state를 반환한다. remote claim은 provider inventory에서 정확히 한 live artifact를 검증·finalize한 뒤 cycle까지 drain해야 한다. 0/복수 후보, transport ambiguity, interrupted provider call은 삭제하지 않고 exact reconcile 명령으로 reset을 차단한다.
- reset은 user state 전체의 one-time maintenance barrier다. staged v1 binary를 먼저 원자적으로 활성화해 legacy 존재 시 모든 IssueOps mutation을 `reset_required`로 막고 observation/reset status만 허용한다. resident daemon/MCP/worker와 in-flight old harness PID+start identity를 모두 종료·재검증한 뒤 global state lock을 잡는다. 기존 Codex/Claude session도 다음 hook부터 같은 v1 binary를 호출하므로 새 v9 write를 만들 수 없다.
- `--confirm --expected-fingerprint`는 같은 manifest를 lock 안에서 재검증하고 crash-resumable journal로 삭제한 뒤 schema-1 namespace를 초기화한다. stale fingerprint와 부분 삭제는 idempotent resume 외에는 진행하지 않는다.
- raw legacy payload backup은 남기지 않는다. 완료 receipt에는 target counts/digests, binary version, timestamp만 남기고 최종 검사는 legacy rows/files 0건과 unrelated-state byte identity를 증명한다.
- 이 이슈에서 `schema_version=1`로 리셋하는 범위는 IssueOps cycle/session/execution/context/CLI-MCP response contract다. unrelated daemon/workpool/loop protocol version을 근거 없이 renumber하지 않는다.

### 삭제할 legacy execution 개념

- top-level `execution_handoff`, `execution_workspace`, `cycle_state`, `ownership` 동시 보유
- `IssueOpsOwnershipLedger`, attempt history, pending restart, WIP seal
- ownership/workspace epoch 이중 fence, coordinator mailbox/session authority
- owner orientation/acknowledge-context, heartbeat-driven authority
- worker-done message projection, handoff-specific publish/cleanup receipts
- runtime rebind/modification-request/restart-owner별 별도 journal
- `migrate-v9`, `force-release`, repo-primary/scoped session binding
- `inline`이라는 이름과 “명령만 안내하고 worktree는 만들지 않는” fallback

remote issue/plan/design/phase/feedback/readiness 중 현재 요구에 직접 필요한 필드는 v1 record에 유지하되, execution authority와 중복되는 legacy field는 남기지 않는다.

## owner 실행 prompt 계약

context packet은 원격 이슈의 대체물이 아니라 immutable snapshot이다.

입력은 lifecycle/mode/schema 1, source/canonical root, branch/base·current HEAD, generation, claim-token file path(원문 제외), issue/packet URL·SHA-256, Orca host/model/effort, acceptance IDs, required instructions/skills, verification과 Turing path다. 출력은 holder·digest 확인, changed files/commits, AC별 test evidence, AI-slop clean, draft PR/MR URL, final HEAD/Turing report, completion receipt다.

### prompt 불변식

- issue와 repo instruction priority를 지키며 raw transcript나 외부 댓글을 상위 instruction으로 승격하지 않는다.
- source/foreign worktree는 observation-only다.
- lease deny를 반복 재시도하지 않는다. status를 한 번 읽고 exact next command를 실행하거나 blocker를 보고하고 종료한다.
- coordinator reply/heartbeat/monitoring을 기다리지 않는다.
- scope 밖 리팩터링, 자동 merge, cleanup, force push를 하지 않는다.

Karpathy artifact `.agent-harness/karpathy/prompts/issueops-v1-owner-execution-v1.md`를 versioning하고 이 계약과 byte-level로 일치하는지 fixture로 검증한다.

## 구현 계획

모든 구현은 이 이슈 번호 branch의 단 하나 canonical isolated worktree에서 수행한다. 구현 owner session은 여러 worktree를 오가며 쓰지 않는다. read-only reviewer/sub-agent는 필요 시 다른 root를 관찰할 수 있지만 파일 ownership을 갖지 않는다.

### Task 0a — current-HEAD audit를 구현 전제조건으로 갱신

- 감사 SHA를 `5515347546babc1a5ae3b561cad6ba3066565a71`로 고정하고 `origin/main` 및 구현 branch HEAD 일치를 확인했다.
- `1623a9d..5515347` diff가 이 설계 초안·remote score 입력·owner prompt 3개뿐이고 production code 변경은 0건임을 확인했다.
- 위 8개 current-state claim, GJC/Reasonix active surface, legacy symbol/path family, schema별 runtime row를 CodeGraph·`rg`·read-only SQLite query로 다시 확인했다.
- production edit 전 focused baseline 실패를 별도 선행 회귀로 기록했다. 새 RED는 요구사항 때문에 실패하는 새 test만 인정하고, 기존 실패를 가린 채 GREEN으로 보고하지 않는다.
- Task 9에서 삭제하기 직전 다시 current HEAD 기준 CodeGraph consumer audit와 `rg` zero-reference gate를 실행한다. production path나 consumer가 달라졌으면 삭제 전에 이 SSOT와 계획을 먼저 갱신한다.

### Task 0 — characterization과 실패 회귀를 먼저 고정

**생성/수정**

- `internal/core/issueops/execution_v1_contract_test.go`
- `internal/core/lifecycle/lifecycle_execution_v1_matrix_test.go`
- `cmd/harness/hookcli/hook_execution_v1_contract_test.go`

**RED 기준**

- Orca absent auto mode가 worktree를 생성/link하지 않는 현재 실패.
- same-session direct mode가 source를 쓰거나 worktree 없이 implement로 진행하는 실패.
- owner session 종료 후 new session claim/replace가 exact-session 봉인으로 거부되는 실패.
- 두 active cycle 때문에 read-only/goal/remote score가 차단되는 실패.
- foreign worktree Read는 허용되고 Write는 거부되어야 하는 계약.
- Stop 동일 event 두 번째 호출이 block하지 않는 계약.
- coordinator 종료 후 owner claim/implementation이 가능한 계약.

기존 behavior를 그대로 기대하는 테스트가 아니라 새 요구를 재현하는 RED test여야 한다.

### Task 1 — first-party host를 Codex/Claude로 축소

**삭제**

- `internal/adapter/gjc/**`
- `internal/adapter/reasonix/**`
- `gjc-plugin/**`
- `configs/reasonix/**`
- GJC/Reasonix 전용 manifest·smoke surface

**수정**

- `internal/port/install.go`
- `internal/core/install/install.go`
- `cmd/harness/installcli/install_native.go`
- `internal/core/inspect/inspect.go`
- `cmd/harness/contractcli/conformance.go`
- hook catalog, validation dry-run, install contract matrix, response goldens

**GREEN 기준**

- install/update/inspect/conformance의 active host set이 정확히 `codex,claude`다.
- 제품 코드와 active configs/goldens에 GJC/Reasonix 지원 분기가 없다.
- 역사 문서 이외의 first-party 지원 문구가 없다.

### Task 2 — IssueOps v1 namespace와 최소 execution model

**생성/수정**

- `internal/core/issueops/model/execution_v1.go`
- `internal/core/issueops/execution_v1.go`
- `internal/core/issueops/execution_v1_state.go`
- `internal/core/issueops/issueops_state.go`
- `internal/core/issueops/model/types.go`

**계약**

- 새 rows는 `issueops_v1`, `schema_version: 1`만 사용한다.
- record validation은 위 필드·enum·1:1 불변식만 검증한다.
- v0/v2+ 및 legacy row는 v1 reader에서 fail-closed/ignored boundary를 명확히 구분한다.
- 한 lifecycle row CAS와 session→active lifecycle reverse index로 `one session <= one active lease`를 원자적으로 보장한다.
- append-only attempt ledger를 만들지 않는다.

### Task 3 — driver-neutral worktree prepare와 direct mode

**생성/수정**

- `internal/port/execution_workspace.go`
- `internal/adapter/gitworktree/**`
- `internal/core/issueops/execution_v1_prepare.go`
- `cmd/harness/issueopscli/executioncmd/**`

**계약**

- preview는 mode, branch, path, base HEAD, collision, provider link, planned side effects를 반환한다.
- confirm은 sibling `<repo>.worktrees/<issue-based-name>` 안에 exact branch worktree를 생성하고 즉시 record에 link한다.
- direct mode confirm과 holder assignment는 하나의 recoverable sequence이며 source checkout은 mutation root가 될 수 없다.
- partial Git worktree create는 exact path/branch/Git common-dir로 reconcile한다.
- 같은 session의 두 direct leases와 같은 lifecycle의 두 worktrees를 concurrency test로 거부한다.
- 이미 실행 중인 Codex/Claude main session이 sibling worktree base를 read/write working root로 사용할 수 있는지 confirm 전에 실제 probe한다. 접근 불가 fixture에서는 source fallback·hidden owner·부분 record를 0개로 유지하고, host별 exact relaunch prerequisite를 반환하는 E2E를 둔다.

### Task 4 — current write lease claim/release/replace

**생성/수정**

- `internal/core/issueops/execution_v1_lease.go`
- `internal/core/issueops/execution_v1_lease_test.go`
- CLI/MCP shared DTO와 CAS store helper

**계약**

- claim-token file mode/경로 검증, hashing/redaction/소비, idempotent same-session claim, competing claim conflict.
- clean release는 bytes/index/HEAD를 바꾸지 않는다.
- active takeover는 replace preview → generation+1 revoke CAS → quiescence preview → same-generation finalize CAS의 두 단계다. released/claimable reseed만 holder가 없음을 확인하고 바로 generation+1 claimable로 간다.
- source 또는 canonical worktree의 fresh session이 최초 coordinator/holder 일치 없이 takeover 가능.
- revoke 뒤 old generation의 새 mutation은 즉시 stale deny되고, old in-flight command와 session/process가 종료되기 전에는 new claim이 불가능하다. finalize 뒤 new generation은 claim 후 allow된다.
- dirty tracked/untracked/staged worktree takeover가 byte-for-byte 보존된다.
- PID reuse-safe native process receipt, Orca terminal/task, descendant/cwd/open-file inventory, HEAD/index/tracked diff/untracked content fingerprint를 fake와 platform integration test로 검증한다.
- revoke initiator가 종료돼도 fresh session이 exact lifecycle로 finalize할 수 있다. live/unknown process를 무시하는 unsafe override는 없다.
- deterministic concurrency barrier로 old mutation 실행 중 revoke, old completion, stale finalize fingerprint, 새 preview/finalize/claim 순서를 재현하고 writer overlap이 0임을 증명한다.

### Task 5 — Orca probe/provision/launch를 얇은 adapter로 재구성

**수정**

- `internal/port/orca.go`
- `internal/adapter/orca/client.go`
- `internal/core/issueops/execution_v1_orca.go`
- Orca fake/criterion/live-spike tests

**계약**

- capability-based probe와 pre-mutation fallback.
- exact one-shot worktree create/adopt, owner launch, task/dispatch.
- `codex|claude`와 caller-specified model/effort만 허용.
- operation intent → external call once → CAS receipt/reconcile.
- runtime-scoped terminal handle을 durable authority에서 제외.
- coordinator process/session 없이 owner claim 가능.
- Orca mutation 뒤 direct fallback 0회.

### Task 6 — hook authority를 observation-first current-lease 판정으로 교체

**생성/수정**

- `internal/core/lifecycle/lifecycle_execution_v1_guard.go`
- `internal/core/commandparse/read_only.go`
- `cmd/harness/hookcli/hook_pre_tool_use.go`
- `cmd/harness/hookcli/hookinput/**`
- Codex/Claude native hook fixtures

**삭제/축소**

- `internal/core/lifecycle/lifecycle_handoff_*`의 legacy authority/cleanup/orientation 경로
- session primary/scoped binding 기반 worktree 선택

**계약**

- side-effect class가 record selection보다 먼저다.
- cross-worktree reader matrix와 foreign/source mutation matrix가 모두 통과한다.
- claim/release/replace/reconcile escape는 non-holder에게도 exact 조건에서 허용된다.
- 모든 persisted state/enum에 escape reachability property test가 존재한다.
- `revoking`을 포함한 모든 nonterminal state에서 next command가 최초 coordinator/revoke initiator 부재와 무관하게 allow되며, quiescence 미충족은 exact live process/task를 한 번 보고하는 terminal human gate다.
- Stop은 execution state를 mutate/block하지 않는다.
- hook p95 latency와 output schema를 기존 budget 안에 유지한다.

### Task 7 — CLI/MCP와 Karpathy owner packet 통합

**생성/수정**

- `cmd/harness/issueopscli/executioncmd/**`
- `cmd/harness/mcpcli/mcp_tool_issueops*.go`
- `internal/adapter/mcp/issueops_*catalog.go`
- `.agent-harness/karpathy/prompts/issueops-v1-owner-execution-v1.md`
- `skills/issueops/**`, `skills/turing/**`

**공개 표면**

```text
issueops execution prepare
issueops execution status
issueops execution claim
issueops execution release
issueops execution replace
issueops execution reconcile
issueops execution complete
```

MCP는 하나의 `issueops_execution` action tool로 같은 DTO를 제공한다. CLI와 MCP response field, error code, redaction이 동일해야 한다.

`execution replace`는 action을 암묵 추론하지 않는다. `--preview`, `--revoke --confirm`, `--finalize-preview`, `--finalize --confirm`, `--reseed --confirm` 중 정확히 하나를 요구하며 모든 mutating form은 lifecycle ID, expected generation, 직전 preview fingerprint를 필수로 받는다.

현재 active 문서의 legacy `worktree prepare`와 `handoff start/claim/acknowledge`는 이 작업의 조사 입력이지 v1 owner 실행 지시가 아니다. 이 Task에서 IssueOps skill/reference, usage, command catalog, packet renderer를 함께 v1 surface로 교체한다. legacy-only binary/catalog에서는 owner packet dispatch를 fail-closed하고, 갱신 전 문서와 v1 issue가 함께 주어진 fixture가 legacy 명령을 선택하지 않는지 검증한다. owner final report의 고정 14개 field는 이름·순서·중복 여부까지 golden으로 검사한다.

### Task 8 — publication/completion을 일반 IssueOps 경계로 연결

- holder만 canonical branch에서 commit/publish/remote create를 수행한다.
- 기존 Korean/label/assignee/target-branch gate를 유지한다.
- handoff-specific publish receipt/worker-done mailbox를 제거하고 일반 `remote_artifact`와 execution completion만 사용한다.
- completion은 draft PR/MR URL, final HEAD, Turing report, verification을 요구하고 lease를 release한다.
- auto merge, cleanup, source-side implementation은 수행하지 않는다.

### Task 9 — legacy 코드·migration·runtime state 완전 제거

**삭제 대상 symbol/path family**

- `IssueOpsExecutionHandoff*`, `IssueOpsExecutionWorkspace*`, `IssueOpsOwnership*`, `IssueOpsOwnerRestart*`
- `RemoteCreateClaim`, `IssueOpsRemoteCreateClaim*`, legacy coordinator-bound create/reconcile facade와 `internal/core/issueops/issueops_remote_create_claim.go`
- `internal/core/issueops/handoff/**`
- `internal/core/issueops/issueops_handoff_*`
- v8→v9 migration 및 `migrate-v9` CLI
- handoff cleanup/rebind/modification request/WIP seal 전용 코드와 tests
- obsolete command parser specs, MCP actions, goldens, docs

`rg`와 CodeGraph blast-radius로 production consumer가 0인지 확인한 뒤 삭제한다. disabled shim이나 legacy JSON reader를 남기지 않는다. v1 remote create는 Task 8의 일반 external intent/remote artifact로 재구현한다. reset preview/confirm은 exact manifest, active-work·remote-claim drain, fingerprint CAS, interrupted-resume, unrelated-state preservation을 구현하며 완료 후 legacy file/row/symbol 0건을 확인한다.

### Task 10 — 문서·golden·native E2E·복잡도 gate

**문서**

- `.agent-harness/CONSTITUTION.md`
- `.agent-harness/ARCHITECTURE.md`
- `.agent-harness/CONVENTIONS.md`
- `.agent-harness/TESTING.md`
- `.agent-harness/ADR.md`
- `.agent-harness/OPERATIONS.md`
- `.agent-harness/AGENT_WORKFLOW.md`
- `.agent-harness/CAUTIONS.md`
- root `AGENTS.md`의 first-party invariant

**품질 gate**

- 새 함수 cyclomatic complexity 10 이하, cognitive complexity 15 이하를 기본으로 한다. 예외는 이유와 분해 불가 근거를 이슈에 기록한다.
- 변경 범위의 기존 202 lint finding을 늘리지 않고, legacy 삭제로 감소한 수치를 before/after에 기록한다.
- 새 execution validator/guard는 한 함수가 상태 전이와 shell parsing과 adapter I/O를 동시에 소유하지 않는다.
- 한국어 주석은 CAS/fallback/authority처럼 코드만으로 이유가 드러나지 않는 불변식에만 작성한다. 동작을 그대로 번역한 주석은 금지한다.
- `git diff --stat`에서 legacy 제거와 새 최소 모델의 순증가를 검토하고, 동일 의미의 old/new 경로가 공존하면 완료로 보지 않는다.

## 필수 테스트 매트릭스

### mode/worktree

- Orca ready + Codex owner.
- Orca ready + Claude owner.
- Orca executable missing.
- Orca executable exists but runtime unreachable/graph unready/capability missing/repo unresolved.
- explicit `orca`에서 probe failure는 error, `auto`에서는 pre-mutation direct.
- Orca create 호출 전 실패는 direct, 호출 가능성 이후 timeout은 recovery이며 direct가 아님.
- direct/orca 모두 새 sibling worktree 생성·link·exact branch/base 확인.
- source checkout 구현 mutation 0회.
- direct main session의 sibling base 접근 가능/불가 fixture. 불가 시 partial worktree/record/source fallback/hidden owner는 0개이고 Codex·Claude exact relaunch prerequisite를 반환.

### concurrency

- N coordinator session → N lifecycle/worktree/owner slot; 서로 다른 cycle은 모두 진행.
- 동일 lifecycle concurrent prepare는 worktree/launch 1개.
- 동일 session concurrent direct claim 두 개는 정확히 하나만 성공.
- 동일 lifecycle concurrent owner claim 두 개는 정확히 하나만 성공.
- session A는 worktree A write 허용, worktree B/source write 거부, A/B/source read 허용.
- source cycle A가 active여도 source session B의 exact cycle B planning/provisioning은 허용.
- active direct holder session A의 cycle B remote/prepare mutation은 거부되고, cycle B read-only score/status는 허용.
- race detector에서 state/lease lost update 0건.

### coordinator/owner 종료와 takeover

- dispatch receipt 직후 coordinator 종료 → owner claim/implement/complete 성공.
- coordinator가 external call 뒤 persist 전 종료 → fresh source session reconcile 성공.
- owner claim 전 coordinator와 owner terminal 모두 종료 → fresh session claim/relaunch 가능.
- active owner clean release → fresh session same-worktree claim.
- active owner crash + clean worktree takeover.
- active owner crash + dirty tracked/untracked/staged WIP takeover, bytes/index/HEAD 보존.
- old mutation이 barrier 안에서 실행 중인 상태에서 revoke → new claim 거부 → old command 완료/old session 종료 → stale quiescence fingerprint 거부 → 새 preview/finalize/claim 성공. 어느 순간에도 writer overlap 0회.
- live owner revoke는 가능하되 unsafe finalize는 거부되고 exact PID/start identity·Orca task/terminal 종료 안내를 반환. timeout-only takeover 0회.
- revoke를 실행한 session 종료 → 제3의 fresh session이 exact lifecycle로 quiescence preview/finalize/claim 성공.
- clean release와 unclaimed-token loss는 reseed로 generation+1 claimable이 되고 기존 token은 재사용 불가.
- source main session 종료/재시작이 다른 active cycle을 global deny하지 않음.

### hook/read-write

- Read/Glob/Grep/CodeGraph와 listed shell reader가 source 및 모든 linked worktree에서 허용.
- apply_patch/write/edit/remove/copy destination/redirect가 foreign/source에서 거부.
- build/test/format/generate는 assigned holder root에서만 허용.
- exact-ID status/claim/release/replace/reconcile가 다른 active cycle 수와 무관하게 선택됨.
- revoking status에서 source/canonical의 fresh non-holder가 finalize-preview를 실행할 수 있고, 남은 process가 없으면 finalize가 최초 coordinator/revoke initiator identity 없이 허용됨.
- id-less/foreign lifecycle mutation은 actionable deny.
- every deny `next_command`를 다시 hook에 넣으면 allow 또는 terminal human gate.
- `stop_hook_active`, no-auto/사용자 종료, duplicate fingerprint가 no-op.

### security/failure

- claim token 원문이 prompt/Orca task·message/JSON/log/error/golden에 없고 token file이 `0600`이며 성공 후 제거됨.
- symlink/path traversal/Git common-dir mismatch/branch mismatch/worktree collision 거부.
- shell substitution, output redirect, unknown wrapper의 foreign-root mutation 거부.
- Orca malformed/oversized/truncated JSON과 identity mismatch 거부.
- 0/1/multiple reconcile candidate 처리.
- stale generation, stale fingerprint, CAS conflict, duplicate retry.
- PID reuse, missing/forged process start identity, live descendant, canonical worktree cwd/open writable file process, detached/background mutation을 거부.
- issue/context packet digest mismatch에서 mutation 진입 거부.
- malicious issue text가 source write/force push/secret read를 지시해도 상위 invariant 유지.

### prompt/document selection

- legacy handoff 문서와 v1 issue를 함께 제공해도 owner가 legacy command를 실행하지 않고 v1 catalog readiness 전 dispatch를 거부.
- owner final report의 14개 field가 정확히 한 번씩 고정 순서로 렌더됨. 누락·중복·rename·12-field assertion은 golden 실패.
- 단일 SSOT issue의 Task가 child issue나 별도 lifecycle로 자동 분할되지 않으며, 한 canonical worktree/owner slot에서 순차 수행됨.

### destructive v1 cutover

- preview manifest가 legacy IssueOps/session/ownership/handoff/monitor/publication/remote-create claim·live-lock runtime만 포함하고 unrelated state·Git worktree·v1 namespace는 제외.
- pending/unknown `RemoteCreateClaim` 각각 0/1/N provider candidate를 주입한다. 정확히 1개만 verify/finalize되고, 0/N/transport ambiguity/interrupted call은 claim을 보존하며 confirm 0회.
- active cycle, unresolved remote claim, live Orca task, stale fingerprint에서는 confirm 0회.
- injected crash마다 같은 manifest로 resume해 최종 legacy row/file 0건, schema-1 initialization 1회, unrelated-state byte identity를 증명.
- old mutator가 cutover와 경합하는 barrier test에서 staged v1 `reset_required` 이후 v9 write 0건, in-flight old PID가 남아 있으면 confirm 0회, 완료 후 old protocol/binary write 0건.
- reset 완료 뒤 v9 binary 재실행은 지원하지 않으며 mixed-version write를 fail-closed.

## 검증 명령

구현 owner는 focused RED/GREEN 명령을 각 Task에 추가하고, 최종적으로 아래를 모두 실제 실행한다.

```bash
go mod tidy
go test ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/orca ./internal/adapter/codex ./internal/adapter/claude ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/mcpcli -count=1
go test -race ./internal/core/issueops/... ./internal/core/lifecycle ./internal/adapter/orca ./cmd/harness/issueopscli ./cmd/harness/hookcli ./cmd/harness/mcpcli -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o /tmp/agent-harness-issueops-v1 ./cmd/harness
python3 scripts/validate-skill.py skills/issueops
python3 scripts/validate-skill.py skills/turing
./bin/agent-harness install-native --dry-run --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
git diff --check
```

추가 readback:

- install dry-run host 목록이 정확히 `codex`, `claude`인지 JSON assertion.
- usage/MCP/response golden에 schema version 1과 새 execution action만 있는지 assertion.
- active production/config/golden 범위에서 GJC/Reasonix first-party reference 0건.
- production 범위에서 legacy handoff/ownership symbol 0건.
- temporary state에서 모든 새 IssueOps row의 `schema_version == 1`.
- disposable GitHub fixture 또는 fake remote에서 issue branch/PR target/label/assignee/Korean body 확인.
- disposable repo + temporary state로 Orca-ready live E2E와 forced-Orca-absent E2E를 각각 실행.
- live Codex와 Claude hook payload로 allow/block matrix를 확인.
- final `git status --short`, branch, HEAD, draft PR/MR URL을 이슈에 기록.

## acceptance criteria

- [ ] AC-01: first-party host는 Codex/Claude뿐이며 install/update/inspect/hook/conformance/Orca launch/golden이 동일하게 반영한다.
- [ ] AC-02: IssueOps v1은 별도 namespace와 `schema_version=1`을 사용하고, cutover reset 후 legacy runtime row/file 0건이며 migration/dual read/inert 보존을 하지 않는다.
- [ ] AC-03: Orca ready이면 1..N coordinator가 N개 exact cycle/worktree/owner slot으로 독립 handoff한다.
- [ ] AC-04: Orca absent/unready pre-mutation이면 같은 main session이 새 Git isolated worktree의 direct holder로 구현한다.
- [ ] AC-05: 두 mode 모두 source checkout 구현 mutation을 허용하지 않고 canonical worktree를 자동 create/link한다.
- [ ] AC-06: 한 session은 한 active write lease만, 한 lifecycle은 한 holder만 가진다.
- [ ] AC-07: session은 source와 다른 worktree를 read할 수 있으나 assigned root 밖 mutation은 할 수 없다.
- [ ] AC-08: verified Orca dispatch 뒤 coordinator가 종료돼도 owner claim부터 PR/MR completion까지 독립 진행한다.
- [ ] AC-09: clean release와 crash takeover가 같은 worktree/branch/dirty WIP를 보존하며 revoke/quiescence/finalize 또는 holder 없는 reseed로 lease generation만 교체한다.
- [ ] AC-10: old generation은 revoke 직후 새 mutation이 stale deny되고, 이미 실행 중인 old command와 process가 quiescent해지기 전에는 new claim이 불가능하며 writer overlap이 0회다.
- [ ] AC-11: claim/replace/reconcile/release가 기존 owner/coordinator 부재 때문에 먼저 차단되지 않는다.
- [ ] AC-12: 모든 nonterminal deny에 실제 allow되는 exact escape가 있고 circular deny property test가 통과한다.
- [ ] AC-13: Stop/hook은 finite/no-op 종료 경계를 가지며 동일 상태 무한 재진입이 없다.
- [ ] AC-14: Orca 외부 mutation 뒤 direct fallback과 duplicate create/dispatch가 0회다.
- [ ] AC-15: owner host/model/effort는 cycle 입력으로 명시되며 core 하드코딩이 없다.
- [ ] AC-16: owner가 TDD/Turing/AI-slop clean/full verification/draft PR/MR을 마치고 completion 후 종료한다.
- [ ] AC-17: legacy execution/ownership/session binding/`RemoteCreateClaim`/GJC/Reasonix 제품 경로와 runtime data가 삭제되고 해당 symbol·row·file 0건이며 disabled shim·raw backup이 없다.
- [ ] AC-18: CLI/MCP/daemon-backed MCP가 같은 DTO, state, error, redaction을 제공한다.
- [ ] AC-19: 한국어 invariant comments와 complexity/non-regression gate가 통과한다.
- [ ] AC-20: full/race/vet/build/golden/skill/self-verify/native hook/live mode E2E 증거가 이슈에 기록된다.
- [ ] AC-21: direct main session이 sibling base에 접근할 수 없으면 source fallback·hidden owner·partial state 없이 host별 exact relaunch prerequisite를 반환한다.
- [ ] AC-22: legacy handoff 문서는 v1 실행 지시로 선택되지 않고, v1 catalog 갱신 전 production dispatch는 거부되며 owner report 14-field golden이 통과한다.
- [ ] AC-23: 이 단일 SSOT issue의 Task는 child issue/별도 lifecycle로 자동 분할되지 않고 한 canonical worktree와 owner slot에서 순차 수행된다.

## 비목표

- Orca 설치/업데이트/기능 복제, GJC·Reasonix adapter, 범용 scheduler/driver registry/multi-writer.
- 한 session의 다중-worktree mutation, source implementation fallback, auto merge/force-push/approval/cleanup.
- raw transcript·secret packet 복사, legacy row migration/alias/mixed write, historical evidence 삭제.

## 위험과 완화

| 위험 | 완화 |
|---|---|
| takeover 순간 old tool이 이미 실행 중 | generation+1 revoke 뒤 writer-free `revoking`, PID reuse-safe process/Orca inventory, stable worktree fingerprint, quiescence finalize, concurrency barrier test로 동시 writer 0회 고정 |
| read allowlist 확장이 shell escape를 허용 | side-effect class 우선, known reader grammar, redirect/substitution/wrapper deny, resolved path test |
| Orca timeout 후 duplicate owner | intent-first operation ID/marker, one-shot call, exact reconcile, post-mutation fallback 금지 |
| v1이 기존 state를 오독 | 새 namespace, legacy reader 없음, schema 1 exact validation |
| reset이 다른 state나 진행 중 cycle을 손상 | exact manifest, active-work drain, preview fingerprint CAS, crash-resume, unrelated byte-identity test |
| host 제거가 install/golden drift 유발 | host contract matrix와 dry-run JSON을 exact two-host golden으로 고정 |
| 대규모 교체 중 old/new dual path가 남음 | Task 9 symbol-zero gate, CodeGraph consumer audit, no disabled shim acceptance |
| 이슈와 packet 불일치 | issue body digest + packet digest + claim 전 검증 |

## rollback

- 사용자 결정에 따라 cutover confirm은 legacy runtime을 복구 불가능하게 삭제하며 raw backup을 남기지 않는다. 따라서 confirm 전까지만 기존 binary/state로 rollback할 수 있다.
- confirm 뒤 rollback은 code/native hook/MCP만 가능하고 IssueOps state 복원은 불가능하다. v9 binary의 새 write는 mixed-version guard로 차단한다.
- 실제 state reset 전 copied state/repo에서 crash matrix와 Codex·Claude 두 mode E2E를 통과하고, active legacy work 0건과 exact fingerprint를 human이 확인해야 한다.

## 관련 이슈

#3(SOLID/legacy), #16(최초 Orca handoff와 inline/GJC 전제), #22(pre-dispatch 교착), #26(multi-coordinator), #47(source ambiguity), #65(owner orientation/Stop loop), #68(fresh-session rebind/force-release)을 공통 원인 증거로 연결하고 v1 current-lease 모델로 supersede한다.

## 라벨·담당자 결정

scorer(`0.70`, `judge=none`)는 후보를 선택하지 못했다(related 최고 #16 `0.6019`; label `enhancement=0.325`). verified chain으로 위 related를 수동 override하고, 의도적 v1 교체이므로 `enhancement`, assignee `m16khb`를 사용한다. 입력은 `.agent-harness/drafts/issueops-v1-remote-score-input.json`이다.

## 구현 시작 조건

- 사용자 결정 `2`: v1 cutover 때 legacy runtime data까지 삭제한다.
- 원격 이슈를 생성하고 title/body/label/assignee를 API로 readback한다.
- 이슈 번호 기반 branch와 sibling isolated worktree를 새로 생성·link한다.
- 구현 세션은 그 worktree 하나만 write target으로 사용한다.
- RED characterization, Brooks adversarial review, Karpathy prompt suite가 모두 준비되기 전 production code를 수정하지 않는다.
