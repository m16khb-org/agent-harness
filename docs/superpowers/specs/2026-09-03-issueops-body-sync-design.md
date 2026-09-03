# IssueOps 원격 본문 최신화(sync) 설계

- 날짜: 2026-09-03
- 상태: accepted
- 범위: `issueops remote sync-issue`, `issueops remote sync-pr`, 스킬 `issueops-sync-issue`·`issueops-sync-pr`

## 문제

`issueops-create-issue`와 `issueops-create-pr`이 만든 원격 산출물은 사이클이
진행되면서 낡는다. 계획이 바뀌고, child가 늘고, 검증 명령이 달라져도 원격
본문은 생성 시점에 머문다. 그런데 현재 하네스에는 그 본문을 고칠 경계가 없다.

- `internal/port/provider.go`의 `UpdateIssueBodySection`은 섹션이
  `devils-advocate | completion` 닫힌 집합이다. 작성 본문은 대상이 아니다.
- PR/MR 본문에는 갱신 경로가 전혀 없다.
- 두 create 스킬은 raw `gh`/`glab` 우회를 금지한다. 경계가 없으면 중지가 규칙이다.

## 결정

원격 본문 교체를 provider capability → domain → application → CLI 순으로
수직 하나 신설하고, 그 위에 스킬 두 개를 얹는다.

### CLI 표면

```
agent-harness issueops remote sync-issue --id ID [--provider github|gitlab] [--url CHILD_URL]
  --body-file PATH [--expected-body-sha256 SHA] [--confirm] [--json]

agent-harness issueops remote sync-pr --id ID --expected-generation N [--provider github|gitlab]
  --body-file PATH [--expected-body-sha256 SHA]
  --host codex|claude|omo --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]
```

### 레이어

| 레이어 | 추가물 |
|---|---|
| `internal/contract/issueopsbodysync` | `Command`·`Plan`·`Result`·`Drift` DTO |
| `internal/port` | `IssueProviderArtifactBodyReader`·`…Replacer`·`IssueProviderChildHierarchyVerifier` 선택 인터페이스 |
| `internal/domain/issueopsbodysync` | 본문 정규화, 관리 블록 보존 병합, drift 분류, CAS 판정 |
| `internal/adapter/issueops` | 대상 해석·게이트·CAS·readback·기록 오케스트레이션 |
| `internal/adapter/provider/{github,gitlab}` | 세 인터페이스 구현 |
| `cmd/harness/issueopscli/remotecmd` | 플래그·dispatch·usage |

오케스트레이션은 별도 application vertical을 만들지 않고
`internal/adapter/issueops`에 둔다. 나머지 `remote` 동사(`reflect-devils-advocate`,
`reflect-completion`, `close-issue`)가 모두 그 자리에서 record·provider·lock을
묶고 있어, 새 vertical은 inbound/outbound 어댑터 한 벌을 더 만들 뿐 경계를
더 선명하게 하지 않는다.

`IssueProvider` 본체에 메서드를 더하지 않는다. 기존
`IssueProviderCreateIssueContexter`·`IssueProviderIssueCreateReconciler` 선례를
따라 선택 인터페이스로 확장해 모든 fake의 호환을 유지한다.

### 관리 블록 보존

전체 본문 교체는 `<!-- issueops:completion:start -->`를 지울 수 있고, cleanup
readiness가 그 마커를 readback 조건으로 쓴다(`port.IssueBodyCompletionStartMarker`).
따라서 병합 규칙을 도메인에 고정한다.

1. 원격 본문에서 `issueops:devils-advocate` 블록, `issueops:completion` 블록,
   `agent-harness:issue-create:<op>` 마커를 추출한다.
2. 제안 본문 뒤에 원래 등장 순서대로 재부착한다.
3. 제안 본문이 이 마커를 이미 품고 있으면 거부한다. 관리 영역은 저작 대상이 아니다.

### 충돌 정책 — fail-closed CAS

`--confirm`은 `--expected-body-sha256` 없이는 실패한다. 쓰기 직전 원격 본문의
sha256이 그 값과 다르면 중단한다. preview가 그 값을 알려준다. 사람이 원격에서
손댄 본문을 조용히 덮어쓰는 경로는 존재하지 않는다.

`drift`가 `remote_edited`면 CAS를 통과해도 `--accept-remote-edits` 없이는 쓰지
않는다. 외부 편집 위에 쓰는 일은 우연이 아니라 명시적 결정이어야 하고, 그
플래그가 없으면 사이클은 편집 내용을 먼저 읽게 된다.

다이제스트는 원문이 아니라 정규화된 본문(CRLF→LF, 끝 공백 제거)에서 낸다.
GitHub은 LF로 보낸 본문을 CRLF로 저장해 돌려주므로, 원문 비교는 아무도 고치지
않은 아티팩트를 첫 sync에서 `remote_edited`로 오탐한다.

`drift` 값은 세 가지다.

| 값 | 뜻 | 스킬의 행동 |
|---|---|---|
| `in_sync` | 기록 sha == 원격 sha == 병합 sha | 쓸 필요 없음 |
| `stale` | 기록 sha == 원격 sha, 병합 결과가 다름 | 그대로 최신화 |
| `remote_edited` | 기록 sha != 원격 sha | 사람 편집을 새 본문에 반영한 뒤에만 진행 |

### 대상 해석

- `sync-issue`: `--url` 없으면 `record.IssueURL`. `--url`이 있으면 그 URL이
  `record.IssueURL`의 provider-native child임을 검증한 뒤에만 쓴다.
- `sync-pr`: `record.RemoteArtifact.URL`만. `merged`/`closed` 상태면 거부하고,
  generation CAS와 native actor 펜스는 `create-pr`과 동일하다.

### 실패 순서

쓸 수 없는 본문(빈 본문, 관리 블록 포함)은 원격을 읽기 전에 거부한다. child
hierarchy 검증도 본문 읽기 전이다. provider 왕복은 판정에 필요할 때만 한다.

### 상태 기록

`IssueOpsRecord.BodySyncs`(URL당 최신 1건, 상한 16)를 추가한다. 다음 staleness
판정의 기준선이며, 한 번도 sync하지 않은 이슈의 기준선은
`IssueCreateIntent.BodySHA256`이다. `omitempty` 선택 필드라 기존 record와 호환된다.

## 비목표

- title·label·assignee 동기화. 요청 범위는 본문이다.
- CLI의 본문 자동 생성. 저작은 에이전트가 `fluent-korean`을 거쳐 수행한다.
- `--template/--field` 렌더링. 기존 `remote render-template`으로 충분하다.
- 전체 sync 이력 보존. 최신 항목만 남긴다.

## 검증

domain table test → orchestration 게이트 test → adapter fake-exec test →
CLI usage/spec parity(`IssueOpsUsageLines`, `IssueOpsCommandSpec`) →
lease stable_v1 미러 → `response_contracts.golden.json` 재생성 →
`validate-skill.py` / `verify-skill-shell.py` → `install` 후 `self-verify`.

## 위험

| 위험 | 완화 |
|---|---|
| 완료 블록 유실로 cleanup 게이트 붕괴 | 도메인 병합 규칙 + 보존 table test |
| 사람 편집 덮어쓰기 | `--expected-body-sha256` 필수 CAS |
| child가 아닌 URL에 쓰기 | hierarchy 검증 후에만 write |
| 머지된 PR 본문 변경 | state readback 후 거부 |
