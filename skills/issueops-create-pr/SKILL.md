---
name: issueops-create-pr
description: Create and verify a linked GitHub pull request or GitLab merge request from an IssueOps execution lease with a readable Korean body, branch and actor fencing, scored metadata, safe recovery, and explicit good and bad examples. Use during the PR or MR publication phase.
---

# IssueOps Create PR

이 스킬은 **연결된 IssueOps cycle의 PR/MR 하나를 publish하는 단계**다.
Issue와 child는 [`issueops-create-issue`](../issueops-create-issue/SKILL.md)가
만든다. 전체 lifecycle은 [`issueops`](../issueops/SKILL.md)가 소유한다.

publish는 완료가 아니다. 이 스킬이 만드는 것은 draft이며, 그 draft를 근거로
완료 증거를 봉인하고 generation을 반납하는 단계는
[`issueops-complete`](../issueops-complete/SKILL.md)가 소유한다.

GitHub의 PR과 GitLab의 MR은 같은 publication 계약을 쓴다. CLI의 canonical
동사는 `remote create-pr`이며, 별도의 `create-mr` alias는 만들지 않는다.

provider 세부 규칙과 원격 본문 한국어 규칙은
[`remote-issue.md`](../issueops/references/remote-issue.md)가 소유한다.

## 읽는 순서

리뷰어가 처음 보는 순서를 고정한다.

1. **의도·이슈**: 왜 바꾸는지와 어느 Issue를 닫는지
2. **변경 사항**: 무엇이 바뀌었는지, 무엇은 안 바뀌었는지
3. **검증**: 실행 명령과 결과
4. **리뷰어 초점·위험**: 무엇을 집중해서 봐야 하는지와 rollback

나머지 canonical section은 빠뜨리지 않되, 같은 내용을 여러 section에
복사하지 않는다. 작은 변경에 큰 다이어그램을 넣지 않는다.

템플릿 설계의 근거는 [`issueops-create-issue`](../issueops-create-issue/SKILL.md)의
[GitHub](https://docs.github.com/en/communities/using-templates-to-encourage-useful-issues-and-pull-requests)
및 [GitLab](https://docs.gitlab.com/user/project/description_templates/)
공식 문서 링크를 따른다. PR과 MR은 provider만 다르고 읽는 순서는 같게 둔다.

## 흐름

```mermaid
flowchart LR
  a["연결된 이슈"] --> b["generation·actor 확인"]
  b --> c["head/base 확인"]
  c --> d["읽기 쉬운 본문 미리보기"]
  d --> e{"critical 검증"}
  e -->|실패| f["중지·수정"]
  f --> d
  e -->|통과| g["같은 요청으로 생성"]
  g --> h["실제 PR/MR 재확인"]
```

## 시작 게이트

다음 하나라도 없으면 이 스킬을 실행하지 않는다.

- record의 canonical `issue_url`과 issue phase evidence
- current `Execution`의 generation, mode, canonical worktree, native holder
- 현재 branch와 일치하는 `head`, sealed target과 일치하는 `base`
- provider/auth, project authority, label/assignee
- native host, session, process, cwd identity
- 봉인된 `project_docs_review` 판정과, 스키마 변경이 있다면 `schema_evidence`

`project_docs_review`·`schema_evidence`가 없거나 `_stale`로 뜨면 PR을 만들지
말고 [`issueops-implement`](../issueops-implement/SKILL.md)의 publication
evidence gates로 돌아간다. stale은 봉인 이후 diff가 바뀌었다는 뜻이므로,
문서를 다시 대조하고 최신 fingerprint로 재기록해야 한다.

`expected-generation`은 현재 lease와 같아야 한다. branch를 새로 만들거나
moving default branch를 추측하지 않는다. label score의 선택/거절과 threshold는
body 또는 durable completion evidence에 남긴다.

## Body 형식

`pull_request` template의 13개 canonical section을 유지하되, 각 section은
한 가지 질문에만 답한다.

| Section 묶음 | 답할 질문 | 권장 포맷 |
|---|---|---|
| 의도·이슈 | 왜, 무엇과 연결되는가 | 2~3문장 + Issue URL |
| 변경 유형·변경 사항 | 무엇을 바꿨는가 | checklist + 짧은 목록 |
| 검증 | 어떻게 확인했는가 | 명령 / 결과 표 |
| 리뷰어 초점 | 어디를 집중해서 볼까 | 2~4개 bullet |
| 위험·Breaking Changes | 무엇이 깨질 수 있나 | risk / rollback 표 |
| 사용자 영향·문서 | 누가 영향을 받나 | 영향과 migration 한 문단 |
| 범위·정리·자동화 | 범위를 지켰나 | 사실만 bullet |

body 초안을 만든 다음, `remote create-pr`을 실행하기 전에 `fluent-korean`
스킬을 Skill 도구로 호출해서 문장을 다듬는다. 한국어 게이트는 한글 비율만
보기 때문에 AI가 쓴 티는 걸러지지 않는다. 이 호출을 건너뛴 body로는
`--confirm`을 붙이지 않는다.

### 좋은 예: PR/MR body

```markdown
## 의도
Issue와 PR/MR publication 책임을 전용 스킬로 분리해 첫 읽기 비용을 낮춘다.

## 이슈
Closes https://github.com/acme/agent-harness/issues/123

## 변경 유형
- [x] refactor
- [x] docs

## 변경 사항
- `issueops-create-issue`와 `issueops-create-pr`을 분리했다.
- remote metadata와 template 조합을 fail-closed로 검증한다.

## 검증
| 명령 | 결과 |
|---|---|
| `go test ./internal/domain/artifacttemplate ./cmd/harness/issueopscli/remotecmd -count=1` | pass |
| `python3 scripts/verify-skill-shell.py skills/issueops-create-pr` | pass |

## 리뷰어 초점
- `create-pr`가 GitHub PR과 GitLab MR 모두에 같은 경계를 쓰는가
- generation과 native actor가 provider 호출 전에 확인되는가

## 위험/rollback
confirm 전에는 dry-run이다. 실패한 remote mutation은 retry하지 않고 reconcile한다.

## Breaking Changes
- [x] 없음

## 사용자 영향/릴리즈 노트
IssueOps 사용자가 필요한 단계만 읽고 publication할 수 있다.

## 문서/마이그레이션
두 전용 skill 링크와 provider 가이드를 갱신했다.

## 범위 관리
provider adapter API는 건드리지 않고 입력 경계만 수정했다.

## 워크트리 정리
merge 후 별도 cleanup gate에서 worktree와 branch를 확인한다.

## 자동화/AI 개입 근거
renderer와 deterministic test 결과를 기록한다.
```

각 section은 한두 문장으로 충분하다. 같은 내용을 여러 section에 복사하지 않는다.
검증하지 않은 `pass`나 생성한 URL을 적지 않는다.

### 나쁜 예

| 나쁜 입력 | 문제 |
|---|---|
| `테스트 완료` | 재현 가능한 명령과 결과가 없다 |
| 파일 30개 나열 | 변경 이유·경계·리뷰 포인트가 보이지 않는다 |
| Issue 링크 없음 | publication이 어느 작업인지 연결되지 않는다 |
| `gh pr create` / `glab mr create` 직접 실행 | IssueOps lease와 readback을 우회한다 |
| `head=main`, `base=feature/*` | 방향이 뒤집혔거나 moving ref를 추측한다 |
| timeout 뒤 create-pr 재실행 | duplicate PR/MR 위험이 있다 |
| 긴 로그와 token 첨부 | durable body에 secret이 남는다 |

## Canonical publication

body file을 먼저 작성하고, 다음 명령은 preview로 실행한다. `--confirm` 없는
경로는 provider mutation을 하지 않는다.

```bash
agent-harness issueops remote create-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --provider "$PROVIDER" --title "[refactor] IssueOps publication 경계를 분리한다" \
  --head "$HEAD_BRANCH" --base "$BASE_BRANCH" \
  --template pull_request --body-file "$BODY_FILE" \
  --label enhancement --assignee "$ASSIGNEE" \
  --host "$HOST" --session-id "$SESSION_ID" --agent-id "$AGENT_ID" \
  --session-pid "$SESSION_PID" --session-started-at "$SESSION_STARTED_AT" \
  --session-executable "$SESSION_EXECUTABLE" --cwd "$WORKER_PATH" --json
```

preview와 같은 명령에만 `--confirm`을 추가한다. 성공 후
`remote verify-artifact`와 provider readback으로 URL, source/target branch,
Issue linkage, labels, assignee, body를 확인한다.

```bash
agent-harness issueops remote verify-artifact \
  --id "$ISSUEOPS_ID" --provider "$PROVIDER" --kind "$ARTIFACT_KIND" \
  --url "$PR_OR_MR_URL" --target-branch "$BASE_BRANCH" \
  --label enhancement --assignee "$ASSIGNEE" --json
```

`ARTIFACT_KIND`는 GitHub에서 `pr`, GitLab에서 `mr`이다. 생성 command 이름과
검증 artifact kind를 혼동하지 않는다.

provider 결과가 불명확하면 create를 반복하지 않는다. execution의 reconcile
경로에서 후보를 검증한 뒤 하나만 연결한다.

## 품질·성능 게이트

- 품질: linked Issue, generation-CAS, head/base, actor, body completeness,
  원격 write 전 `fluent-korean` 호출, label/assignee, live artifact readback,
  secret redaction.
- 성능: publication 단계에서 issue creation과 전체 lifecycle reference를
  중복 로드하지 않는다. 변경 전후 byte 수와 focused 검증 시간을 기록할 수
  있지만 측정 없는 latency 개선 주장은 하지 않는다.
- 검증:

```bash
python3 scripts/validate-skill.py skills/issueops-create-pr
python3 scripts/verify-skill-shell.py skills/issueops-create-pr
wc -c skills/issueops-create-pr/SKILL.md
```

linked Issue, lease, branch pair, owner identity, body validation, provider
authority, label/assignee, 또는 live readback이 모호하면 publish하지 않는다.
