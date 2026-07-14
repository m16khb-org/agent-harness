# Codex Kubectl Session Read-Only Exec Approval Design

## 목적

Codex에서 사용자가 승인한 진단용 `kubectl exec`를 같은 session, canonical repo, 명시적 kube context, namespace 범위에서 반복 허용한다. 허용은 결정론적 exact allowlist에 한정하고 마지막 허용 시점부터 30분 동안 유효하며, 허용할 때마다 만료 시각을 30분 뒤로 연장한다.

다음 기존 동작은 유지한다.

- `kubectl port-forward`는 exact-command one-shot 승인을 사용한다.
- Codex의 allowlist 밖 `kubectl exec`는 사용자 승인으로 우회할 수 없게 차단한다.
- 직접적인 mutating `kubectl` 명령은 GitOps 경계에서 차단한다.
- Claude와 native `ask`를 지원하는 host의 승인 흐름은 변경하지 않는다.
- 일반 read-only `kubectl`과 dry-run은 state 접근 없이 허용한다.

## 입력과 출력 계약

Session-scope 판정 입력은 Codex `PreToolUse`의 host, session ID, canonical repo, tool, shell command다. 후보 shell command는 다음 조건을 모두 만족해야 한다.

- 전체 shell command가 단일 `kubectl exec` 호출이다.
- `--context`와 `-n` 또는 `--namespace`가 각각 정확히 한 번 명시된다.
- kube context와 namespace 값이 비어 있지 않다.
- exec target은 하나이며 remote argv 앞에 `--`가 있다.
- stdin/TTY와 shell composition을 사용하지 않는다.
- remote argv가 아래 exact allowlist grammar 중 하나와 일치한다.

허용하는 local command 구조는 다음과 같다. Kubectl이 허용하는 범위에서 context/namespace flag 위치와 `--flag=value` 형태는 정규화하되, 같은 의미의 flag 중복은 거부한다.

```text
kubectl --context <context> --namespace <namespace> exec <target> [--container <container>] -- <allowlisted-remote-argv>
```

Global flag는 `--context`와 `-n`/`--namespace`, exec flag는 `-c`/`--container`만 허용한다. Env assignment, `sudo`, `timeout` 같은 command prefix와 그 밖의 kubectl/exec flag는 session grant 후보에서 제외한다.

판정 결과는 다음 셋 중 하나다.

- `allow`: 유효한 동일-scope grant가 있고 명령이 allowlist와 일치한다.
- `approval required`: 명령은 allowlist와 일치하지만 동일-scope grant가 없거나 만료됐다. Codex에서는 token 안내가 포함된 block으로 렌더한다.
- `block`: Codex 명령이 session-scope 후보 조건 또는 allowlist를 만족하지 않는다. Token을 발급하지 않으며 기존 grant를 소비하거나 갱신하지 않는다. Claude는 이 경우에도 기존 native `ask`를 유지한다.

`kubectl port-forward`는 이 판정에 들어오지 않고 기존 exact-command one-shot 흐름을 사용한다.

## Exact allowlist

Remote argv는 executable token과 인수 배열로 판정한다. Executable token은 slash 없는 `getent`, `nslookup`, `dig`, `cat`, `curl` 중 하나와 정확히 일치해야 한다. 임의 경로의 동명 바이너리와 alias 형태는 허용하지 않는다. 다음 grammar만 허용한다.

```text
getent hosts <dns-name>
nslookup <dns-name>
dig <dns-name>
dig +short <dns-name>
dig <dns-name> A
dig <dns-name> AAAA
dig +short <dns-name> A
dig +short <dns-name> AAAA
cat /etc/resolv.conf
curl -fsS http://localhost:4191/metrics
curl -fsS http://127.0.0.1:4191/metrics
```

`<dns-name>`은 전체 길이 253 이하의 ASCII DNS name 하나여야 한다. 각 dot-separated label은 영숫자로 시작하고 끝나며 내부에 영숫자와 `-`만 포함하고 길이는 1~63이다. 마지막 dot은 허용한다. 공백, slash, underscore, shell metacharacter, URL, option처럼 해석될 수 있는 leading `-`는 허용하지 않는다. 이 규칙은 Kubernetes short service name, dot-separated FQDN과 IPv4 textual form을 포함하고 IPv6 literal은 포함하지 않는다.

다음 항목은 명시적으로 차단한다.

- `sh -c`, `bash -c`와 모든 범용 shell
- `-i`, `-t`, `--stdin`, `--tty`
- `&&`, `||`, `;`, `|`, redirect, command substitution, background execution
- active parameter/tilde expansion, pathname expansion, process substitution, ANSI-C/special quoting
- `env`, `printenv`와 임의 path의 `cat`
- allowlist에 없는 `curl` URL 또는 option
- HTTP method/data/upload/header/config/output-file option
- custom DNS server, `dig -f`, output file처럼 추가 I/O를 만드는 DNS option
- remote argv 뒤에 붙은 추가 command 또는 인수

분류가 불가능하거나 새로운 진단 명령이 필요하면 read-only라고 추정하지 않는다. Allowlist를 코드와 테스트로 명시적으로 확장하기 전까지 차단한다.

## Scope와 fingerprint

Session grant scope는 다음 값을 length-delimited canonical encoding으로 결합해 SHA-256으로 계산한다.

- domain tag: `kubectl-readonly-exec-scope:v1`
- normalized host
- session ID
- canonical repo root
- explicit kube context
- explicit namespace

State에는 scope fingerprint만 저장한다. Raw command, session ID, repo path, kube context, namespace, workload, container, DNS name은 저장하지 않는다. 파일명에는 기존처럼 session ID의 SHA-256 hash를 사용한다.

같은 scope 안에서는 pod/deployment target과 container가 달라도 allowlist 명령을 실행할 수 있다. Context 또는 namespace가 달라지면 기존 grant를 사용할 수 없다. Session당 활성 exec scope는 하나만 유지하며, 다른 안전 scope의 요청은 기존 exec grant를 폐기하고 새 approval token을 만든다.

Working directory는 scope에 포함하지 않는다. Canonical repo 안에서 cwd가 달라져도 같은 session approval을 재사용하려는 의도다. Tool은 shell tool이어야 하지만 tool label 자체는 scope fingerprint에 포함하지 않는다.

## 승인 상태와 전이

Project-scoped user state의 session별 keyed lock 아래에서 다음 상태를 관리한다. Exec scope grant는 기존 one-shot file과 분리한다.

```text
kubectl-readonly-exec-approval-<session-hash>.json
kubectl-live-approval-<session-hash>.json          # 기존 port-forward one-shot
```

두 파일의 read-modify-write와 UserPromptSubmit token 조회는 같은 session approval lock으로 직렬화한다.

```json
{
  "schema_version": 2,
  "kind": "readonly_exec",
  "status": "pending",
  "token": "AH-XXXXXX",
  "request_fingerprint": "<sha256>",
  "scope_fingerprint": "<sha256>",
  "expires_at": "2026-07-14T12:00:00Z"
}
```

- `pending`: exact safe request와 scope에 결합되며 10분 뒤 만료된다. 같은 request 재시도는 token을 재사용한다.
- `granted`: 사용자 prompt가 exact token과 일치하면 같은 record가 session scope grant로 전환된다. Token과 request fingerprint를 record에서 제거하고 scope fingerprint만 권한 판정에 사용한다.
- 승인 후 첫 allow 전까지 grant activation TTL은 10분이다. 이 기간에 같은 scope의 allowlisted exec가 없으면 만료되고 새 승인이 필요하다.
- 유효한 `granted` record에서 같은 scope의 allowlisted exec를 허용할 때 `expires_at`을 현재 시각 + 30분으로 갱신한다.
- 마지막 허용 이후 30분이 지나면 grant는 만료된다. 단순 user prompt, 차단된 exec, 일반 kubectl, 실패한 분류는 TTL을 연장하지 않는다.
- 갱신 write가 성공한 뒤에만 `allow`를 반환한다. 실제 exec가 이후 실패하더라도 갱신을 되돌리지 않는다.
- 다른 scope의 안전한 exec 요청은 현재 exec record를 교체하고 새 pending token을 발급한다.

기존 `port-forward` one-shot record는 별도 state key와 기존 10분 pending/granted 소비 계약을 유지한다. Exec scope grant와 port-forward approval이 서로 교체하거나 소비하지 않는다. UserPromptSubmit은 같은 session lock 아래에서 두 pending record 중 token이 정확히 하나와 일치할 때만 해당 record를 승인하고, 일치가 없거나 둘 이상이면 fail-closed한다.

기존 schema v1 exec pending/granted record는 session grant로 승격하지 않는다. 다음 접근에서 유효 권한으로 해석하지 않고 새 schema의 pending flow를 시작한다. Schema v1 port-forward record는 기존 one-shot 경로에서만 해석한다.

## 컴포넌트 경계

### `internal/core/commandguard`

- 전체 shell input이 단일 `kubectl exec`인지 검증한다.
- global context/namespace와 exec target/container/remote argv를 구조화한다.
- 기존 `commandparse`의 active control, command/process substitution, output redirect, parameter/tilde/pathname expansion, special quoting 탐지를 재사용한다.
- shell composition, interactive flag와 위험한 kubectl connection/auth override를 거부한다.
- exact allowlist grammar를 판정하고 scope 재료를 반환한다.
- allowlist 밖 remote argv를 `unsafe_exec`로 분류한다. Host별 block/ask 결정은 lifecycle에 맡긴다.

### `internal/core/lifecycle/liveapproval`

- 기존 exact-command one-shot approval을 `port-forward`용으로 보존한다.
- read-only exec pending token과 session-scope grant 상태 머신을 소유한다.
- scope/request fingerprint, 10분 pending TTL, 30분 sliding grant TTL을 관리한다.
- keyed lock 아래에서 grant 생성, 교체, 갱신, 만료를 원자적으로 처리한다.

### Lifecycle와 host adapter

- Codex `PreToolUse`는 commandguard 결과가 안전한 exec일 때만 read-only exec approval을 평가한다.
- Codex의 `unsafe_exec`는 token 없이 block하고 liveapproval을 호출하지 않는다.
- Claude는 기존 native `ask` 결과를 유지한다. Codex session grant가 Claude 동작을 바꾸지 않는다.
- Host-specific hook JSON schema와 required response fields는 변경하지 않는다.

## 오류와 fail-closed 동작

- session ID, canonical repo, explicit context 또는 namespace가 없으면 session grant를 만들거나 사용하지 않는다.
- 중복 context/namespace, value 누락, kubeconfig/server/token/user/impersonation override는 차단한다.
- State init/read/write/lock 오류가 나면 allow하지 않는다.
- Corrupt, unknown 또는 future schema record는 권한으로 인정하지 않는다.
- 만료 record는 lock 아래에서 제거하거나 새 pending record로 원자적으로 교체한다.
- 동일 grant를 병렬로 사용하는 요청은 lock으로 직렬화한다. 각 허용은 이전 write를 관찰하고 만료를 30분 뒤로 갱신한다.
- Scope mismatch는 기존 grant를 다른 context/namespace에 적용하지 않는다.
- Block reason과 approval context에는 raw command, fingerprint 또는 cluster 식별자를 추가로 노출하지 않는다.

## 사용자 흐름

1. Agent가 explicit context/namespace를 포함한 allowlisted `kubectl exec`를 호출한다.
2. Grant가 없으면 Codex hook이 명령을 차단하고 `승인 AH-XXXXXX`를 안내한다.
3. 사용자가 같은 session에서 exact token을 입력한다.
4. UserPromptSubmit이 해당 request의 scope grant를 기록한다.
5. Agent가 같은 scope의 allowlisted exec를 호출하면 hook이 만료를 현재 시각 + 30분으로 갱신하고 허용한다.
6. 이후 같은 scope의 allowlisted exec도 별도 승인 없이 허용되고 각각 TTL을 갱신한다.
7. 30분 동안 허용된 exec가 없거나 scope가 바뀌면 새 승인이 필요하다.

## 테스트 전략

### Commandguard tests

- 허용 grammar 각각의 table test
- short service name과 FQDN 검증
- context/namespace flag 위치와 `--flag=value` 형태
- context/namespace 누락, 중복, 빈 값 차단
- interactive exec, shell composition, redirect, substitution 차단
- generic shell, env/secret/path 조회와 curl/dig option 우회 차단
- 여러 kubectl 호출 또는 다른 shell command와 결합된 입력 차단
- mutating kubectl, 일반 read-only kubectl, dry-run 기존 matrix 회귀

### Liveapproval tests

- pending token 생성, 동일 request 재사용, 10분 만료
- exact approval prompt만 scope grant 생성
- 승인 후 10분 activation TTL 안에 첫 allow가 없으면 만료
- 같은 scope의 서로 다른 target/container/allowlisted argv 반복 허용
- 각 allow가 fake clock 기준 만료를 30분 뒤로 연장
- 30분 idle 뒤 새 token 요구
- 다른 session/repo/context/namespace 격리
- 다른 scope 요청이 기존 grant를 교체
- 차단된 명령과 일반 prompt가 TTL을 갱신하지 않음
- concurrent allow의 lock 직렬화와 유효 expiry write
- state write/lock 오류, corrupt/future schema의 fail-closed 동작
- record에 raw command와 cluster 식별자가 없고 mode가 `0600`임
- port-forward가 계속 정확히 한 번만 허용되고 exec grant와 독립적임

### Hook integration tests

- Codex 최초 block -> token 승인 -> 동일 scope의 여러 safe exec allow
- Codex scope 변경과 TTL 만료 시 재승인
- Codex unsafe exec는 token 없이 block
- Claude live access는 계속 native `ask`
- 기존 response contract golden과 hook JSON shape 회귀

### 검증 명령

```bash
go test ./internal/core/commandguard ./internal/core/lifecycle/... ./cmd/harness/hookcli ./internal/adapter/hook -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

## 문서 반영

구현과 함께 `.agent-harness/CAUTIONS.md`와 `.agent-harness/OPERATIONS.md`의 one-shot exec 설명을 session-scoped exact allowlist와 30분 sliding TTL 계약으로 갱신한다. Port-forward one-shot과 Claude native ask 보존을 명시한다.

## 범위 밖

- LLM 또는 자연어 기반 read-only 판정
- 임의 shell command 승인
- Allowlist 밖 exec를 사용자 token만으로 우회
- 여러 context/namespace grant 동시 유지
- Web UI, remote approval, approval history
- Mutating kubectl 승인
- Port-forward의 session grant 전환
- Claude native ask 대체
