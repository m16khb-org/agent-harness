# IssueOps Handoff Modification Request Design

## 문제

IssueOps ownership handoff가 PR/MR을 만든 뒤에도 수정 요청이 들어올 수 있다.
현재 owner는 worker root에서 수정, 커밋, publish receipt 갱신을 수행할 수
있지만 source/main 세션이 그 요청을 안전하게 전달하는 정식 경로는 없다.

직접 `orca terminal send`를 사용하면 PTY 입력을 주입하게 되고, 직접
`orca orchestration send`를 사용하면 lifecycle ID와 봉인된 task/dispatch
identity를 우회할 수 있다. 반대로 source를 완전히 observation-only로 두면
원 coordinator 세션이 종료된 뒤 새 main 세션이 active owner에게 리뷰 수정
요청을 전달할 수 없다.

## 목표

- PR/MR이 검증된 active handoff에 새 exact-source native 세션이 수정 요청을
  보낼 수 있다.
- 요청은 봉인된 Orca mailbox, task, dispatch identity로만 전달한다.
- 외부 메시지 mutation 전에 durable intent를 기록하고 결과를 CAS로 남긴다.
- owner만 feedback, 코드, 커밋, phase, publish receipt를 변경한다.
- owner의 후속 publish는 기존 receipt HEAD의 fast-forward descendant만 허용한다.
- CLI와 MCP가 같은 core request/result DTO를 사용한다.
- raw terminal steering과 unfenced orchestration send는 계속 차단한다.

## 비목표

- source가 worker 파일을 수정하거나 owner shell에 입력을 주입하는 기능
- source가 feedback item, phase, publish receipt, remote artifact를 직접 기록하는 기능
- completed, cleanup, closed, cancelled, recovery handoff를 다시 여는 기능
- non-fast-forward push, force-push, branch 교체, PR/MR 재생성
- Orca가 아닌 임의 메시징 backend 또는 일반-purpose send command
- 전송 실패를 자동 재시도하거나 전달 여부를 추정하는 기능

## 선택한 접근

CLI에 `issueops handoff request-modification`을 추가하고 MCP의 기존
action-discriminated `issueops_handoff` tool에 `request-modification` action을
추가한다. 별도 MCP tool은 만들지 않는다.

Core는 source actor와 handoff 상태를 검증한 뒤 bounded projection intent를
IssueOps record에 먼저 append한다. Lock을 해제한 뒤 Orca adapter가 정확히 한
번 `status` message를 전송하고, 다시 lock을 잡아 같은 request key의 intent만
`sent` 또는 `failed`로 갱신한다.

이 경로는 source disengagement의 일반 예외가 아니다. 수정 요청 message라는
한 종류의 typed control과 그 delivery projection write만 허용한다. Feedback,
phase, code/work artifact, publication 같은 durable workflow mutation은
owner-only로 유지한다.

## 권한 모델

요청자는 handoff를 시작한 원 coordinator 세션일 필요가 없다. 다음 조건을
모두 만족하는 fresh source session을 허용한다.

- `host`, `session_id`, optional `agent_id`가 현재 hook request의 native identity와
  정확히 일치한다.
- `source_cwd`와 tool request CWD가 모두 canonical `record.Repo`다.
- host와 session ID가 비어 있지 않고 host가 `codex|claude|gjc` 중 하나다.
- 요청자가 봉인된 `OwnerSession`과 동일하지 않다.
- worker root, sibling worktree, repo 밖 경로에서 실행하지 않는다.

Core는 hook을 우회한 직접 CLI/MCP 호출에도 host/session 존재, supported host,
exact source root, non-owner 조건을 다시 검증한다. Hook은 command flags가 실제
hook actor identity와 일치하는지 추가로 검증한다. Mailbox handle은 routing
identity이며 native session 권한을 대신하지 않는다.

## 입력 계약

CLI:

```text
agent-harness issueops handoff request-modification \
  --id ID \
  --host HOST \
  --session-id SESSION \
  [--agent-id ID] \
  --source-cwd PATH \
  --body TEXT \
  --confirm \
  [--json]
```

MCP `issueops_handoff`:

```json
{
  "action": "request-modification",
  "id": "io-...",
  "host": "codex",
  "session_id": "...",
  "agent_id": "...",
  "source_cwd": "/absolute/source/root",
  "body": "bounded correction request",
  "confirm": true
}
```

`body`는 trim 후 `policy.RedactFreeform`을 통과한 1~4096 bytes의 valid UTF-8
text다. LF(U+000A)와 tab(U+0009)을 제외한 ASCII C0 control, CR(U+000D),
DEL(U+007F)을 거부한다. Redaction 결과가 비거나 한도를 넘으면 외부 호출 전에
실패한다. 전송과 durable projection에는 항상 같은 redacted body만 사용한다.

`--confirm`/`confirm=true`가 없으면 외부 호출과 state write를 모두 수행하지
않고 오류를 반환한다. 이 명령은 preview mode를 제공하지 않는다.

## 상태 전제조건

Intent를 기록하기 직전 cycle lock 안에서 다음 조건을 다시 검증한다.

- `ExecutionHandoff.State == owner_active`
- `record.Phase == pr`
- `ExecutionHandoff.Completion == nil`
- `ExecutionHandoff.PublishReceipt != nil`
- `record.RemoteArtifact != nil`
- publish receipt의 provider/base가 remote artifact와 일관됨
- `CoordinatorMailboxHandle`, `WorkerMailboxHandle`, `TaskID`, `DispatchID`가
  canonical하고 비어 있지 않음
- 현재 request actor가 fresh authenticated exact-source session임

`cleanup_pending_human_decision`, `cleanup_executing`, `closed`,
`recovery_required`, cancellation state에서는 요청을 거부한다. 이 기능은
`handoff complete`를 되돌리지 않는다.

## Durable projection

`IssueOpsExecutionHandoff`에 다음 optional bounded history를 추가한다.

```go
ModificationRequests []IssueOpsExecutionHandoffModificationRequest `json:"modification_requests,omitempty"`
```

각 entry는 다음 정보를 가진다.

```text
request_key
attempt
ownership_epoch
context_sha256
state                 intent | sent | failed
invoked
diagnostic_code
payload_sha256
from_handle
to_handle
subject
body
task_id
dispatch_id
published_head
remote_artifact_url
requested_by_host
requested_by_session
requested_by_agent_id
message_id
message_sequence
started_at
completed_at
```

History는 최대 32개다. 33번째 요청은 오래된 기록을 지우지 않고 명확한 limit
오류로 거부한다. 모든 string과 timestamp는 handoff envelope에서 canonical,
redacted, length-bounded인지 검증한다.

현재 ownership contract인 IssueOps schema version 8을 유지한다. 이 필드는
기존 schema 8 record에서 absent인 optional additive field이며, 기존 active
handoff를 invalidating하는 schema bump나 migration을 만들지 않는다. Schema
round-trip test는 field가 존재할 때 손실 없이 보존되고 absent record가 기존과
같이 읽히는지 고정한다.

## Request identity와 idempotency

`request_key`는 length-delimited canonical encoding의 SHA-256이다.

```text
domain: issueops-handoff-modification-request:v1
record id
attempt
ownership epoch
context sha256
publish receipt final head
remote artifact URL
redacted body
```

Actor identity는 key에 포함하지 않는다. 동일 published HEAD와 동일 body를 새
source session이 다시 요청해도 같은 logical request로 처리하기 위해서다.

- 같은 `request_key`가 이미 있으면 기존 projection을 반환하고 Orca를 다시
  호출하지 않는다.
- owner가 수정 후 fast-forward 재게시하면 publish receipt HEAD가 바뀌므로 같은
  body도 새로운 request가 될 수 있다.
- `intent`는 no-retry tombstone이다. Process가 intent write 뒤 종료되더라도 같은
  key를 자동 재전송하지 않는다.
- 다른 body 또는 다른 published HEAD는 별도 key와 별도 intent를 만든다.
- 동시에 들어온 같은 key는 cycle lock 아래 하나만 append된다.

## 전송 흐름

1. Request를 normalize/redact하고 read-only outer validation을 수행한다.
2. Cycle lock 안에서 record와 actor/state를 재검증한다.
3. Existing key가 있으면 해당 projection을 반환한다.
4. 새 projection을 `intent`, `invoked=false`, `diagnostic_code=intent_persisted`로
   append하고 record를 write한다.
5. Lock을 해제한다.
6. Orca adapter를 정확히 한 번 호출한다.
7. Cycle lock 안에서 request key와 immutable projection fields를 비교한다.
8. 성공이면 `sent`, message ID/sequence, completion timestamp를 기록한다.
9. 실패면 `failed`, bounded diagnostic code, invoked flag, completion timestamp를
   기록한다.

Post-call write는 다른 정상 IssueOps 필드가 바뀌었다는 이유만으로 결과를
버리지 않는다. History에서 같은 key의 immutable intent를 찾아 exact compare한
뒤 그 entry만 갱신한다. Entry가 없거나 변조됐으면 stale 결과를 기록하지 않고
CAS 오류를 반환한다.

Timeout 또는 transport error에서 adapter가 process invocation 여부를 알 수
있으면 `invoked`에 반영한다. 어떤 실패도 자동 재시도하지 않으며 handoff를
inline mode로 전환하지 않는다.

## Orca adapter 계약

`internal/port`에 narrow request/result DTO와 client interface를 추가한다.

```text
OrcaModificationRequest
OrcaModificationResult
OrcaModificationClient.SendModificationRequest(ctx, request)
```

Adapter argv는 다음 고정 shape다.

```text
orca orchestration send
  --to <WorkerMailboxHandle>
  --from <CoordinatorMailboxHandle>
  --type status
  --subject "IssueOps <id> modification requested"
  --body <redacted-body>
  --task-id <TaskID>
  --dispatch-id <DispatchID>
  --json
```

Shell string을 만들지 않고 literal argv slice만 사용한다. Response는 message
ID, positive sequence, from/to/type/subject/body, payload의 taskId/dispatchId가
request와 정확히 일치해야 성공이다. Malformed, oversized, mismatched response는
fail-closed diagnostic으로 반환한다.

Fresh source session의 native identity가 권한이고, persisted
`CoordinatorMailboxHandle`은 source 역할의 안정된 orchestration routing
identity다. Live terminal handle을 mailbox identity로 승격하지 않는다.

## Owner 처리와 재게시

Owner는 status message를 받은 뒤 canonical worker root에서 다음 순서를 따른다.

1. `issueops feedback add`로 요청을 durable feedback으로 기록한다.
2. Contract change면 remote issue를 갱신하고 `feedback mark-issue-updated`를 기록한다.
3. `feedback -> implement`로 돌아가 TDD 수정과 검증을 수행한다.
4. AI slop cleanup evidence를 새 HEAD에 맞게 갱신한다.
5. 로컬 commit을 만든다. 직접 `git push`는 계속 금지한다.
6. `pr` phase로 돌아간다.
7. `issueops handoff publish --confirm`으로 새 HEAD를 재게시한다.

CLI `issueops feedback add`를 post-transfer exact owner recorder allowlist에 추가한다.
허용 조건은 existing `validatePostTransferMutation`: exact owner native session,
canonical worker root, `owner_active`다. Source 또는 다른 session의 feedback add는
CLI와 MCP 모두 거부한다.

기존 publish receipt 교체는 provider, project, remote, push target, branch, base,
remote ref가 모두 같고 previous `FinalHead`가 new `FinalHead`의 ancestor일 때만
허용한다. 이 규칙을 focused regression test로 고정한다. Non-fast-forward,
다른 branch/base/remote, ancestry lookup error는 push 전에 거부한다.

## Lifecycle guard

Exact command parser와 lifecycle authority에
`handoff request-modification`을 추가한다.

- ID, actor flags, exact source CWD, body, confirm을 요구한다.
- Hook request identity와 CLI actor flags가 일치해야 한다.
- active PR handoff와 verified artifact를 요구한다.
- owner session, worker CWD, sibling worktree, dynamic path는 거부한다.

Protected Orca resource targeting은 direct `orca orchestration send`의
`--to`와 `--from`도 terminal identity로 수집한다. `--task-id`와
`--dispatch-id`는 기존처럼 함께 수집한다. MCP orchestration send도
`to/from/to_handle/from_handle`을 동일하게 분류한다.

따라서 persisted owner mailbox, task, dispatch 중 하나를 직접 겨냥한 raw send는
해당 handoff record를 선택하고 일반 source control로 빠져나가지 않는다. Typed
IssueOps command만 source 예외로 허용한다. Raw `orca terminal send` guidance,
arbitrary orchestration type/payload, group recipient, dynamic identifier는 허용하지
않는다.

## CLI/MCP 출력 계약

CLI JSON과 MCP result는 같은 projection result를 반환한다.

```json
{
  "ok": true,
  "id": "io-...",
  "request_key": "<sha256>",
  "state": "sent",
  "invoked": true,
  "diagnostic_code": "sent",
  "published_head": "<commit>",
  "message_id": "msg_...",
  "message_sequence": 42
}
```

Human output은 request key, state, diagnostic code, message receipt만 출력한다.
본문, session ID, mailbox handle은 기본 human output에 반복 노출하지 않는다.

## 오류 처리

- Confirm 누락: write/call 0회
- Invalid actor/path/state/phase/artifact/receipt: write/call 0회
- Empty, secret-only, control-containing, oversized body: write/call 0회
- History limit: write/call 0회
- Duplicate key: 기존 projection 반환, call 0회
- Intent write failure: call 0회
- Adapter validation failure: failed outcome 기록, call 0회
- Orca non-invoked failure: failed/invoked=false 기록
- Orca invoked timeout 또는 malformed response: failed/invoked=true 기록
- Post-call stale/missing projection: 결과 write 거부, 자동 재시도 0회

Message failure는 owner authority나 phase를 바꾸지 않는다. Source가 결과를 보고
새 요청을 작성할지는 별도 사용자 결정이다.

## 테스트 전략

### Core projection

- fresh exact-source session이 active PR handoff에 intent를 먼저 기록
- original coordinator가 아닌 새 source session 허용
- owner/wrong path/empty identity/completed/recovery/non-PR/no-artifact 거부
- body redaction, control rejection, 4096-byte boundary
- deterministic key와 cross-session duplicate idempotency
- 32-entry limit과 silent pruning 금지
- intent crash seam에서 Orca call 0회 및 같은 key no-retry
- sent, non-invoked failure, invoked timeout, malformed response outcome
- concurrent same-key append가 한 entry와 한 call만 생성
- post-call entry CAS가 unrelated record update를 보존
- current schema round-trip이 projection history를 보존

### Orca adapter

- exact literal argv와 fixed `status` type
- sealed coordinator/worker mailbox와 task/dispatch payload
- body가 한 argv로 전달됨
- response identity/payload exact match
- malformed JSON, oversized envelope, missing ID, non-positive sequence,
  wrong sender/recipient/type/body/task/dispatch 거부

### Lifecycle와 command parser

- exact typed source request만 허용
- missing/duplicate/unknown flag와 shell composition 거부
- owner, worker, sibling worktree, wrong session 거부
- CLI `feedback add`는 exact owner만 허용
- raw orchestration `--to/--from`이 protected record를 선택하고 차단
- MCP resource targeting이 CLI와 같은 결과
- raw terminal guidance와 arbitrary raw send 차단 유지

### Publication

- old receipt HEAD가 new HEAD의 ancestor면 exact push 후 receipt 교체
- non-fast-forward면 push 0회, receipt 불변
- ancestry reader 부재/error, identity mismatch면 push 0회
- same HEAD는 기존 idempotent readback 유지

### Contract와 문서

- CLI usage golden에 subcommand 추가
- existing `issueops_handoff` MCP action enum/property/handler test 갱신
- CLI/MCP response contract golden 갱신
- AGENT_WORKFLOW, ARCHITECTURE, OPERATIONS, TESTING, CAUTIONS가 같은 권한과
  recovery semantics를 설명

## 검증 명령

```bash
go test ./internal/core/issueops ./internal/core/issueops/handoff ./internal/adapter/orca ./internal/core/lifecycle ./internal/core/commandparse ./internal/adapter/mcp ./cmd/harness/issueopscli ./cmd/harness/mcpcli ./cmd/harness/harnessapp -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

설치된 Orca가 available/ready인 환경에서는 disposable cycle로 source request,
owner feedback, 수정 commit, fast-forward republish까지 확인한다. Live smoke는 기존
사용자 handoff나 terminal을 재사용하지 않고 cleanup 승인을 별도로 받는다.

## 문서 변경

- `.agent-harness/AGENT_WORKFLOW.md`: source disengagement의 typed correction 예외
- `.agent-harness/ARCHITECTURE.md`: durable projection과 CLI/MCP 표면
- `.agent-harness/OPERATIONS.md`: source request와 owner feedback/republish 순서
- `.agent-harness/TESTING.md`: crash seam, resource targeting, ancestry matrix
- `.agent-harness/CAUTIONS.md`: raw terminal guidance 예외 제거, typed path 사용
- `skills/issueops/references/worktree-context.md`: owner-active correction workflow
- 필요한 CLI/MCP usage 및 response-contract golden

## 거부한 대안

1. Source `feedback add` 허용: owner-only durable mutation 원칙을 깨고 전달 실패와
   feedback 기록이 원자적이지 않다.
2. Stateless direct send: crash/timeout 뒤 전달 여부를 재현할 수 없고 external
   mutation convention을 위반한다.
3. Raw orchestration allowlist: arbitrary type, subject, payload surface를 열고
   host별 CLI/MCP 계약을 분산시킨다.
4. PTY guidance injection: owner shell을 직접 steering하며 control-character와
   terminal rollover 위험을 되살린다.
5. Completed handoff re-open: immutable completion과 human cleanup boundary를
   무효화한다.

## 호환성과 위험

- 기존 schema 8 record는 새 optional history가 없어도 동일하게 유효하다.
- Projection이 기록된 뒤에는 이 optional field를 모르는 과거 binary로
  downgrade해 record를 rewrite하는 운영을 지원하지 않는다. Rollback code는
  field를 계속 decode/encode해 audit entry를 보존해야 한다.
- CLI/MCP에 additive action/subcommand만 추가하며 기존 flags를 바꾸지 않는다.
- Source authority가 original coordinator에서 any authenticated exact-source
  session으로 넓어지므로 hook actor match와 non-owner 검증이 핵심 보안 경계다.
- Persisted body는 redacted form이지만 workflow evidence이므로 4096-byte limit과
  32-entry cap으로 record growth를 제한한다.
- Orca message는 취소할 수 없으므로 intent 이후 자동 retry를 금지한다.
- Direct `git push`와 non-fast-forward publication 금지는 유지된다.

Rollback은 새 command/action을 제거하고 optional history read를 보존하는 방식으로
수행한다. 이미 전송된 Orca message나 persisted audit entry는 삭제하거나 되돌리지
않는다.
