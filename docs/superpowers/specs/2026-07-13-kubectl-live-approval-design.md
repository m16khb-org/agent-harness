# Codex Kubectl Live-Access One-Shot Approval Design

## 목적

Codex의 `PreToolUse` hook은 `permissionDecision="ask"`를 지원하지 않으므로 현재 `kubectl exec`와 `kubectl port-forward` 요청을 매번 `block`으로 변환한다. 이 설계는 첫 시도에서 정확한 명령에 결합된 짧은 token을 발급하고, 사용자가 `승인 <token>`을 입력한 뒤 동일한 명령의 다음 한 번만 허용한다.

Claude의 native `ask` 동작, 직접적인 mutating `kubectl` 명령의 GitOps 차단, read-only `kubectl` 허용은 변경하지 않는다.

## 사용자 흐름

1. Agent가 Codex session에서 `kubectl exec` 또는 `kubectl port-forward`를 호출한다.
2. `PreToolUse`가 live-access 요청 fingerprint를 계산한다.
3. 유효한 pending record가 없으면 `crypto/rand`로 `AH-XXXXXX` 형식 token을 만들고 project-scoped user-state에 저장한다.
4. Hook은 명령을 차단하고 `승인 AH-XXXXXX`를 입력하라는 이유를 반환한다.
5. 같은 명령을 승인 전에 재시도하면 기존 pending token을 재사용한다.
6. 사용자가 정확히 `승인 AH-XXXXXX`를 입력하면 `UserPromptSubmit`이 같은 repo와 session의 pending record를 granted record로 전환한다.
7. Agent가 같은 명령을 다시 호출하면 `PreToolUse`가 lock 안에서 grant를 삭제하고 한 번만 허용한다.
8. 같은 명령을 다시 호출하면 새 token이 발급되고 다시 차단된다.

같은 session에서 승인 전에 다른 live-access 명령을 시도하면 기존 pending record를 폐기하고 새 fingerprint와 token으로 교체한다.

## 컴포넌트 경계

### `internal/core/lifecycle/liveapproval`

Host-neutral approval 상태 머신을 소유한다.

- exact request fingerprint 계산
- random token 생성
- pending record 생성과 동일 요청 token 재사용
- 사용자 승인 prompt 검증과 `pending -> granted` 전이
- granted record의 atomic one-shot 소비
- 만료 및 mismatch 처리

Clock과 token generator는 테스트에서 주입할 수 있어야 한다. Production 기본값은 `time.Now`와 `crypto/rand`를 사용한다.

### `cmd/harness/hookcli`

Host event와 core approval 상태 머신을 연결한다.

- Codex `PreToolUse`: live-access `ask` 결과를 받으면 pending token을 발급하거나 grant를 소비한다.
- `UserPromptSubmit`: exact approval prompt를 core에 전달하고 성공 또는 실패 context를 짧게 추가한다.
- Claude `PreToolUse`: 기존 `permissionDecision="ask"` 출력을 유지하고 one-shot state를 생성하지 않는다.

Host-specific JSON schema 선택은 계속 `internal/adapter/hook`이 소유한다. Approval 정책이나 state mutation을 adapter에 넣지 않는다.

## State contract

Runtime state는 target repo가 아니라 기존 lifecycle project namespace 아래에 둔다.

```text
$HARNESS_STATE_DIR/projects/<repo-id>/kubectl-live-approval-<session-hash>.json
```

파일명에는 raw session ID를 쓰지 않고 SHA-256 session hash를 사용한다. 파일 권한은 `0600`, parent directory는 `0700`이다. Session별 keyed lock을 잡은 상태에서 read-modify-write 또는 remove를 수행한다.

Record는 다음 의미의 필드만 가진다.

```json
{
  "schema_version": 1,
  "status": "pending",
  "token": "AH-XXXXXX",
  "request_fingerprint": "<sha256>",
  "expires_at": "2026-07-13T12:00:00Z"
}
```

`status`는 `pending` 또는 `granted`만 허용한다. Raw command, kube context, namespace, workload 이름, environment, secret은 record에 저장하지 않는다.

## Request fingerprint

Fingerprint는 다음 값을 길이 구분이 있는 canonical encoding으로 결합한 뒤 SHA-256으로 계산한다.

- schema/domain tag: `kubectl-live-approval:v1`
- host
- session ID
- canonical repo root
- request cwd
- tool name
- exact trimmed command

따라서 다른 session, workspace, cwd, tool 또는 command는 기존 grant를 소비할 수 없다. Whitespace나 quoting이 달라진 명령도 다른 명령으로 취급한다.

Token alphabet은 혼동하기 쉬운 `0`, `O`, `1`, `I`를 제외한다. `AH-` prefix 뒤에 random 문자 6개를 사용하며, token은 authorization secret이 아니라 사용자가 어떤 pending request를 승인하는지 식별하는 challenge다.

## 만료와 bounded state

- Pending과 granted record는 생성 또는 승인 시점부터 10분 후 만료된다.
- 만료 record는 다음 접근에서 lock 안에서 삭제한다.
- Session당 record는 최대 하나이므로 state가 무제한 증가하지 않는다.
- 새로운 live-access request는 같은 session의 기존 pending record를 교체한다.
- Granted record가 존재할 때 다른 request가 오면 grant를 소비하지 않고 폐기한 뒤 새 pending token을 발급한다. 이전 grant는 다시 사용할 수 없다.

## 승인 prompt 계약

허용되는 prompt는 공백을 trim한 뒤 다음 형식과 정확히 일치해야 한다.

```text
승인 AH-XXXXXX
```

Token 비교는 대소문자를 구분한다. 부가 문장, 여러 token, token만 입력, 단순 `승인`, 잘못된 token은 승인하지 않는다. Exact-match 규칙 때문에 Stop hook continuation, goal continuation, 인용된 transcript는 approval로 해석되지 않는다.

승인 성공 시 UserPromptSubmit context는 grant가 다음 동일 명령 한 번에만 유효함을 알린다. Token 부재, mismatch, 만료 시에는 실행 권한이 생성되지 않았음을 알린다. 어느 경우에도 raw command나 fingerprint를 출력하지 않는다.

## 오류와 fail-closed 동작

- Codex live-access 요청에 session ID, canonical repo 또는 유효한 lifecycle namespace가 없으면 token을 만들지 않고 block한다.
- State init/read/write/lock 오류가 나면 allow하지 않고 승인 state를 기록할 수 없다는 block reason을 반환한다.
- Corrupt 또는 unknown schema record는 grant로 인정하지 않고 fail-closed한다.
- Grant 소비는 lock 안에서 record 삭제가 성공한 뒤에만 allow를 반환한다.
- 삭제 후 tool이 실행되지 않거나 실패해도 grant를 복원하지 않는다. 사용자는 새 token으로 다시 승인한다.
- 동시에 두 PreToolUse 요청이 같은 grant를 소비하면 keyed lock으로 직렬화되어 하나만 allow된다. 다른 요청은 새 pending token과 함께 block된다.

## 기존 동작 보존

- `kubectl apply`, `delete`, `patch`, `rollout restart` 등 direct mutation은 계속 GitOps block이다. One-shot approval 대상이 아니다.
- `kubectl get`, `logs`, `diff`, dry-run은 state 접근 없이 계속 allow된다.
- Claude와 native ask를 지원하는 host는 기존 ask schema를 유지한다.
- `--enforce-gitops-kubectl`이 없으면 approval state를 읽거나 쓰지 않는다.
- Hook output의 기존 required fields와 host별 JSON shape를 변경하지 않는다.

## 테스트 전략

### Core tests

- 동일 request의 fingerprint 결정성
- session, repo, cwd, tool, command 각각의 mismatch
- pending 생성과 동일 request token 재사용
- 다른 request가 pending 또는 granted record를 교체하는 동작
- exact approval prompt만 granted로 전환
- pending/granted 10분 만료와 접근 시 cleanup
- grant의 정확히 한 번 소비
- concurrent consume에서 allow가 정확히 하나
- corrupt/future schema 및 state I/O 오류의 fail-closed 동작
- state record에 raw command가 없고 file mode가 `0600`임

### CLI and host tests

- Codex: 첫 block과 token 안내, 승인 UserPromptSubmit, 동일 명령 한 번 allow, 다음 시도 재차 block
- Codex: 잘못된 token, 다른 session, 다른 command는 allow하지 않음
- Claude: live-access가 계속 native `ask`이고 approval state를 생성하지 않음
- 기존 mutating/read-only/dry-run `kubectl` matrix 회귀
- UserPromptSubmit의 성공·실패 context가 host schema와 호환됨

### Verification commands

```bash
go test ./internal/core/lifecycle/... ./cmd/harness/hookcli ./internal/adapter/hook -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

## 범위 밖

- 임의 shell command를 위한 범용 approval framework
- 여러 pending request를 한 session에서 동시에 유지하는 queue
- Approval history 또는 audit log
- Web UI, notification, remote approval
- Mutating `kubectl` command의 승인 우회
- Claude native ask 흐름 대체

이 범위는 현재 Codex live-access deadlock을 제거하는 데 필요한 최소 기능으로 제한한다.
