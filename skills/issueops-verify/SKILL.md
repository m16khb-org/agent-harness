---
name: issueops-verify
description: Run the IssueOps verify stage on the sealed diff without touching any file. Re-run the gate ledger and the repository's verification battery read-only, record the conditional schema evidence, run the adversarial implementation review through issueops-review, re-check compatibility against the real diff, and prove strict PR readiness leaves only commit and push. Use when "issueops next" reports verify, or when the user says "검증 단계", "검증해줘", "리뷰 돌리고 검증".
---

# IssueOps Verify

이 스킬의 일은 **봉인된 변경이 정말 통과하는지 확인하고 그 판정을 기록하는 것**이다.
파일은 하나도 바꾸지 않는다. 고칠 것이 나오면 앞 단계로 돌아간다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 게이트 원장: [`gates-ledger`](../gates-ledger/SKILL.md)
- 적대 리뷰: [`issueops-review`](../issueops-review/SKILL.md)
- 이전 단계: [`issueops-docs`](../issueops-docs/SKILL.md)
- 커밋: [`atomic-commit-push`](../atomic-commit-push/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `verify`면 이 스킬이다. `clean`이나 `docs`면 봉인이나 문서 반영이
아직이므로 그 단계로 돌아간다.

## 파일을 만지지 않는다

change fingerprint는 `git diff <base>..HEAD`와 `git status`가 가리키는 **모든 경로의
내용 해시**다. untracked 파일도 들어간다. 그래서 이 단계에서 파일을 하나라도 바꾸면
정리 단계의 봉인과 문서 단계의 판정이 동시에 stale이 된다.

이 단계의 명령은 전부 읽기이거나 record 기록이다. record 기록은 durable state를 바꾸지만
워크트리 파일을 바꾸지 않으므로 fingerprint에 영향이 없다.

검증이 실패해 코드를 고쳐야 하면 4단계로 돌아가 구현·정리·재봉인·문서 반영을 다시
밟는다. `next`가 `clean`으로 되돌리는 것이 그 신호다.

## 1 읽기 전용 재검증

```bash
# 원장은 다시 채우지 않고 읽기만 한다. EVIDENCE는 앞 단계가 채웠다.
issueops gates check --file "$LEDGER" --cwd "$WORKTREE" --workspace-root "$WORKTREE" --json

# 저장소가 정한 검증 battery를 실행한다. 이 저장소는 AGENTS.md가 그 목록을 소유한다.
issueops verify-work --json -- "$VERIFY_COMMAND"
```

- endpoint·DTO·OpenAPI가 바뀌었으면 `.issueops/OPEN_API_SPEC.md` 게이트를 적용하고
  `issueops api-doc check --json`을 실행한다. 대상 저장소에
  `npm run swagger:check` 같은 wrapper가 있으면 그것을 먼저 실행한다.
- `verify-work`는 실행한 명령과 결과를 evidence로 남긴다. 실행하지 않은 검증을 pass로
  적지 않는다.
- 미충족 게이트가 남으면 여기서 멈추고 4단계로 돌아간다. 원장을 고쳐 통과시키지 않는다.

## 2 스키마 실측과 기록

변경 집합에 마이그레이션·엔티티·`.sql`·`schema.prisma` 파일이 있을 때만 활성화된다.
없으면 이 게이트는 뜨지 않는다.

활성화되면 추정이 아니라 **실제 데이터베이스 관찰값**을 요구한다. 대상 테이블의 기존
인덱스 현황과 row 수, 그리고 그 값을 어디서 봤는지다. 조회는 [`database-design`](../database-design/SKILL.md)
또는 DB MCP 서버로 한다.

커넥션을 소모하는 대형 스캔을 던지지 않는다. `COUNT(*)` 전수 대신 카탈로그의 추정 row
수(`pg_class.reltuples`, `information_schema`, `SHOW INDEX`)를 쓰고 필요하면 `LIMIT`을
건다. 운영 DB에서 무거운 쿼리 하나가 커넥션 풀을 마르게 한다.

```bash
issueops schema-evidence record --id "$ISSUEOPS_ID" \
  --measurement "orders: 8.4M rows(reltuples), idx_orders_user_id 없음" \
  --source "mcp db-bc-prod execute_sql_bc_prod_market" \
  $RECORD_ACTOR_FLAGS --json
```

- measurement와 source는 짝이다. 출처 없는 수치는 추정과 구분되지 않는다.
- 관찰이 불가능하면 `--waive --waiver-rationale "<근거>"`로 남긴다. rationale 없는
  waive는 게이트를 열지 않는다.
- 실측 결과가 구현을 바꿔야 한다면 그것은 이 단계가 아니라 4단계의 일이다. row 수가
  크면 인덱스 생성 전략, 마이그레이션 잠금 시간, 백필 배치 크기가 달라진다.

## 3 구현 리뷰

[`issueops-review`](../issueops-review/SKILL.md)를 `--target diff`로 호출한다. 루프
절차는 그 스킬이 소유한다. 이 단계가 아는 것은 셋이다.

- 리뷰어에게 diff와 **계획을 함께** 준다. 무엇을 하기로 했는지 모르는 리뷰어는 구현이
  계획에서 벗어났는지 판정할 수 없다.
- `pass`만 통과한다. `revise`면 지적을 고쳐야 하므로 4단계로 돌아간다. 이 단계에서
  고치면 fingerprint가 바뀌어 앞 판정이 전부 stale이 된다.
- 모드에 따른 면제는 없다. execution이 있는 사이클은 전부 이 게이트의 대상이다.

## 4 호환성 재확인과 readiness

구현된 diff가 계획 시점의 compatibility review와 다르면 durable 판정을 최신으로 맞춘다.

```bash
issueops compatibility review --id "$ISSUEOPS_ID" \
  --backward-compatibility "<실제 diff 기준>" --side-effect "<실제 diff 기준>" \
  --rollback-plan "<실제 되돌리는 방법>" --verification "<이 단계에서 실행한 검증>" \
  --approved $RECORD_ACTOR_FLAGS --json

issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json
```

strict readiness의 `missing`이 `worktree_clean`, `upstream`, `upstream_fetch`,
`upstream_synced`의 부분집합이면 남은 일은 커밋과 푸시뿐이다. 그 밖의 키가 남으면
`next`가 가리키는 단계로 돌아간다. 특히 `gates_incomplete:*`와 `*_stale`은 앞 단계로
되돌아가라는 뜻이다.

## 출구

다음은 8단계 커밋·푸시다. [`atomic-commit-push`](../atomic-commit-push/SKILL.md)로
plan.md, gates.md, verified-execution report, 문서, 구현을 커밋·푸시하고, `next`가 렌더한
`phase --to pr`를 실행한다.

## 나쁜 예

| 나쁜 행동 | 문제 |
|---|---|
| 이 단계에서 파일을 고친다 | 앞 두 단계의 봉인과 판정이 동시에 stale이 된다 |
| 실행하지 않은 검증을 pass로 기록한다 | 기록은 남고 검증은 없다 |
| 리뷰 없이 구현 리뷰를 pass로 기록한다 | 게이트 연극이다 |
| 운영 DB에 `SELECT COUNT(*)` 전수 스캔을 던진다 | 커넥션 풀을 마르게 한다. 카탈로그 추정치를 쓴다 |
| direct 모드라서 구현 리뷰를 생략한다 | 모드 면제는 없다. 리뷰 없이 게시된 변경은 검토되지 않은 변경이다 |
| revise 지적을 이 단계에서 고치고 재리뷰를 생략한다 | fingerprint가 바뀌어 판정이 stale이 되거나, 검토되지 않은 수정이 게시된다 |
| 미충족 게이트를 원장 수정으로 통과시킨다 | 원장이 통과하고 결과는 검증되지 않는다 |
| 이 단계에서 커밋한다 | 커밋은 8단계다. 여기서 커밋하면 무엇을 검증한 상태인지 흐려진다 |

## 검증

- `issueops pr-readiness --id "$ISSUEOPS_ID" --strict --json`의 `missing`이
  커밋·푸시로 해소되는 키만 남았다.
- `issueops status --id "$ISSUEOPS_ID" --json`의 `implementation_review`와
  `schema_evidence`(활성화된 경우)가 현재 fingerprint에 묶여 있다.
- `git -C "$WORKTREE" status --porcelain`이 이 단계 시작 때와 같다. 달라졌으면 이 단계가
  파일을 바꾼 것이다.
