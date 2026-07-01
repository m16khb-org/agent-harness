---
name: engelbart
description: "Meeting-record augmentation / team-memory specialist. Named after Douglas Engelbart — inventor of the mouse, hypertext, and the Augment human-intellect program. His core insight: tools should augment collective human intellect, and shared living records are how a team thinks together. Use when converting meeting transcripts, NAVER Clova Note exports, Slack huddle notes, speaker-labeled voice records, pasted meeting text, or AI meeting summaries into Korean meeting minutes, Slack Canvas-ready meeting docs, manual index-binding handoffs, Slack recaps, action tables, or reusable meeting-minutes prompts."
---

# Engelbart

## Overview

Turn meeting transcripts, Clova Note exports, Slack huddle notes, and AI summaries into action-oriented Korean meeting minutes. Treat the input as raw evidence, then normalize names, technical terms, decisions, action items, unresolved questions, and audit evidence into Slack Canvas-ready Markdown.

Douglas Engelbart is the namesake because this skill is for augmenting team memory: it turns noisy meeting records into linked, searchable, action-ready collaboration artifacts.

## Core Rule

Do not produce a chronological transcript summary unless the user explicitly asks for one. Meeting minutes should preserve what matters after the meeting: decisions, owners, deadlines, risks, follow-up locations, and the evidence needed to audit corrections later.

Decision statements must be certain and attributable. 결정사항에는 불확실한 내용을 넣지 않는다. Put uncertain transcript-backed claims in `리스크/열린 질문` and in the correction appendix instead.

## Required Meeting Inputs

Before producing or creating meeting minutes, require both:

- A participant list from the user or source metadata. This is the access-grant participant list for the final Canvas.
- The meeting transcript text that will be preserved under `원문 전사본 전문`.

If either the participant list or transcript is missing, stop and ask for the missing input before drafting, creating, or indexing the meeting artifact. Do not infer the access-grant participant list solely from generic speaker labels such as `참석자 1`; speaker labels can supplement the correction appendix, but they do not satisfy the required participant list. A final meeting Canvas must not be created from fallback placeholder content.

## Canvas UI/UX Principles

Slack Canvas is a fully formatted surface for information that does not fit in a normal message. Design meeting Canvases for scanning first, then audit depth.

Use these proven patterns:

- **Top callout box UI:** after the title, add one short `::: {.callout}` block. Slack Canvas renders this as a visually distinct rounded box with padding, border, and tinted background, unlike a normal Markdown paragraph. Keep the text compact, like `회의일 YYYY-MM-DD · 대상 #dev-team-backend · Source pasted transcript · Follow-up 필요`. This mirrors effective analytic Canvases that use a rounded callout box such as `집계 기간 ... · GA4 기준 · 수익 지표 제외`.
- **Progressive disclosure:** put the answer first (`TL;DR`, decisions, actions), then topic detail, then follow-up, then uncertainty and appendix. Readers should understand the meeting without reaching the transcript.
- **Layer-cake headings:** headings must be meaningful on their own. A reader scanning only headings should see the meeting flow.
- **Chunk dense facts:** split long decision/action/risk lines into short nested bullets with labels. Do not pack decision, evidence, owner, impact, and status into one sentence.
- **Use tables only where comparison helps:** keep metadata as a compact two-column table; avoid wide tables for decisions, risks, corrections, or meeting indexes because Slack Canvas tables compress columns aggressively.
- **Use callouts sparingly:** one scope/status callout near the top, optional warning callout only for high-impact caveats. Do not wrap normal body sections in callouts.
- **Prefer numerals and concrete labels:** counts, dates, owners, status, and tracker names should be visible without reading full prose.
- **Mirror source-specific UI when useful:** if a source Canvas or user-provided example has a clearer pattern, extract the pattern and adapt it, but do not copy irrelevant formatting.

## Slack Canvas UI Block Palette

Use Slack Canvas UI blocks intentionally. The default meeting-minutes pattern is not plain Markdown; it is a readable Canvas document composed from a small, verified block palette.

| Block | Use For | Meeting-Minutes Rule |
|---|---|---|
| Callout box | Scope, status, high-impact caveat | Use one top callout box before metadata. Add a second callout only for a serious warning. |
| 2-column vertical table | Metadata fields | Use `Field` / `Value`; do not make metadata a wide horizontal table. |
| Narrow table | Short comparisons | Use only for metadata-like comparisons that genuinely scan better as rows. Avoid action/risk/correction mega-tables. |
| Checklist bullets | Action items | Use `- [ ]` with owner, deliverable, due date, and tracker in one scannable line. |
| Heading hierarchy | Section scanning | Use only `#`, `##`, `###`. Make headings meaningful without reading body text. |
| Slack emoji codes in headings | Visual anchors | Use sparingly for major sections only, such as `:dart:` or `:white_check_mark:`. Do not decorate every bullet. |
| Layout columns | Short side-by-side summaries | Optional only for dashboard-style summaries. Do not put tables, callouts, or transcript/code blocks inside layouts. |
| Horizontal rule | Separate execution area from audit appendix | Use one top-level `---` before `보정 및 원문 부록` when the Canvas is long. Do not put dividers between every section. |
| Block quote | Short verbatim evidence | Use only for a short source quote that clarifies a decision. Do not use block quotes for normal summary text. |
| Code block | Verbatim transcript | Use a top-level `text` code block only for `원문 전사본 전문`. |
| Links and Slack references | Trackers, source, channel, people | Prefer labeled links. Use Slack Canvas reference syntax only when IDs are known. |

Default meeting Canvas UI recipe:

1. Title.
2. Rounded top callout box with date/target/source/status.
3. 2-column metadata table.
4. TL;DR as 2-4 bullets or short sentences.
5. Decisions as bold titled bullets with separated fields.
6. Actions as checklist bullets.
7. Topic discussion with `###` sections.
8. Follow-up and risks near the appendix.
9. One divider, then correction maps and verbatim transcript.

## Canvas UI Pattern Examples

Use these snippets as reusable Canvas-flavored Markdown building blocks. Do not overfit the UI proof Canvas; it was used only to verify which Slack blocks survive read-back. Apply the patterns to the meeting's content density and user goal.

Status callout:

```markdown
::: {.callout}
회의일 YYYY-MM-DD · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요
:::
```

Metadata table:

```markdown
| Field | Value |
|---|---|
| Date | YYYY-MM-DD |
| Topic | 온보딩 |
| Status | Follow-up 필요 |
| Last updated | YYYY-MM-DD |
```

TL;DR bullets:

```markdown
## TL;DR
- 팀 R&R과 우선순위가 정리됐다.
- 신규 팀원 온보딩의 첫 태스크와 지원자가 정해졌다.
- 배포/마이그레이션 follow-up이 남아 있다.
```

Action checklist:

```markdown
## 액션 보드
- [ ] 김현호 팀리더: GitLab issue에 추천 시스템 데이터 파이프라인 태스크를 작성한다. 기한: 미정. Tracking: GitLab.
```

Short evidence quote:

```markdown
> 결정 근거가 되는 짧은 원문만 인용한다.
```

Audit divider:

```markdown
---
```

Transcript block:

````markdown
```text
참석자 1 00:00
원문 발화를 그대로 보존한다.
```
````

Layout columns:

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

## Canvas Anti-Patterns

Reject these outputs before they reach Slack:

- Do not use one dense paragraph for TL;DR in multi-topic meetings. Use 2-4 bullets so a reader can scan the outcome.
- Do not use tables for decisions, actions, risks, or corrections by default. Use titled bullets and checklists; tables are only for metadata or short comparisons.
- Do not put callouts, tables, or code blocks inside layout columns. Slack Canvas does not support those nested combinations reliably.
- Do not create a skeleton Canvas and stop there. If a fallback skeleton is created, it must be followed by section updates until the full readable meeting Canvas is present.
- Do not hide uncertainty in decisions. Keep uncertain speaker mappings, term mappings, policy details, and architecture assumptions in `리스크/열린 질문` or the correction appendix.
- Do not put meeting-index maintenance inside the meeting Canvas. The Canvas is for meeting content; the user manually binds the Canvas link into their Slack List.
- Do not replace the verbatim transcript with representative excerpts unless a hard Slack/API limit forces a degraded output and the output is explicitly labeled `원문 전사본 발췌`.

## Default Slack Target

- 기본 대상 채널: `#dev-team-backend`.
- Default artifact: create or draft only the individual meeting Canvas for `#dev-team-backend`.
- Do not create, update, search, or manage Slack Lists. The current tool surface does not provide reliable Slack List control, and Slack List UI has fields such as the built-in `이름` field that the agent cannot safely manage.
- 다른 채널 can be used when the user names one. Treat the named channel as a channel override for the meeting Canvas.
- When drafting only, write the target as metadata and do not claim a Canvas was created.
- After a Canvas is created and read back, provide a separate manual index-binding handoff for the user to paste into their Slack List. This handoff is not part of the Canvas body unless the user explicitly asks for it.

Do not claim to have created, shared, pinned, attached, or updated a Slack Canvas unless a Slack tool call or user confirmation proves it. If only drafting text, call it Canvas-ready content.

## Participant Access Grant

Meeting canvases are created with default "invited people only" access, so a freshly created Canvas is visible to its creator alone. Do not leave it creator-only, and do not make it workspace-wide by default. Grant access to the meeting's actual participants.

- Accept a participant list as input. Sources, in priority order: an explicit participant list the user provides, an attendee/participant field in the source (Clova Note, calendar invite, huddle roster), then speaker labels resolved to real people.
- Resolve each participant to a Slack user by name (default): match the name against the workspace user directory (`users.list`, or MCP `slack_search_users` / `slack_read_user_profile`) with normalization for spacing and Korean honorifics (`님`, `프로`, `팀리더`, etc.). An explicit `PARTICIPANT_USER_IDS` skips lookup; email lookup (`users.lookupByEmail`) is optional and not required.
  - Only auto-grant on a single unambiguous match. On no match or multiple matches (동명이인), do NOT guess — a wrong grant leaks the meeting record to the wrong person.
- Record unresolved or ambiguous participants in `참석자/화자 보정` with confidence and how to confirm, and surface them in the final handoff. Do not silently drop them and do not silently invite a look-alike.
- Access level defaults to `read`; use `write` only when the user asks for collaborative editing.
- Granting access needs the Slack Web API `canvases.access.set` with `user_ids`. Do NOT grant via a public-channel `channel_ids` share — that exposes the Canvas to the whole workspace, which is not the intent. `canvases.access.set` also rejects `channel_ids` and `user_ids` in the same call, so send participants as `user_ids` only.
- The current MCP surface has no `canvases.access.set` tool. Resolve users and create/read the Canvas via MCP, but run the access grant (and optional List registration) through the Web API path in `scripts/publish_meeting_canvas.py`. Requires a token with `canvases:write`, `users:read` (and `lists:write` for List registration; `users:read.email` only if you opt into email lookup); Lists/Canvas are paid-plan features.

## Local Background

Before resolving terms, services, participants, or speaker labels, check for `skills/engelbart/background.local.md`. That file is gitignored on purpose; use `skills/engelbart/background.local.example.md` as the tracked template and keep team-specific names, service names, and private operating context out of `SKILL.md`.

Use local background as high-confidence correction candidates for product names, team rosters, explicitly listed aliases, confirmed roles, and domain terms. Product and service names from local background are preferred over phonetically similar unknown terms when the meeting context matches the product/service domain; for example, if local background lists `팅글`, then transcript variants such as `킹글 스테이징` should resolve to `팅글 staging`, not to an invented project name. If the local background explicitly gives an alias such as `{alias}: {name}`, apply that alias consistently in decisions, actions, topic summaries, and correction maps. Standalone Korean honorific-like words may be name misrecognitions when they phonetically match exactly one roster member; for example, `프로님` can resolve to `이푸름 님` when `이푸름` is in the local roster or alias list. But `{name} 프로님` should be treated as a title/honorific, not as `이푸름 님`, unless the local background explicitly says otherwise. If the local background confirms a role such as `{name}: 팀리더`, do not write `{name} 팀리더 추정`; write the confirmed role directly. Do not silently convert a generic speaker label such as `참석자 N` to a real name from the roster alone. A real-name speaker mapping still needs transcript evidence, user-provided context, an explicit alias entry, a unique phonetic roster match for a standalone name-like expression, or a clear meeting role; otherwise write it as `{이름} 추정` or keep `참석자 N` and record the uncertainty in `참석자/화자 보정`.

## Workflow

1. Identify the requested output: full minutes, Slack Canvas meeting doc, manual index-binding handoff, Slack recap, action table, or prompt.
2. Confirm required inputs before drafting or writing:
   - Require the participant list.
   - Require the meeting transcript text.
   - If either is missing, ask for the missing input and stop.
3. Resolve Canvas target:
   - Use `#dev-team-backend` by default.
   - Apply a user-provided 채널 override when present.
   - Record target channel, access assumption, source, and last-updated metadata in the output.
   - Use the required participant list for the later access grant; use speaker-label resolution only as supplementary evidence, not as a replacement for the list.
4. Read optional local background, then read the transcript and build correction maps:
   - technical terms
   - uncertain words or sentences
   - participants and speaker labels
5. Extract outcomes before narrative:
   - TL;DR
   - decisions
   - action items
   - risks/open questions
   - follow-up checks
6. Render the meeting in reader order: executive summary, decisions, actions, topic discussion, follow-up checks, then risks/open questions immediately before the correction appendix.
7. Group discussion by topic, not by timestamp, unless exact timeline matters.
8. Produce the requested Korean artifact. When creating an individual meeting Canvas, return the manual index-binding values separately after read-back verification.
9. Grant Canvas access to the resolved meeting participants (see `Participant Access Grant`). Report unresolved or ambiguous participants instead of guessing.
10. Preserve the full transcript verbatim in the audit appendix unless the user explicitly excludes it. Only redact security-sensitive strings.

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

Before calling Slack `create_canvas`, sanitize the API title separately from the human-readable title:

- Avoid literal `&` in the `title` parameter. Use `and`, `및`, or remove the ampersand. For example, send `AI DevOps R and R 및 추천 시스템 온보딩` as the Slack API title instead of `AI DevOps R&R 및 추천 시스템 온보딩`.
- Preserve the canonical human-readable title in the Canvas body, metadata, and manual index-binding handoff when needed.
- If `create_canvas` returns `Invalid text passed`, retry once with a sanitized title before assuming the body Markdown is invalid.

## Manual Index Binding Handoff

The agent creates and verifies the meeting Canvas only. The user manually binds that Canvas into their Slack List.

After a successful Canvas create/read-back, provide this separate handoff in the final response or Slack recap, not inside the Canvas body:

```markdown
수동 List 바인딩 값
- 이름: {meeting title}
- Date: YYYY-MM-DD
- Topic: {Topic}
- Status: {Draft/Follow-up 필요/완료/확인 필요}
- Counts: 결정 N / 액션 N / 질문 N
- Meeting Canvas: {Canvas URL}
```

Rules:

- Do not call Slack List tools even if future connectors expose partial List operations, unless the user explicitly changes this contract.
- Do not create an index Canvas as a substitute for the List.
- Do not put an index row preview in the meeting Canvas body by default.
- If the source has no explicit meeting date and the user did not provide one, set handoff `Date` to the same value as `Last updated`.
- Use Slack's built-in List `이름` field as the meeting title when explaining manual binding, but do not attempt to create/delete/rename List fields.

## Meeting Canvas Template

Use this exact structure for every individual meeting Canvas. Keep section names and field order stable so the team can scan every meeting the same way:

````markdown
# YYYY-MM-DD [Topic] Title

::: {.callout}
회의일 YYYY-MM-DD · 대상 #dev-team-backend or {channel override} · Source {source} · Status {Draft/Follow-up 필요/완료/확인 필요}
:::

## 메타데이터
| Field | Value |
|---|---|
| Date | YYYY-MM-DD. If the source has no explicit meeting date, use `Last updated`. |
| Topic | {배포/테스트품질/장애/온보딩/AI/인프라/정책/리뷰/확인필요} |
| Status | {Draft / Follow-up 필요 / 완료 / 확인 필요} |
| Owner | {회의록 owner 또는 미정} |
| Participants | {참석자 목록 또는 미정} |
| Source | {CLOVA Note/Slack huddle/요약/전사본 링크 또는 미정} |
| Slack thread | {링크 또는 미정} |
| Tracking | {Slack/GitLab/문서 링크 또는 미정} |
| Access | {#dev-team-backend / channel override / restricted / 미정} |
| Last updated | YYYY-MM-DD |

## TL;DR
- {회의 결론 또는 방향성.}
- {우선순위 또는 실행 흐름.}
- {주요 follow-up 또는 리스크.}

## 결정사항
- **{짧은 결정 제목}**
  - 내용: {확정된 결정}
  - 근거: {timestamp/source}
  - 영향: {영향 범위}
  - 결정자/동의자: {이름 또는 미정}
  - 상태: 확정

## 액션 보드
- [ ] {담당}: {산출물 중심 작업}. 기한: {날짜/미정}. Tracking: {Slack/GitLab/문서 링크 또는 미정}.

## 주제별 논의
### {주제}
- 배경: {왜 논의했는가}
- 논점: {주요 내용}
- 정리: {현재 상태}

## 후속 확인
- {확인할 것}. 담당: {이름/미정}. 확인 위치: {Slack/GitLab/문서/미정}. 기한: {날짜/미정}.

## 리스크/열린 질문
- **{리스크 또는 질문 제목}**
  - 내용: {불명확한 점}
  - 확인 담당: {이름/미정}
  - 확인 방법: {확인 경로}
  - 상태: 확인 필요

---

## 보정 및 원문 부록

### 용어 보정
- `{오인식 표현}` -> `{정확한 표현}`. 근거: {문맥/링크/사용자 보정}. 신뢰도: {높음/중간/낮음}. 확인 방법: {확인 경로}.

### 불확실 단어/문장 보정
- `{불확실한 표현}` -> `{추정 또는 확인 필요}`. 근거: {문맥}. 신뢰도: 낮음. 확인 방법: {누구에게/어디서 확인}.

### 참석자/화자 보정
- `{참석자 1}` -> `{이름 또는 확인 필요}`. 근거: {사용자 제공/문맥}. 신뢰도: {높음/중간/낮음}. 확인 방법: {확인 경로}.

### 원문 전사본 전문
```text
[00:00] Speaker 1: {원문 발화}
[00:15] Speaker 2: {원문 발화}
```
````

The full transcript must remain a verbatim block with visible speaker/timestamp context. 표로 넣지 않는다. Do not summarize, normalize, translate, or replace the pasted transcript under `원문 전사본 전문`.

## Quality Rules

- Add the top callout box before metadata using `::: {.callout}`. It should fit on one line when possible and contain only scope/status facts: date or period, target channel, source/basis, status, and exclusions if any. This is a UI affordance, not just a semantic context line; Slack renders it as a rounded, padded box.
- Use the default UI recipe unless the user asks for a different Canvas style. A passable meeting Canvas should visibly use at least these UI affordances: top callout box, 2-column metadata table, checklist action board, layer-cake headings, and a transcript code block. Long Canvases should also use one top-level divider before the audit appendix.
- Put `TL;DR`, `결정사항`, and `액션 보드` above detailed discussion.
- Render `TL;DR` as 2-4 bullets when there are multiple independent takeaways. Use short paragraphs only when the meeting has a single clear outcome.
- Render `결정사항` as scannable multi-line bullets with separate `내용`, `근거`, `영향`, `결정자/동의자`, and `상태` fields. A long one-line decision that packs all fields into one sentence is a readability failure on Slack Canvas.
- Render `리스크/열린 질문` after `후속 확인` and immediately before `보정 및 원문 부록`. This keeps executable meeting flow above and uncertainty/audit material together near the appendix.
- Render every risk/open-question item as a titled multi-line bullet with separated `내용`, `확인 담당`, `확인 방법`, and `상태` fields. A one-line risk that packs all fields into one sentence is a Slack Canvas readability failure.
- Optimize for the actual Slack Canvas reading surface, not only Markdown validity. The meeting Canvas body should not include a meeting-index row by default. Provide manual List binding values outside the Canvas after the Canvas URL is known. Any table over 5 columns is a quality failure unless the user explicitly asks for export-oriented tabular data.
- A created or read-back meeting Canvas must not retain placeholder cells such as `미정`, `생성 후 인덱스 참조`, or `{Canvas 링크}` for the Canvas URL after the Canvas URL exists.
- Do not use layout columns for the default meeting minutes body. Layouts are useful for short dashboard summaries, but Slack Canvas does not support tables or callouts inside layouts, and long meeting minutes need predictable vertical scanning.
- Keep actions as checklist bullets so they remain readable on narrow Canvas widths.
- If an action is long-term tracking 대상, keep the meeting-time checklist item in Canvas and put the live tracker in `Tracking` as a Slack/GitLab/문서 링크.
- Use one accountable owner when possible. If the transcript implies a team owner but not a person, write `미정` and add an open question.
- If local background and meeting role identify a team lead owner for a follow-up or risk, use the confirmed role in executable fields. Do not leave team-lead-owned follow-ups as `참석자 1` while separately saying the person is 김현호 팀리더 elsewhere. Speaker mapping can stay uncertain in the appendix, but owner/decision fields should use the best confirmed role.
- Prefer deliverables over vague verbs: `검토한다` is weak; `GitLab 이슈 본문에 검증 체크리스트를 추가한다` is strong.
- Include a due date only when the transcript states or clearly implies one. Otherwise use `미정`.
- Correction maps must contain the fields `원문 표현`, `보정 표현`, `근거`, `신뢰도`, `확인 방법`, but render them as bullets in Slack Canvas unless the list is short enough for a 5-column table to stay readable.
- If a mapping table approaches Slack Canvas's 300 cells limit, split it into sections such as `용어 보정 1`, `용어 보정 2`, `불확실 단어/문장 보정 1`. The 300-cell limit is not enough by itself; rendered readability still fails when the table is too wide.
- Preserve general meeting statements by default. Redact only security-sensitive raw strings such as secret/token/password/API key/private key values with `[민감정보 생략]`.
- Do not paste uncertain transcript-backed claims into `결정사항`; put them in `리스크/열린 질문` and the appendix.
- If Canvas update noise matters, mention that workspace/admin Canvas update-message settings may need review before bulk appendix or transcript updates.

## Slack Canvas Write Path

When using Slack tools, prefer direct Canvas creation after the Markdown is structurally validated:

1. Sanitize the Slack API title, especially by removing or replacing literal `&`, then create the complete Canvas in one `create_canvas` call when the body follows Slack Canvas formatting rules and contains the final readable structure.
2. Read the created Canvas back to verify rendered section order, the 2-column metadata table, absence of index-row sections, multi-line decision bullets, checklist actions, topic discussion, follow-up checks, risks/open questions, correction bullets, and transcript appendix.
3. If direct creation fails with `Invalid text passed`, retry once with a sanitized title if the first title contained punctuation such as `&`. If the sanitized title still fails or the failure is clearly a body formatting/length error, fall back to a skeleton Canvas. Read the created Canvas to capture section IDs, and then append or replace complete sections.

Do not leave the skeleton as the final artifact. A Canvas with only a 1-2 line summary or 2-3 actions is a failed forward-test for a long planning meeting. In live Slack verification, a full long meeting body with verbatim transcript returned `canvas_creation_failed: Invalid text passed`, while the skeleton-create plus section-replace path accepted complete readable sections. Therefore, direct create is the preferred path, and skeleton+replace is a verified fallback for long transcript bodies.

When replacing a header section ID, remember that Slack preserves the targeted header and replaces only that header's body content. The replacement body must not repeat the same heading. For example, replacing the `### 원문 전사본 전문` section should send only the transcript code block, not another `### 원문 전사본 전문` line. After each targeted replace, read the Canvas back and reject adjacent duplicate headings such as `### 원문 전사본 전문` repeated twice.

For the meeting index, do not use tools. The user manually binds the meeting Canvas into their Slack List. After read-back verification, return the manual index-binding handoff with `이름`, `Date`, `Topic`, `Status`, `Counts`, and `Meeting Canvas`. Avoid destructive full-canvas replace unless the user explicitly asks for a full rewrite, the existing Canvas is already corrupted and must be repaired, or the just-created test Canvas is visibly wrong and must be corrected.

## Forward Quality Rubric

Use a rubric before calling a meeting-minutes output good enough. Treat the first run as the baseline score and keep improving until the current output meets the pass line.

- Baseline: record the first output's baseline score before edits. A weak but structurally valid first pass is expected to be around 25-70/100.
- Pass line: a reusable meeting Canvas must score at least 92/100 before it is treated as ready for `#dev-team-backend`.
- Do not fabricate meeting dates from the transcript. If the source has no explicit meeting date and the user did not provide one, set `Date` to the same value as `Last updated`. Do not add a separate open question just to confirm the meeting date unless the user asks for calendar reconciliation.
- 실명이 전사본만으로 확정되지 않으면 speaker labels remain uncertain. Use `참석자 1(팀 리드 추정)` or similar wording, then put the exact mapping in `참석자/화자 보정`.
- 용어 보정 후보를 먼저 작성, then use only high-confidence corrected terms in decisions/actions. Low-confidence terms stay in `불확실 단어/문장 보정` and open questions.
- Verbatim full transcript handling: if the user provides transcript text, `원문 전사본 전문` must contain that pasted transcript verbatim in one top-level `text` code block, preserving speaker labels, timestamps, wording, omissions, order, and line breaks as much as the Canvas surface allows. Do not summarize, normalize, translate, or substitute representative blocks under `원문 전사본 전문`.
- The pasted transcript must appear exactly once. Do not append the transcript once during skeleton creation and then append or replace it again during section repair. Before declaring a Canvas ready, compare the `원문 전사본 전문` section against the source transcript by marker count or hash: the transcript start marker, transcript end marker, and full transcript body must each occur once.
- If a hard Slack/API limit prevents inserting the complete transcript after trying the skeleton-create plus targeted section update path, do not label the appendix `원문 전사본 전문`. Rename it to `원문 전사본 발췌`, state that the full source is linked in `Source`, and include the exact source location. This is a degraded output and must be called out before treating the Canvas as ready.
- A passable output must create/read back the meeting Canvas and provide separate manual index-binding handoff values that use Slack's built-in `이름` field as the title. The Canvas body itself should focus on meeting content, not index maintenance.
- A passable created/read-back output must not contain Canvas URL placeholders such as `미정`, `{Canvas 링크}`, or `생성 후 인덱스 참조` after the Canvas URL exists.
- A passable output must not leave known team-lead follow-ups or risks as `참석자 1` when local background and meeting role evidence identify the owner as `김현호 팀리더`.
- A passable output must render risks/open questions as separated fields, not dense one-line records containing `확인 담당`, `확인 방법`, and `상태` in the same bullet.
- A passable output must contain exactly one `### 원문 전사본 전문` heading and one top-level `text` transcript code block under it. Duplicate adjacent transcript headings are a Slack Canvas write-path failure.
- A passable output must include a compact top callout box before metadata. This must use `::: {.callout}` so Slack Canvas renders the rounded box UI, not a plain Markdown paragraph.
- A passable output must follow the Canvas UI recipe: top callout box, vertical metadata table, TL;DR bullets for multi-topic meetings, multi-line decision bullets, checklist actions, meaningful `###` topic headings, one divider before the audit appendix, correction bullets, and a transcript code block.
- A passable output must keep section order coherent for readers: `TL;DR` -> `결정사항` -> `액션 보드` -> `주제별 논의` -> `후속 확인` -> `리스크/열린 질문` -> `보정 및 원문 부록`.
- A passable output must show care for readability: decisions use multi-line bullets, not one dense sentence containing decision, evidence, impact, owner, and status.
- A passable output must not contain one 12-column index table, a secondary index detail table, or any table wider than 5 columns for Slack Canvas. Preserve detail fields in the meeting Canvas metadata and manual handoff, not in an index table embedded in the Canvas.
- A passable output must have enough substance for the input length. For a 30+ minute planning transcript, `TL;DR` plus sections must cover the major topics; a two-line meeting summary is a failed output.
- Re-run `python3 scripts/engelbart_quality_rubric.py` after prompt/skill changes when this repo's fixture is available.

## Slack Recap

Use this when the user asks for a channel message to accompany the Canvas:

```markdown
회의록 Canvas입니다: {Canvas 링크}

- 대상 채널: #dev-team-backend or {channel override}
- 핵심 결론: {1문장}
- 결정사항:
  - {결정}
- 액션 아이템:
  - {담당}: {할 일} ({기한})
- 확인 필요:
  - {열린 질문}
```
