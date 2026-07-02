# Engelbart Canvas UI Examples

Use these snippets only when drafting or debugging a Slack Canvas body.

## Status Callout

```markdown
::: {.callout}
회의일 YYYY-MM-DD · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요
:::
```

## Metadata Table

```markdown
| Field | Value |
|---|---|
| Date | YYYY-MM-DD |
| Topic | 온보딩 |
| Status | Follow-up 필요 |
| Last updated | YYYY-MM-DD |
```

## TL;DR Bullets

```markdown
## TL;DR
- 팀 R&R과 우선순위가 정리됐다.
- 신규 팀원 온보딩의 첫 태스크와 지원자가 정해졌다.
- 배포/마이그레이션 follow-up이 남아 있다.
```

## Action Checklist

```markdown
## 액션 보드
- [ ] 김현호 팀리더: GitLab issue에 추천 시스템 데이터 파이프라인 태스크를 작성한다. 기한: 미정. Tracking: GitLab.
```

## Short Evidence Quote

```markdown
> 결정 근거가 되는 짧은 원문만 인용한다.
```

## Audit Divider

```markdown
---
```

## Transcript Block

````markdown
```text
참석자 1 00:00
원문 발화를 그대로 보존한다.
```
````

## Layout Columns

```markdown
::: {.layout}
::: {.column}
### 읽는 순서
- TL;DR
- 결정사항
- 액션
:::
::: {.column}
### 감사 순서
- 리스크
- 보정
- 원문
:::
:::
```

Use layout columns only for short dashboard-style summaries. Do not use them in the default meeting minutes body because tables, callouts, and transcript/code blocks are not supported inside layouts.
