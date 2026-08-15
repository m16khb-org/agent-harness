# Engelbart Canvas UI Examples

Use these snippets only when drafting or debugging a Slack Canvas body.

## Status Block

Raw Slack Web API `canvases.create`/`canvases.edit` does not support `::: {.callout}`.
Use the quote form when publishing with `.zshrc` token or `publish_meeting_canvas.py`.

```markdown
> 회의일 YYYY-MM-DD · 대상 #sample-platform-team · Source synthetic transcript · Status Follow-up 필요
```

Use the callout form only for Slack MCP Canvas-flavored Markdown, and only after
read-back confirms it renders as callout UI instead of literal text.

```markdown
::: {.callout}
회의일 YYYY-MM-DD · 대상 #sample-platform-team · Source synthetic transcript · Status Follow-up 필요
:::
```

## Metadata Table

Keep metadata table values short. If a value needs a full participant list,
source URL, tracking URL, or access explanation, put only a summary in the table
and move detail to `### 메타데이터 메모`.

```markdown
|Field|Value|
|  ---  |  ---  |
| Date | YYYY-MM-DD |
| Topic | 온보딩 |
| Status | Follow-up 필요 |
| Participants | 회의진행자 외 4명 |
| Source | 붙여넣은 화자분리 전사본 |
| Tracking | Slack List / GitLab |
| Last updated | YYYY-MM-DD |
```

```markdown
### 메타데이터 메모
- Participants: 회의진행자, 백엔드리더, 데이터담당, 플랫폼담당, 신규팀원
- Source: 붙여넣은 회의 전사본. 긴 원문은 부록의 `원문 전사본 전문`에 보존.
- Tracking: Slack List row와 GitLab/런북 링크는 Canvas 검증 후 별도 바인딩.
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
- [ ] 데이터담당: 샘플 이슈에 이벤트 파이프라인 태스크를 작성한다. 기한: 미정. Tracking: 샘플 이슈.
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
