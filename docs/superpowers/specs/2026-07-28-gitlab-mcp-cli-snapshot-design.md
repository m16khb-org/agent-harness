# GitLab MCP capability discovery와 glab CLI fallback 설계

## 상태

사용자가 승인한 구현 전 설계다. `glab-mcp-wrapper` 같은 개인용 실행 파일이나
MCP server 이름을 계약으로 삼지 않고, host agent가 발견한 일반 `glab_api`
capability와 일반 `glab api` CLI를 동일한 GitLab issue snapshot 경계에 연결한다.
개인 wrapper가 이 capability를 노출하면 공식·일반 registration과 똑같이 지원하고
우선 사용한다.

## 배경

IssueOps execution은 owner context를 봉인하거나 claim 시 본문 drift를 확인할 때
`ExecutionIssueSnapshotReader`를 사용한다. 현재 GitLab adapter는
`glab api projects/<project>/issues/<iid> --hostname <host>`만 실행한다.
따라서 Codex나 Claude Code에 GitLab MCP가 이미 등록되어 있어도 execution core는
그 capability를 소비할 수 없고, 로컬 `glab` credential이 대상 host/project와
맞지 않으면 MCP로는 읽을 수 있는 이슈를 CLI 경로에서 실패한다.

MCP server는 agent-harness의 child process나 dependency가 아니다. Host가 노출한
tool catalog에서 `glab_api` capability를 발견하고 호출할 수 있는 주체는 host
agent다. 현재 사용자 환경의 `glab-mcp-wrapper`도 이 capability를 제공하는 한
개의 등록 방식일 뿐이며, 다른 머신에서는 공식 `glab mcp serve`나 다른 server
registration이 같은 capability를 제공할 수 있다.

GitLab remote lookup과 Orca worktree identity는 별개다. Issue snapshot transport는
issue URL·본문·상태를 읽고, Orca adapter는 sealed marker와 worktree receipt로
provider/IID를 검증한다. Snapshot transport 실패를 Orca readiness 실패나
`mode=direct` 전환 사유로 합치지 않는다.

## 목표

- Codex와 Claude Code가 server namespace나 실행 파일 이름과 무관하게 일반
  `glab_api` MCP capability를 우선 사용할 수 있게 한다.
- 개인 wrapper가 `glab_api`를 노출하면 별도 설정이나 예외 코드 없이 같은 MCP
  candidate로 발견하고 사용할 수 있게 한다.
- MCP가 없거나 모든 read-only candidate가 인증·권한 문제로 실패할 때 일반
  `glab api` CLI를 fallback으로 사용한다.
- MCP와 CLI 결과를 하나의 bounded snapshot validation 경계에서 검증한다.
- `issueops_execution` MCP와 execution CLI가 같은 core request 의미를 공유한다.
- 선택된 snapshot source를 secret이나 개인 server identity 없이 관찰할 수 있게
  한다.
- 설치·업데이트·self-verify는 GitLab MCP나 `glab` 설치를 요구하지 않는다.

## 비목표

- `glab-mcp-wrapper`, profile 이름, token 환경 변수, 사용자 MCP config path를
  agent-harness에 등록하거나 특별 취급하지 않는다.
- agent-harness binary가 sibling MCP server를 직접 시작하거나 MCP tool catalog를
  탐색하지 않는다.
- GitLab MCP 또는 `glab`을 agent-harness install/update가 설치·인증하지 않는다.
- MCP server마다 별도 provider adapter를 만들지 않는다.
- GitLab issue snapshot transport로 Orca의 worktree/terminal/task 기능을
  복제하지 않는다.
- Snapshot source를 lease authority나 remote mutation authority로 사용하지 않는다.

여기서 개인 wrapper를 “특별 취급하지 않는다”는 것은 사용을 금지한다는 뜻이
아니다. Wrapper 이름·경로·profile을 하드코딩하지 않을 뿐, wrapper가 노출한
`glab_api`는 명시적인 지원 대상이다.

## 검토한 접근

### 1. Host-provided snapshot + 일반 glab CLI fallback — 채택

Host agent가 노출된 tool 중 semantic leaf capability가 `glab_api`이고 입력 schema가
일반 glab MCP의 `args`/`flags` 계약을 제공하는 candidate를 찾는다. Read 결과를
`issue_snapshot` evidence로 IssueOps에 넘기고, evidence가 아예 없을 때만 기존
GitLab provider adapter가 일반 `glab api`를 실행한다.

장점:

- MCP server 이름, wrapper path, credential profile을 core가 알 필요가 없다.
- Codex와 Claude Code가 같은 core validation과 execution state machine을 쓴다.
- MCP가 없는 머신에서도 기존 CLI 경로가 유지된다.
- Agent가 이미 가진 MCP capability를 binary가 역으로 호출하려는 계층 역전이 없다.

### 2. agent-harness가 `glab mcp serve`를 직접 실행 — 기각

Binary가 별도 MCP client와 process lifecycle, config discovery, credential
selection을 소유해야 한다. Host가 이미 관리하는 MCP 연결을 중복하고, 사용자
환경마다 다른 registration을 추측하게 되며 standalone install 불변식도 깨뜨린다.

### 3. `glab-mcp-wrapper` 이름 또는 server namespace를 직접 감지 — 기각

현재 머신에서는 동작할 수 있지만 다른 사용자·다른 머신에서 재현되지 않는다.
개인 path/profile/token naming이 public contract가 되고, 같은 `glab_api`
capability를 제공하는 공식 또는 다른 registration을 배제한다.

## 아키텍처

```text
Codex / Claude tool catalog
        |
        | leaf capability = glab_api인 read-only candidate를 bounded 탐색
        v
GitLab API payload
        |
        | issue_snapshot evidence
        v
issueops_execution MCP / execution CLI
        |
        | 동일한 provider·authority·project·IID·body bound 검증
        v
ExecutionIssueSnapshotReader
        |
        +-- evidence 있음  -> glab_mcp snapshot 사용
        |
        `-- evidence 없음  -> 일반 `glab api` CLI adapter
```

Host adapter는 capability 발견과 evidence 전달만 담당한다. Core는 어느 server나
실행 파일이 응답했는지 알지 못하며, 입력 evidence와 linked IssueOps record의
identity만 비교한다. CLI/MCP는 같은 `ExecutionActionRequest`로 수렴한다.

## Snapshot request 계약

`ExecutionActionRequest`에 선택적인 `issue_snapshot`을 추가한다.

```json
{
  "provider": "gitlab",
  "source": "glab_mcp",
  "web_url": "https://gitlab.example.com/group/project/-/issues/2609",
  "body": "원격 이슈 본문",
  "state": "opened"
}
```

필드 계약:

- `provider`: injected evidence에서는 `gitlab`만 허용한다. Record provider와
  반드시 같아야 한다.
- `source`: injected evidence에서는 `glab_mcp`만 허용한다. MCP server 이름이나
  wrapper 이름은 기록하지 않는다.
- `web_url`: GitLab API가 반환한 canonical issue URL이다.
- `body`: GitLab API의 `description` 원문이다.
- `state`: GitLab API의 `state`다.

MCP `issueops_execution`은 위 객체를 직접 받는다. CLI는
`--issue-snapshot-file <path>`로 동일 JSON 객체를 읽는다. CLI adapter는 소유자만
읽고 쓸 수 있는 regular file, non-symlink, 1 MiB 이하 JSON만 허용하고 이를 같은
core DTO로 변환한다.

Snapshot이 필요한 action은 `prepare`, `claim`, `replace --replace-action reseed`,
그리고 remote issue를 다시 읽는 `reconcile` 단계다. Snapshot을 소비하지 않는
`status`, `release`, `complete`나 reseed가 아닌 replace action에 evidence를
넘기면 무시하지 않고 validation error를 반환한다.

## Capability discovery 계약

Host agent와 공용 IssueOps skill/hook hint는 다음 순서를 따른다.

1. 현재 host가 노출한 MCP tool catalog에서 leaf tool 이름 또는 선언된 capability가
   `glab_api`인 candidate를 찾는다.
2. Server namespace, command path, wrapper 이름으로 candidate를 필터링하지 않는다.
3. Linked issue URL에서 exact hostname, project path, IID를 구하고
   `projects/<URL-path-escaped-project>/issues/<iid>` endpoint를 만든다.
4. 일반 glab MCP schema에 맞춰 endpoint를 `args` 위치 인자로, hostname을
   `flags.hostname`으로 전달한다.
5. Candidate가 여러 개면 각 candidate를 최대 한 번만 read-only 호출한다. 응답의
   `web_url` identity가 정확히 맞는 첫 결과만 채택한다.
6. 한 candidate의 `401`, `404`, permission error, schema mismatch는 target issue
   부재의 증거로 보지 않는다. 다음 candidate를 확인하고, 모두 실패하면 CLI
   fallback을 사용한다.

현재 환경에서 `glab-mcp-wrapper`가 등록한 tool이 이 조건을 만족하면 agent는
지원 대상으로서 그 tool을 우선 사용할 수 있다. 이것은 wrapper 사용을 배제하는
것이 아니라 노출된 `glab_api` capability와 live response identity로 portable하게
선택하는 것이다.

탐색은 host agent가 수행한다. `agent-harness` hook은 네트워크를 호출하지 않고
GitLab 관련 prompt에 이 capability-first 순서를 bounded routing hint로 제공한다.
공용 skill은 Codex와 Claude Code에 같은 지시를 제공한다.

## Core validation

Injected MCP evidence와 CLI adapter 결과는 다음 검증을 공유한다.

- Linked record provider가 `gitlab`인지 확인한다.
- `web_url`이 canonical HTTPS URL이고 userinfo, query, fragment, control
  character가 없는지 확인한다.
- Record issue URL과 evidence `web_url`의 authority를 port까지, project path와
  IID를 exact match한다.
- 같은 authority/project/IID라면 `/issues/:iid`와 `/work_items/:iid`를 같은
  identity로 취급한다.
- `body`가 비어 있지 않고 512 KiB 이하인지 확인한다. 이는 현재
  `executionOwnerArtifactLimit/2`와 같다.
- `state`가 `opened` 또는 `closed`인지 확인한다.
- 기존 acceptance ID와 exact verification command block 검증을 그대로 적용한다.
- Core가 body SHA-256을 직접 계산한다. Caller가 보낸 digest나 timestamp를
  freshness 증거로 신뢰하지 않는다.

Evidence는 한 execution action 안에서만 사용한다. Prepare/claim/reseed/reconcile
각 action은 필요할 때 새 snapshot을 받아야 한다. Durable IssueOps record에는
기존과 같이 body digest와 sealed owner artifact만 남기고 raw MCP server
identity, token, profile, command path는 저장하지 않는다.

Injected evidence는 GitLab이 서명한 원격 receipt가 아니라 exact native host
actor가 전달한 read observation이다. Core는 remote provenance를 암호학적으로
증명한다고 주장하지 않고, actor/session/process/CWD fence와 confirm gate 안에서
identity·bound·digest를 검증한다. Prepare preview와 confirm은 `confirm` 외에 같은
snapshot bytes를 사용하고, claim/reseed/reconcile은 각 action 시점에 새로 읽은
evidence를 사용한다. 이 trust boundary를 `glab_cli`보다 강하다고 표시하지 않는다.

## Source 선택과 오류 처리

- 유효한 `issue_snapshot`이 있으면 CLI를 호출하지 않는다.
- `issue_snapshot`이 없으면 일반 GitLab provider adapter가 기존 bounded
  `glab api` CLI를 호출한다.
- 전달된 evidence가 malformed, oversized, provider mismatch, identity mismatch면
  CLI로 fallback하지 않고 fail-closed한다. 잘못된 MCP 결과를 다른 credential
  surface로 조용히 덮지 않는다.
- MCP candidate가 없거나 candidate read가 인증·권한 오류로 끝났다는 판단은 host
  agent가 하고, 이때 evidence를 생략해 CLI fallback을 요청한다.
- CLI도 unavailable/auth/permission failure면
  `gitlab_issue_snapshot_unavailable` 오류로 종료한다. Error에는 시도한 source
  종류와 redacted 원인만 포함하고 token·env·server path는 포함하지 않는다.
- Snapshot transport 실패는 Orca probe failure가 아니다. `mode=auto`를 direct로
  바꾸거나 이미 시작한 external intent를 재시도하지 않는다.
- External mutation 뒤 ambiguity는 기존 `execution reconcile` 계약을 그대로
  따른다.

## 관찰 가능성

Snapshot을 소비한 action result에는 `issue_snapshot_source`를
`glab_mcp` 또는 `glab_cli`로 제공한다. Prepare의 기존
`issue_body_sha256`과 함께 확인하면 어느 transport의 어떤 본문이 봉인됐는지
검증할 수 있다.

다음 값은 결과·로그·record에 남기지 않는다.

- MCP server namespace
- `glab-mcp-wrapper` 경로 또는 이름
- profile 이름
- token 및 token 환경 변수 이름/값
- raw MCP config

## Hook과 공용 skill 반영

- `hookprompt`에 GitLab 관련 prompt용 `glab_api` capability discovery hint를
  추가한다. Hint는 capability 우선, 일반 `glab` CLI fallback, exact response
  identity 검증을 설명한다.
- `skills/gitlab-usecase/SKILL.md`는 portable capability discovery를 기본 규칙으로
  둔다. 기존 profile-scoped 설명은 특정 환경의 optional 예시로만 보존하고 public
  contract로 승격하지 않는다.
- `skills/issueops/references/execution.md`는 GitLab snapshot이 필요한 action에서
  MCP evidence를 전달하는 예와 CLI fallback을 설명한다.
- MCP tool description/schema와 CLI help는 같은 field 의미와 실패 정책을
  설명한다.
- 개인 홈의 `glab-mcp` skill이나 `glab-mcp-wrapper` 파일은 수정하지 않는다.

## 테스트

구현은 TDD로 다음 regression을 먼저 실패시킨다.

1. 유효한 `glab_mcp` evidence가 있으면 provider CLI reader를 한 번도 호출하지
   않고 동일 body digest를 봉인한다.
2. Evidence가 없으면 일반 `glab api` adapter를 호출하고 source가 `glab_cli`로
   보고된다.
3. Malformed/oversized/provider mismatch/authority mismatch/project mismatch/IID
   mismatch evidence는 CLI fallback 없이 거부된다.
4. 같은 authority/project/IID의 `/issues`와 `/work_items` URL은 허용된다.
5. `prepare`, `claim`, reseed, reconcile은 injected reader를 사용하고,
   snapshot 비소비 action은 evidence를 거부한다.
6. MCP schema가 nested `issue_snapshot`을 노출하고 CLI
   `--issue-snapshot-file`이 같은 DTO를 만든다.
7. CLI snapshot file의 symlink, non-regular file, oversized input, invalid JSON을
   거부한다.
8. GitLab prompt hook result가 `glab_api` capability discovery를 안내하고
   `glab-mcp-wrapper` 문자열이나 개인 path를 포함하지 않는다.
9. 개인 wrapper를 포함한 임의 server namespace가 같은 `glab_api` leaf
   capability를 노출하면 동일한 candidate 규칙으로 선택된다.
10. 기존 GitHub snapshot, GitLab CLI snapshot, Orca marker/receipt, CLI/MCP
   response golden이 회귀하지 않는다.

구현 후 검증:

```bash
go test ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/hookprompt -count=1
go test ./cmd/harness/issueopscli ./cmd/harness/mcpcli ./internal/adapter/mcp -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
```

설치 surface 검증은 `ah update --json` 뒤 daemon을 갱신하고 Codex/Claude MCP
catalog에서 변경된 `issueops_execution` schema를 다시 읽는다. GitLab live smoke는
server 이름이 아니라 실제 노출된 `glab_api` capability를 사용해 exact issue를
읽고 evidence를 전달한 결과가 `issue_snapshot_source=glab_mcp`,
`resolved_mode=orca`인지 확인한다. 별도 격리된 CLI credential fixture에서는
evidence를 생략해 `issue_snapshot_source=glab_cli`를 확인한다.

## 완료 기준

- Repo code, hook, 공용 skill 어디에도 `glab-mcp-wrapper` path/profile 의존성이
  없다.
- 현재 등록된 개인 wrapper가 노출한 `glab_api`도 일반 capability와 동일하게
  발견·호출할 수 있다.
- 임의 namespace의 일반 `glab_api` MCP 결과를 IssueOps가 bounded evidence로
  소비한다.
- MCP가 없는 환경에서는 일반 `glab api` CLI가 같은 snapshot contract를
  충족한다.
- Invalid supplied evidence는 fallback 없이 fail-closed한다.
- Snapshot transport failure가 `mode=direct` 선택으로 오분류되지 않는다.
- CLI/MCP request와 response golden, targeted/full/race/vet/build가 통과한다.
- `ah update`와 daemon refresh 뒤 Codex/Claude installed surface에서 동일 schema가
  관찰된다.
- #2609 live preview가 MCP evidence로 `resolved_mode=orca`를 유지한다.
- OpenWiki 자동 update는 실행하지 않는다.
