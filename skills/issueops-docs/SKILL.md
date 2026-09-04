---
name: issueops-docs
description: Reflect a finished IssueOps implementation into the project's operating documents. Route the diff to the .issueops documents it touches, check both directions (did the change break a documented rule, and did it create a decision, pitfall, command, or convention the documents do not know yet), update ADR, CAUTIONS, CONVENTIONS, or ARCHITECTURE through the project_docs MCP contract, re-seal the ai-slop-clean fingerprint, and record the project-docs-review verdict. Use when "issueops next" reports docs, or when the user says "문서 반영", "ADR 남겨줘", "주의사항 기록", "update the project docs".
---

# IssueOps Docs

이 스킬의 일은 **끝난 구현을 운영 문서에 반영하는 것**이다. 코드는 고치지 않는다.
문서만 고치고, 그 수정이 바꾼 봉인을 다시 맞춘 뒤 판정을 기록한다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 문서 갱신 절차: [`project-docs-update`](../project-docs-update/SKILL.md)
- 이전 단계: [`issueops-clean`](../issueops-clean/SKILL.md)
- 다음 단계: [`issueops-verify`](../issueops-verify/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `docs`면 이 스킬이다. `clean`이면 정리와 봉인이 아직이므로
[`issueops-clean`](../issueops-clean/SKILL.md)으로 돌아간다.

## 왜 이 단계가 정리 뒤에 오는가

문서에 남길 결정과 함정은 **정리가 끝난 최종 diff**에서 확정된다. 구현 도중에 적으면
정리가 지운 것을 문서가 설명하게 된다.

동시에 문서 수정은 파일 변경이므로 정리 단계의 fingerprint 봉인을 stale로 만든다.
그래서 재봉인은 이 단계가 소유한다. 문서를 고치고 재봉인하지 않으면
`ai_slop_clean_stale`로 다음 단계가 전부 막힌다.

## 1 라우팅

구현 diff 요약을 만들어 읽을 문서를 고른다. 요약에는 변경 파일 목록, 새로 생긴
명령·플래그·구조, 작업 중 만난 함정을 넣는다.

```bash
git -C "$WORKTREE" diff --stat "$BASE_SHA"
# MCP: project_docs_route에 위 요약을 넣어 문서를 고른다.
# MCP: project_docs_read로 각 문서의 현재 content와 SHA를 받는다.
```

MCP를 쓸 수 없으면 `issueops docs --json`의 required-doc 목록에서 `CONSTITUTION.md`,
`ARCHITECTURE.md`(해당 모듈), `CONVENTIONS.md`, `CAUTIONS.md`(색인과 해당 모듈),
`ADR.md`, `TESTING.md`를 읽는다.

## 2 양방향 대조

한 방향만 보면 절반을 놓친다.

**문서 → 구현.** CONSTITUTION, CONVENTIONS, ARCHITECTURE, CAUTIONS가 정한 것을 이번
diff가 어겼는가. 어겼으면 **문서가 아니라 구현을 고친다.** 이때는 4단계
[`issueops-implement`](../issueops-implement/SKILL.md)로 돌아간다. 규칙을 지키기 어렵다는
이유로 규칙을 고치는 것은 이 단계의 권한이 아니다.

**구현 → 문서.** 이번 변경이 문서가 아직 모르는 것을 만들었는가.

- 구조 결정이나 대안 기각 사유가 생겼다 → ADR
- 다시 밟을 함정이나 재발한 문제를 해결했다 → CAUTIONS
- 새 명령·플래그·컨벤션·모듈 경계가 생겼다 → CONVENTIONS 또는 ARCHITECTURE
- 검증 방식이 바뀌었다 → TESTING

계획의 `## 적용되는 결정과 주의사항` 절과 대조한다. 계획 때 몰랐던 항목이 이 단계에서
찾은 것이고, 그것을 evidence에 적는다.

## 3 문서 수정

[`project-docs-update`](../project-docs-update/SKILL.md)를 호출한다. 도구는 셋이다.

| 대상 | 도구 |
|---|---|
| 결정 | `project_docs_append(kind=adr)` |
| 함정·주의사항 | `project_docs_append(kind=caution)` |
| 기존 문서의 내용 변경 | `project_docs_revise` (SHA-CAS, 한 문서씩) |

ADR은 append-only다. 기존 항목을 고치지 않고, 뒤집는 결정이면 그것을 새 항목으로
적는다.

하네스 저장소에서 `.issueops/*.md`를 고쳤으면 응답 계약 골든이 드리프트한다.
같은 단계에서 재생성한다.

```bash
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update -count=1
```

대상 저장소에 그 골든이 없으면 이 줄은 해당 없음이다.

## 4 재봉인

문서를 하나라도 고쳤으면 정리 단계의 봉인을 다시 맞춘다.

```bash
issueops ai-slop-clean record --id "$ISSUEOPS_ID" \
  --category "<5단계와 같은 category>" \
  --verification "<5단계의 검증 결과 그대로>" $RECORD_ACTOR_FLAGS --json
```

category는 정리 단계와 같고, verification도 그때의 결과를 그대로 적는다. 문서만
바뀌었으므로 코드 검증 결과는 달라지지 않는다. 고친 문서가 없으면 재봉인하지 않는다.

## 5 판정 기록

```bash
issueops project-docs-review record --id "$ISSUEOPS_ID" \
  --verdict updated --doc ".issueops/CAUTIONS.md" --doc ".issueops/cautions/<module>.md" \
  --evidence "<무엇을 대조했고 왜 이 문서를 고쳤는가>" $RECORD_ACTOR_FLAGS --json

# 고칠 것이 없을 때
issueops project-docs-review record --id "$ISSUEOPS_ID" \
  --verdict no-change --evidence "<대조한 문서 목록과 판단>" $RECORD_ACTOR_FLAGS --json
```

- `updated`는 `--doc` 경로가 **실제 변경 집합 안에** 있어야 통과한다. 고쳤다는
  자기신고만으로는 통과하지 않는다.
- `no-change`는 `--doc`을 받지 않는다. 대신 무엇을 대조했는지가 evidence다.
  "대조했으나 없음"과 "대조하지 않음"은 다르므로, 읽은 문서를 나열한다.
- 이 기록도 변경 집합 fingerprint를 봉인한다. 이후 diff가 바뀌면
  `project_docs_review_stale`이 되고 `next`가 이 단계로 되돌린다.

## 출구

다음은 [`issueops-verify`](../issueops-verify/SKILL.md)다. 이 단계에서는 커밋하지 않는다.

## 나쁜 예

| 나쁜 행동 | 문제 |
|---|---|
| 문서를 고치지 않고 `--verdict updated` | `--doc`이 변경 집합에 없어 거부된다 |
| 변경 집합 밖 문서를 `--doc`에 적는다 | 같은 이유로 거부된다. 이번에 고친 것만 적는다 |
| 문서를 고치고 재봉인을 생략한다 | `ai_slop_clean_stale`로 이후 단계가 전부 막힌다 |
| 구현이 규칙을 어겼는데 규칙을 고쳐 덮는다 | 문서가 코드를 따라가면 문서는 아무것도 제약하지 못한다 |
| ADR의 기존 항목을 고친다 | ADR은 append-only다. 뒤집는 결정은 새 항목으로 적는다 |
| `no-change`를 근거 없이 기록한다 | 읽지 않은 것과 읽었는데 없는 것을 구분할 수 없다 |
| 이 단계에서 코드를 고친다 | 그 변경은 정리와 검증을 다시 통과해야 한다. 4단계로 돌아간다 |

## 검증

- `issueops status --id "$ISSUEOPS_ID" --json`의 `project_docs_review`에
  이번 판정이 있고 `reviewed_fingerprint`가 현재 변경 집합과 같다.
- `issueops next --id "$ISSUEOPS_ID" --json`의 `stage.key`가 `verify`이거나
  그 뒤 단계다. `docs`나 `clean`으로 남아 있으면 `missing`이 이유를 말한다.
- `verdict updated`로 기록했으면 `git -C "$WORKTREE" status --porcelain`에 그 문서들이
  보인다.
