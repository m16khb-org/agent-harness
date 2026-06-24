---
name: clova-meeting-minutes
description: "Use when converting NAVER Clova Note transcripts, speaker-labeled voice records, pasted meeting text, or AI meeting summaries into Korean meeting minutes, Slack Canvas meeting docs, Canvas index updates, Slack recaps, action-item tables, or reusable meeting-minutes prompts."
---

# Clova Meeting Minutes

## Overview

Turn Clova Note voice transcripts into action-oriented Korean meeting minutes. Treat the transcript as raw evidence, then normalize names, technical terms, decisions, action items, and unresolved questions into a reusable minutes format.

## Core Rule

Do not produce a chronological transcript summary unless the user explicitly asks for one. Meeting minutes should preserve what matters after the meeting: decisions, owners, deadlines, risks, and follow-up locations.

## Workflow

1. Identify the requested output: full minutes, Slack Canvas meeting doc, Canvas index update, Slack recap, action table, template, or prompt.
2. Read the transcript and build a correction map:
   - speaker aliases: `참석자 1`, nicknames, titles, unclear speaker labels
   - technical terms: misheard product names, Git/GitLab/MR/PR/branch names, model names
   - uncertain phrases: mark as `확인 필요` instead of inventing certainty
3. Extract outcomes before narrative:
   - decisions
   - action items
   - risks/open questions
   - follow-up meetings or checkpoints
4. Group discussion by topic, not by timestamp, unless exact timeline matters.
5. Return the requested artifact in Korean by default, with concise wording and no filler.

## Slack Canvas Operating Model

When the user wants Slack Canvas output, assume this durable team workflow:

- Team channel such as `#dev-team-backend`: keep one shared meeting index Canvas as a channel tab.
- Each meeting: create one individual meeting Canvas and link it from the index. If standalone Canvas is unavailable on the workspace plan, use an available channel/DM Canvas and still keep the same body template.
- Huddle meetings: when notes are created during a huddle, use the huddle Canvas, then link it from the huddle thread and index Canvas.
- Channel message: post only a short TL;DR and the Canvas link.
- Task tracking: use Slack List, GitLab issue/MR, or another explicit tracker for action state. Do not rely on prose-only action items.

Do not claim to have created, shared, pinned, or updated a Slack Canvas unless a Slack tool call or user confirmation proves it. If only drafting text, call it Canvas-ready content.

## Canvas Naming Rules

Always title individual meeting canvases with this exact pattern:

```text
YYYY-MM-DD [Topic] Title
```

Examples:

```text
2026-06-24 [배포] release/stg 배포 회고
2026-06-25 [테스트품질] 2468 parent 정리 회의
```

Use short topic labels such as `배포`, `테스트품질`, `장애`, `온보딩`, `AI`, `인프라`, `정책`, `리뷰`. If unclear, use `[확인필요]`.

## Output Template

Use this structure for full minutes:

```markdown
# 회의록: {회의명}

## 1. 회의 정보
- 일시: {YYYY-MM-DD HH:mm 또는 미상}
- 작성 기준: NAVER Clova Note 전사본 기반
- 참석자: {확인된 이름}
- 참조 대상: {필요 시}
- 회의 목적: {이번 회의에서 정하려던 것}
- 관련 링크: {Clova Note, 이슈, 문서, PR/MR 등}

## 2. 한 줄 요약
{회의 결론을 1~2문장으로 요약}

## 3. 핵심 요약
- {핵심 내용}

## 4. 결정사항
| 결정 | 배경/이유 | 영향 범위 | 결정자/동의자 |
|---|---|---|---|
| {정한 내용} | {왜 정했는가} | {팀/서비스/일정 영향} | {이름 또는 미상} |

## 5. 액션 아이템
| 상태 | 할 일 | 담당 | 기한 | 후속 확인 위치 |
|---|---|---|---|---|
| TODO | {산출물 중심 할 일} | {한 명 또는 미정} | {날짜 또는 미정} | {이슈/Slack/문서} |

## 6. 주제별 논의 내용

### 6.1 {주제명}
- 배경:
  - {왜 논의했는가}
- 주요 논점:
  - {논의 내용}
- 정리:
  - {현재 결론 또는 상태}

## 7. 리스크 / 열린 질문
| 항목 | 내용 | 확인 담당 | 확인 방법 |
|---|---|---|---|
| {리스크/질문} | {불명확한 점} | {이름 또는 미정} | {확인 경로} |

## 8. 후속 회의 / 다음 확인 지점
- 다음 회의: {일시 또는 미정}
- 다음 확인 항목:
  - {확인할 것}

## 9. Clova Note 전사본 보정 메모
최종 공유본에서 필요 없으면 삭제한다.

### 이름/화자 매핑
| 전사본 표기 | 실제 이름 |
|---|---|
| 참석자 1 | {이름 또는 확인 필요} |

### 용어 보정
| 전사본 표현 | 실제 표현 |
|---|---|
| {오인식 표현} | {정확한 표현} |

### 불확실한 부분
- {확인이 필요한 문장, 이름, 기한, 결정사항}
```

## Action Item Rules

- Use one accountable owner when possible. If the transcript implies a team owner but not a person, write `미정` and add an open question.
- Prefer deliverables over vague verbs: `검토한다` is weak; `GitLab 이슈 본문에 검증 체크리스트를 추가한다` is strong.
- Include a due date only when the transcript states or clearly implies one. Otherwise use `미정`.
- Keep follow-up location explicit: Slack thread, GitLab issue, MR, doc, calendar, or `미정`.

## Clova Note Cleanup Rules

- Correct obvious speech-to-text errors, but do not silently change ambiguous business meaning.
- Preserve uncertainty with `확인 필요`; do not invent names, dates, issue numbers, decisions, or consent.
- Drop small talk unless it affects logistics, onboarding, relationships, or a decision.
- If the user provides corrections like `참석자1은 김현호`, apply them consistently and mention the correction only if useful.
- If the transcript contains sensitive details, summarize the decision or action without exposing unnecessary private content.

## Common Output Variants

### Canvas-Ready Meeting Doc

Use this exact structure for every individual meeting Canvas. Keep section names and field order stable so the team can scan every meeting the same way:

```markdown
# YYYY-MM-DD [Topic] Title

| Field | Value |
|---|---|
| Date | YYYY-MM-DD |
| Topic | {배포/테스트품질/장애/온보딩/AI/인프라/정책/리뷰/확인필요} |
| Tags | {comma-separated tags} |
| Status | {Draft / Follow-up 필요 / 완료 / 확인 필요} |
| Source | {CLOVA Note 링크 또는 미정} |
| Slack Thread | {링크 또는 미정} |
| Tracking | {GitLab Issue/MR, Slack List, 문서 링크 또는 미정} |

## TL;DR
{결론 1~2문장}

## 결정사항
| 결정 | 이유 | 영향 | 상태 |
|---|---|---|---|
| {결정} | {배경} | {영향 범위} | 확정 |

## 액션 보드
| 상태 | 담당 | 할 일 | 기한 | 링크 |
|---|---|---|---|---|
| TODO | {이름/미정} | {산출물 중심 작업} | {날짜/미정} | {이슈/문서/Slack/미정} |

## 확인 필요
| 질문 | 확인 담당 | 확인 방법 |
|---|---|---|
| {열린 질문} | {이름/미정} | {확인 경로} |

## 논의 내용
### {주제}
- 배경: {왜 논의했는가}
- 논점: {주요 내용}
- 정리: {현재 상태}

## 전사본 보정 메모
### 화자/이름
| 전사본 | 실제 |
|---|---|
| {참석자 1} | {이름/확인 필요} |

### 용어
| 전사본 | 실제 |
|---|---|
| {오인식} | {정확한 표현} |

## 변경 로그
| 일시 | 변경 내용 |
|---|---|
| YYYY-MM-DD | 최초 작성 |
```

Canvas rules:

- Put `TL;DR`, `결정사항`, and `액션 보드` above detailed discussion.
- Keep each table row self-contained so it can become a task or comment thread.
- Move transcript cleanup and uncertainty to the bottom; do not let it interrupt the main flow.
- Use short headings that become useful document anchors.
- Do not include long verbatim transcript excerpts unless the user asks for audit evidence.
- Do not paste the full transcript into the Canvas. Put the Clova Note link in `Source`.
- Split uncertain transcript-backed claims into `확인 필요`, not into `결정사항`.
- If Canvas update noise matters, mention that workspace/admin Canvas update message settings may need review instead of trying to solve it in the minutes.

### Canvas Index

Use this for the persistent channel-tab index Canvas:

```markdown
# Backend 회의록 인덱스

## 날짜별
| Date | Topic | Title | Status | Link |
|---|---|---|---|---|
| YYYY-MM-DD | {Topic} | {Title} | {Status} | {Canvas 링크} |

## 주제별
### 배포
- YYYY-MM-DD - {Title}: {Canvas 링크}

### 테스트품질
- YYYY-MM-DD - {Title}: {Canvas 링크}

### 장애 / Incident
- YYYY-MM-DD - {Title}: {Canvas 링크}

### 온보딩
- YYYY-MM-DD - {Title}: {Canvas 링크}

### AI / 인프라 / 정책 / 리뷰
- YYYY-MM-DD - {Title}: {Canvas 링크}
```

When drafting an index update, output only the new row and the topic section entry unless the user asks for the full index Canvas.

### Slack Recap

Use this when the user asks for a channel message to accompany the Canvas:

```markdown
오늘 회의록 Canvas입니다: {Canvas 링크}

- 핵심 결론: {1문장}
- 결정사항:
  - {결정}
- 액션 아이템:
  - {담당}: {할 일} ({기한})
- 확인 필요:
  - {열린 질문}
```

Keep channel recaps short. Put durable details in the Canvas, not in the channel message.

### Prompt for Reuse

Use this when the user asks for an AI prompt:

```text
다음 NAVER Clova Note 전사본을 기반으로 Slack Canvas에 붙여넣을 한국어 회의록을 작성해줘.

규칙:
- 전사본은 원천 기록일 뿐 최종 회의록이 아니다.
- 먼저 화자/이름/기술 용어 오인식을 보정한다.
- 결정사항, 액션 아이템, 열린 질문을 먼저 추출한다.
- 액션 아이템은 담당자, 기한, 후속 확인 위치를 표로 정리한다.
- 불확실한 이름/기한/결정은 추측하지 말고 `확인 필요`로 표시한다.
- 잡담은 제외하되, 일정/온보딩/업무 배정에 영향을 주면 남긴다.
- Canvas 제목은 반드시 `YYYY-MM-DD [Topic] Title` 형식으로 만든다.
- 본문 상단에는 항상 `Date`, `Topic`, `Tags`, `Status`, `Source`, `Slack Thread`, `Tracking` 메타 테이블을 둔다.
- 원본 transcript 전체를 넣지 말고 `Source`에 CLOVA Note 링크만 둔다.
- 출력은 다음 고정 섹션을 사용한다:
  1. TL;DR
  2. 결정사항
  3. 액션 보드
  4. 확인 필요
  5. 논의 내용
  6. 전사본 보정 메모
  7. 변경 로그
- 인덱스 Canvas에 추가할 날짜별 row와 주제별 entry도 함께 제안한다.

전사본:
{여기에 붙여넣기}
```

## Final Check

Before responding, verify:

- The output is not just a cleaned transcript.
- Every decision is separated from discussion.
- Every action item has owner, due date, and follow-up location fields, even if some are `미정`.
- Canvas-ready output keeps summary, decisions, and action board at the top.
- Individual meeting Canvas output follows `YYYY-MM-DD [Topic] Title` and the fixed metadata table.
- Canvas index updates include both the date row and topic-section entry when requested or useful.
- Uncertain transcript content is marked instead of guessed.
- The response matches the user's requested format: full minutes, recap, action table, template, or prompt.
