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
- 성공적으로 검증한 provider capability 사용법을 진행 중인 repo의
  `.agent-harness/VCS.md`에 남겨 다음 agent/session이 재사용하게 한다.
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

Snapshot이 필요한 action은 `prepare`, `claim`, `replace --reseed`, 그리고
pending `worktree_create` receipt에서 remote issue를 다시 읽는
`reconcile --confirm`이다. Reconcile preview, no-pending/remote-PR/owner-launch/
dispatch reconcile, `status`, `release`, `complete`, reseed가 아닌 replace
action에 evidence를 넘기면 무시하지 않고 validation error를 반환한다.

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

## Repo-local VCS.md 기록과 재사용

### 문서 위치와 생명주기

사용자가 말한 “진행 중인 repo의 `VCS.md`”는 프로젝트 문서 경계에 맞춰
`<repo>/.agent-harness/VCS.md`로 둔다. Root `VCS.md`를 새로 만들지 않는다.

현재 project-doc catalog에는 `VCS.md`가 없으므로 이를 **optional first-class
project doc**으로 추가한다. 다음 경계를 지킨다.

- `requiredProjectDocNames`와 `optionalProjectDocNames`를 분리하고,
  read/update allowlist만 두 목록의 합집합을 사용한다.
- 기존 `ProjectDocNames`/`PrefixedProjectDocNames`는 required 목록을 유지해
  bootstrap과 doctor의 의미를 바꾸지 않는다.
- `project_docs_read`와 SHA-checked `project_docs_update`가 읽기·쓰기를 지원한다.
- GitLab, GitHub, `glab`, `gh`, MCP, IssueOps remote 작업을 route할 때 파일이
  존재하면 우선 참고한다.
- 일반 bootstrap/doctor가 모든 repo에 빈 `VCS.md`를 강제로 만들거나 missing
  warning을 내지 않는다.
- 첫 검증된 capability가 생겼을 때 on-demand로 만들고, 이후 recipe가 실제로
  바뀔 때만 갱신한다.

### Provider-neutral 문서 계약

`VCS.md`는 GitLab 전용 문서가 아니다. Repo의 exact remote와 연결된 provider별
recipe를 같은 형식으로 기록한다.

- GitLab remote: GitLab MCP capability와 `glab` CLI recipe
- GitHub remote: GitHub MCP capability와 `gh` CLI recipe
- 여러 remote가 서로 다른 provider를 가리키면 remote name과 authority를 함께
  기록해 recipe를 분리한다.

각 recipe는 다음 공통 필드를 가진다.

- provider와 remote name/authority
- semantic operation
- transport 종류와 capability leaf
- request shape와 endpoint/argument template
- required response fields
- exact identity acceptance rule
- CLI fallback
- canonical recipe fingerprint

Server namespace, wrapper path, profile, token은 어느 provider에서도 recipe identity가
아니다.

### “발견”의 정의

Tool catalog에 이름이 보인 것만으로는 문서를 만들지 않는다. 다음 조건을 모두
만족한 첫 successful read를 발견으로 본다.

1. 현재 repo의 exact remote에서 provider와 project identity를 결정한다.
2. Provider별 read-only MCP capability 또는 bounded CLI command를 사용한다.
   이번 자동 snapshot transport 범위는 exact `glab_api`인 GitLab이다. GitHub는
   exact `gh issue view` 또는 현재 host에서 실제 호출·검증한 MCP issue-read
   schema만 문서화하고, 이름이나 schema를 미리 추측하지 않는다.
3. 호출 대상이 해당 remote host/project와 연결된다.
4. 응답 URL의 authority, project path, issue number/IID가 요청과 exact match한다.
5. 응답이 bounded JSON이고 secret-like field를 포함하지 않는다.
6. 사용한 request/response shape가 일반화 가능한 recipe로 정규화된다.

`401`, `404`, permission error, malformed response, identity mismatch는 발견
evidence가 아니며 `VCS.md`를 갱신하지 않는다.

### 기록 내용

`VCS.md`에는 재사용 가능한 provider-neutral recipe만 기록한다.

```markdown
## GitLab

### Issue snapshot read

- Provider: `gitlab`
- Remote: `origin`
- Remote host: `<repo remote host>`
- Capability: `glab_api`
- Endpoint: `projects/<URL-path-escaped-project>/issues/<iid>`
- MCP input: endpoint는 `args`, hostname은 `flags.hostname`
- Required response: `description`, `web_url`, `state`
- Accept: authority + project + IID exact match
- CLI fallback: `glab api <endpoint> --hostname <host>`
- Recipe fingerprint: `<canonical recipe SHA-256>`

## GitHub

### Issue snapshot read

- Provider: `github`
- Remote: `origin`
- Remote host: `<repo remote host>`
- Transport: `cli`
- Capability: `gh issue view`
- Arguments: `<canonical issue URL> --json url,body,state`
- Required response: `url`, `body`, `state`
- Accept: authority + owner/repo + issue number exact match
- Recipe fingerprint: `<canonical recipe SHA-256>`
```

GitHub MCP issue-read를 실제로 성공시킨 경우에만 별도 transport 항목으로 관찰한
semantic leaf, input schema, required response를 추가한다. 이번 구현은 존재하지
않는 공통 GitHub MCP 이름이나 schema를 만들지 않는다.

다음 값은 기록하지 않는다.

- MCP server namespace와 wrapper 이름·경로
- profile 이름과 token/env/config
- issue description과 다른 raw response body
- 실제 issue IID처럼 recipe 재사용에 불필요한 task-specific 값
- 인증 실패 stderr 또는 credential 진단 원문

Repo remote host와 project path는 이미 해당 repo의 VCS identity이므로 endpoint
template 구성에 사용할 수 있다. 개인 wrapper를 사용해 발견했어도 문서에는
portable capability leaf와 schema만 남긴다. GitHub MCP도 server namespace가
아니라 검증된 semantic capability와 request schema를 기록한다.

### Hook와 main-agent 역할

SessionStart catalog와 UserPrompt hook은 현재 repo에 `VCS.md`가 있으면 VCS remote
작업 전에 읽도록 안내한다. UserPrompt hook은 GitLab remote 작업에서 일반
`glab_api` capability discovery와 CLI fallback도 안내한다. Hook은 tool response를
원격 provenance로 승격하거나 shared 문서를 직접 수정하지 않는다.

Successful provider read를 실제로 확인한 main agent는:

1. 현재 작업의 exact repo와 write authority를 확인한다.
2. `project_docs_read(VCS.md)`로 current content/SHA를 읽는다.
3. 같은 recipe fingerprint가 있으면 no-op한다.
4. 새 recipe면 기존 내용을 보존해 병합하고
   `project_docs_update(expected_sha256, confirm=true)`로 쓴다.
5. Active IssueOps cycle이면 source checkout을 더럽히지 않고 active holder의
   canonical worktree에서만 갱신한다. 아직 holder/worktree가 없으면 문서를
   쓰지 않고 worktree 생성 뒤 successful read를 다시 검증한다.

동시 갱신의 SHA mismatch는 overwrite하지 않고 reread/merge한다. Hook은 network
read, git mutation, shared-doc write를 수행하지 않는다. Source checkout과 sibling
worktree가 서로 다른 lifecycle namespace를 쓰므로 이번 범위에서 PostToolUse
candidate queue나 cross-worktree handoff를 새로 만들지 않는다.

### 재사용과 stale 처리

다음 GitLab/GitHub/VCS 작업에서 agent는 `VCS.md`의 현재 repo/provider/remote와
일치하는 recipe를 먼저 읽어 endpoint와 schema를 재사용하되, 문서를 availability
또는 credential authority로 믿지 않는다. 현재 tool catalog에서 recipe의
capability candidate를 다시 발견하고 live response identity를 검증해야 한다.

- Recipe가 그대로 성공하면 문서를 다시 쓰지 않는다.
- Server namespace나 wrapper가 바뀌어도 recipe가 같으면 문서를 바꾸지 않는다.
- Input schema가 바뀌고 새 recipe가 live validation을 통과하면 fingerprint와
  recipe를 갱신한다.
- 문서 recipe가 실패하면 bounded rediscovery 후 CLI fallback을 사용한다.
- Auth/permission failure만으로 recipe를 삭제하지 않는다.

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

Evidence는 한 execution action 안에서만 사용한다. Prepare/claim/reseed와 pending
`worktree_create`의 `reconcile --confirm`은 필요할 때 새 snapshot을 받아야 한다.
Durable IssueOps record에는 기존과 같이 body digest와 sealed owner artifact만
남기고 raw MCP server identity, token, profile, command path는 저장하지 않는다.

Injected evidence는 GitLab이 서명한 원격 receipt가 아니라 exact native host
actor가 전달한 read observation이다. Core는 remote provenance를 암호학적으로
증명한다고 주장하지 않고, actor/session/process/CWD fence와 confirm gate 안에서
identity·bound·digest를 검증한다. Prepare preview와 confirm은 `confirm` 외에 같은
snapshot bytes를 사용하고, claim/reseed/reconcile은 각 action 시점에 새로 읽은
evidence를 사용한다. 여기서 reconcile은 pending `worktree_create`의 confirm만
뜻한다. 이 trust boundary를 `glab_cli`보다 강하다고 표시하지 않는다.

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

Supplied evidence를 core가 검증·선택한 action result 또는 provider fallback이
실제로 remote issue를 읽은 action result에는 `issue_snapshot_source`를
`glab_mcp` 또는 `glab_cli`로 제공한다. Prepare preview의 `glab_mcp`는 해당
snapshot이 identity/boundary validation을 통과한 입력이라는 뜻이지 이미 본문을
봉인했다는 뜻이 아니다. Confirm 결과의 `issue_body_sha256`과 함께 확인해야 어느
transport의 어떤 본문이 실제로 봉인됐는지 검증할 수 있다.

다음 값은 결과·로그·record에 남기지 않는다.

- MCP server namespace
- `glab-mcp-wrapper` 경로 또는 이름
- profile 이름
- token 및 token 환경 변수 이름/값
- raw MCP config

## Hook과 공용 skill 반영

- SessionStart project-doc catalog와 `hookprompt`에 provider-neutral `VCS.md`
  reuse hint와 GitLab 관련
  `glab_api` discovery hint를 추가한다. Hint는 repo provider recipe 우선,
  provider CLI fallback, exact response identity 검증을 설명한다.
- `skills/gitlab-usecase/SKILL.md`는 portable capability discovery를 기본 규칙으로
  둔다. 기존 profile-scoped 설명은 특정 환경의 optional 예시로만 보존하고 public
  contract로 승격하지 않는다.
- `skills/issueops/references/execution.md`는 GitLab snapshot이 필요한 action에서
  MCP evidence를 전달하는 예와 CLI fallback을 설명한다.
- Project-doc catalog는 optional `.agent-harness/VCS.md`를 read/update 대상으로
  허용하고 VCS/GitLab/GitHub task routing에 포함한다.
- Hook/skill 안내를 따른 main agent가 successful read 뒤 canonical worktree와
  SHA-CAS를 확인해 `VCS.md`를 materialize한다.
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
5. `prepare`, `claim`, reseed와 pending `worktree_create`의
   `reconcile --confirm`은 injected reader를 사용하고, snapshot 비소비 action은
   evidence를 거부한다.
6. MCP schema가 nested `issue_snapshot`을 노출하고 CLI
   `--issue-snapshot-file`이 같은 DTO를 만든다.
7. CLI snapshot file의 symlink, non-regular file, oversized input, invalid JSON을
   거부한다.
8. GitLab prompt hook result가 `glab_api` capability discovery를 안내하고
   `glab-mcp-wrapper` 문자열이나 개인 path를 포함하지 않는다.
9. Hook/skill 계약은 server namespace를 필터링하지 않고 `glab_api` leaf와
   live response identity만 사용한다. 현재 개인 wrapper는 설치 후 live smoke로
   같은 경로가 실제 호출되는지 확인한다.
10. `.agent-harness/VCS.md`는 optional project doc으로 read/update/route되지만
    bootstrap과 doctor가 모든 repo에 생성을 강제하지 않는다.
11. Required와 optional project-doc 목록이 분리되어 `VCS.md` allowlist 추가가
    기존 bootstrap/doctor required 목록을 바꾸지 않는다.
12. Hook result는 VCS remote 작업에서 기존 `VCS.md`를 먼저 읽고 successful
    discovery 뒤 SHA-CAS로 기록하라고 안내하며 shared doc을 직접 쓰지 않는다.
13. GitHub remote에서는 검증된 GitHub MCP/`gh` recipe를 기록할 수 있고 GitLab
    `glab_api` recipe가 provider match 없이 재사용되지 않는다.
14. 여러 provider remote가 있으면 remote authority별 recipe가 분리된다.
15. 기존 GitHub snapshot, GitLab CLI snapshot, Orca marker/receipt, CLI/MCP
    response golden이 회귀하지 않는다.

구현 후 검증:

```bash
go test ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/hookprompt ./internal/core/projectdoc ./internal/core/projectdocs ./internal/core/projectbootstrap ./internal/core/doctor -count=1
go test ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli ./cmd/harness/hookcli ./internal/adapter/mcp -count=1
go test -race ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/projectdoc ./internal/core/projectdocs -count=1
go vet ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/hookprompt ./internal/core/projectdoc ./internal/core/projectdocs ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli ./cmd/harness/hookcli ./internal/adapter/mcp
go build -o bin/agent-harness ./cmd/harness
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
```

사용자의 명시적 자원 제한 지시에 따라 머신 자원을 많이 쓰는 `go test ./...`,
전체 `-race`, full self-verify는 이번 작업에서 실행하지 않는다. 위 targeted
package와 contract/install/live surface 증거로 회귀 범위를 검증한다.

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
- Successful discovery의 portable recipe가 진행 중인 repo의
  `.agent-harness/VCS.md`에 secret 없이 기록되고 다음 session에서 route된다.
- `VCS.md`는 GitLab과 GitHub recipe를 provider/remote별로 기록하며 현재 repo의
  VCS와 맞는 recipe만 재사용한다.
- 같은 recipe 재사용은 문서 churn을 만들지 않고, wrapper/server 이름 변경도
  recipe fingerprint가 같으면 갱신 사유가 아니다.
- Hook은 읽기·기록 절차를 안내하되 shared doc을 직접 쓰지 않는다.
- 임의 namespace의 일반 `glab_api` MCP 결과를 IssueOps가 bounded evidence로
  소비한다.
- MCP가 없는 환경에서는 일반 `glab api` CLI가 같은 snapshot contract를
  충족한다.
- Invalid supplied evidence는 fallback 없이 fail-closed한다.
- Snapshot transport failure가 `mode=direct` 선택으로 오분류되지 않는다.
- CLI/MCP request와 response golden, targeted/race/vet/build가 통과한다.
- `ah update`와 daemon refresh 뒤 Codex/Claude installed surface에서 동일 schema가
  관찰된다.
- #2609 live preview가 MCP evidence로 `resolved_mode=orca`를 유지한다.
- OpenWiki 자동 update는 실행하지 않는다.
