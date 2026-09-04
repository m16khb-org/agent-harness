---
name: issueops-sync-pr
description: Refresh the body of a GitHub pull request or GitLab merge request this cycle already published, with generation and actor fencing, drift classification, a fail-closed body compare-and-swap, and explicit good and bad examples. Use when the diff moved on and the PR/MR description did not.
---

# IssueOps Sync PR

이 스킬의 일은 **이미 publish한 PR/MR의 본문을 지금 변경 사항에 맞게 다시
쓰는 것**이다. 새 PR/MR을 만들거나, 구현하거나, 완료를 기록하지 않는다.

- PR/MR publication: [`issueops-create-pr`](../issueops-create-pr/SKILL.md)
- Issue·child 본문 최신화: [`issueops-sync-issue`](../issueops-sync-issue/SKILL.md)
- 완료 기록·generation 반납: [`issueops-complete`](../issueops-complete/SKILL.md)
- 원격 쓰기 절차: [`issueops-remote-write`](../issueops-remote-write/SKILL.md)
- provider 세부 규칙: [`remote-issue.md`](../issueops/references/remote-issue.md)

시작 전 `issueops next --id "$ISSUEOPS_ID" --json`으로 현재 단계를
확인한다. 본문 동기화는 어느 단계에서든 할 수 있다.

GitHub PR과 GitLab MR은 같은 계약을 쓴다. CLI의 canonical 동사는
`remote sync-pr` 하나이며 `sync-mr` alias는 없다.

## 언제 쓰는가

draft를 올린 뒤에도 커밋은 쌓인다. 다음 중 하나가 사실이면 본문이 낡은 것이다.

| 신호 | 본문에서 낡은 부분 |
|---|---|
| 리뷰 피드백을 반영해 구현이 바뀌었다 | `## 변경 사항`, `## 리뷰어 초점` |
| 검증 명령·결과가 달라졌다 | `## 검증` 표 |
| target branch를 retarget했다 | `## 위험/rollback`, base 설명 |
| breaking change 판단이 바뀌었다 | `## Breaking Changes`, `## 사용자 영향/릴리즈 노트` |
| 문서·마이그레이션을 추가했다 | `## 문서/마이그레이션` |

머지되거나 닫힌 artifact의 본문은 다시 쓰지 않는다. 하네스가 state readback으로
거부한다.

## 시작 게이트

다음 하나라도 없으면 실행하지 않는다.

- record의 검증된 `remote_artifact`(URL과 kind). 없으면
  [`issueops-create-pr`](../issueops-create-pr/SKILL.md)이 먼저다.
- 현재 `Execution`의 generation과 native holder. `--expected-generation`은
  현재 lease와 같아야 한다.
- canonical worktree에서의 실행. `--cwd`는 실제 프로세스 cwd와 같아야 한다.
- 원격 본문을 읽은 뒤 확정한 새 본문. preview 없이 본문을 고정하지 않는다.

`--url`로 다른 PR/MR을 가리키지 않는다. 이 스킬은 **이 사이클이 검증한 artifact
하나**만 다룬다.

## drift를 읽는 법

```bash
issueops remote sync-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --body-file "$BODY_FILE" --json
```

| `drift` | 뜻 | 다음 행동 |
|---|---|---|
| `in_sync` | 제안 본문이 원격과 같다 | 쓰지 않는다 |
| `stale` | 원격은 하네스가 마지막에 쓴 그대로다 | 그대로 confirm |
| `remote_edited` | 리뷰어나 봇이 본문을 고쳤다 | 그 편집을 새 본문에 반영한 뒤 `--accept-remote-edits` |

PR/MR은 리뷰어가 본문에 체크리스트나 메모를 직접 추가하는 일이 잦다.
`remote_edited`를 "덮어써도 되는 잡음"으로 취급하지 않는다.

## Body 형식

`pull_request` template의 canonical section을 유지한다. 최신화는 **본문 전체
교체**이므로, 지금 사실이 아닌 문장은 남기지 않는다. 특히 `## 검증` 표에
실행하지 않은 결과를 남겨 두지 않는다.

원격에 쓰기 전에 `fluent-korean` 스킬을 Skill 도구로 호출해서 문장을 다듬는다.
이 호출을 건너뛴 body로는 `--confirm`을 붙이지 않는다.

### 좋은 예: 리뷰 반영 후 최신화

```markdown
## 의도
IssueOps가 만든 원격 본문을 사이클 진행에 맞춰 다시 쓸 수 있게 한다.

## 이슈
Closes https://github.com/acme/repo/issues/412

## 변경 사항
- 본문 병합·drift 판정을 도메인으로 옮기고 관리 블록을 보존한다.
- 리뷰 반영으로 `--accept-remote-edits`를 추가해 사람 편집을 명시 확인한다.

## 검증
| 명령 | 결과 |
|---|---|
| `go test ./internal/domain/issueopsbodysync -count=1` | pass |
| `go test ./internal/adapter/issueops -run TestSync -count=1` | pass |

## 리뷰어 초점
- compare-and-swap이 preview와 confirm 사이의 변경을 잡는가
- 완료 블록이 전체 교체 뒤에도 남는가

## 위험/rollback
confirm 전에는 dry-run이다. sha가 어긋나면 쓰지 않고 멈춘다.
```

### 나쁜 예

| 나쁜 입력 | 왜 나쁜가 |
|---|---|
| 낡은 검증 표를 그대로 둔 채 문단만 추가 | 본문이 통째로 교체되어 거짓 근거가 남는다 |
| `--expected-generation` 생략 | lease 밖 write가 된다. 거부된다 |
| 머지된 PR 본문 재작성 시도 | 완료된 기록을 흔든다. 거부된다 |
| 리뷰어가 쓴 체크리스트를 지우고 confirm | 리뷰 맥락이 사라진다 |
| `gh pr edit` / `glab mr update` 직접 실행 | lease·readback·기록을 우회한다 |
| preview 결과와 다른 body-file로 confirm | 검토한 본문과 다른 것이 올라간다 |

## Canonical sync

```bash
issueops remote sync-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --provider "$PROVIDER" --body-file "$BODY_FILE" \
  --expected-body-sha256 "$EXPECTED_BODY_SHA256" \
  --host "$HOST" --session-id "$SESSION_ID" --agent-id "$AGENT_ID" \
  --cwd "$WORKER_PATH" --confirm --json
```

confirm 결과의 `updated`, `remote_body_sha256`, `drift`를 확인하고 실제 artifact를
readback한다. 본문 최신화는 완료 기록이 아니다. 완료 증거 봉인과 generation
반납은 [`issueops-complete`](../issueops-complete/SKILL.md)가 소유한다.

## 품질·성능 게이트

- 품질: generation-CAS, native actor, artifact 수명 상태, drift 판정 기록,
  원격 write 전 `fluent-korean` 호출, compare-and-swap sha 일치,
  secret redaction, live readback.
- 성능: 최신화 단계에서만 이 스킬을 로드한다. issue 생성·완료 reference를
  함께 중복 로드하지 않는다.
- 검증:

```bash
python3 scripts/validate-skill.py skills/issueops-sync-pr
python3 scripts/verify-skill-shell.py skills/issueops-sync-pr
wc -c skills/issueops-sync-pr/SKILL.md
```

generation, actor, artifact 상태, 리뷰어 편집 반영 여부, body 완성도, 또는
live readback이 모호하면 쓰지 않는다.
