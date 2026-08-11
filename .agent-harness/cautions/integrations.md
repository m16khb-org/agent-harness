---
name: cautions/integrations.md
description: Cautions for host adapters, native hooks, MCP, skills, and external-tool boundaries.
---

# Host, hook, MCP, skill, and external-tool cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for Codex/Claude
host adapters, native hooks, MCP schema and tool-use, shared skills, and
external tools. Host hook output semantics live here; IssueOps-specific hook
procedures (numbered choices, worktree guards) live in
[issueops-lifecycle.md](issueops-lifecycle.md).

Native hook 설치는 lifecycle worktree의 binary를 영구 target으로 쓰거나 실행 중 inode를 제자리 덮어쓰면 안 된다. invoking checkout에서 build하되 Git common-dir의 source `bin/agent-harness`에 staged-file fsync와 atomic rename으로 활성화하고, 이전 target을 캐시한 host session은 재시작한다.

## 1. Host-specific lock-in

Codex plugin 또는 Claude Code hook에 핵심 로직을 넣으면 다른 host에서 같은 동작을 재사용할 수 없다.

주의:
- core behavior는 Go core에 둔다.
- plugin/skill/slash command/hook은 CLI/MCP 호출 wrapper로 제한한다.
- host adapter가 늘어날수록 contract test로 결과 동일성을 확인한다.

## 2. Plugin-only 착각

plugin 방식은 설치 UX에는 좋지만, Codex와 Claude Code가 같은 plugin runtime을 공유하지 않는다.

주의:
- plugin은 배포/발견/문서화 layer로 본다.
- 장기 상태, command policy, audit log는 외부 core/worker가 담당한다.

## 7. MCP schema drift

CLI와 MCP가 서로 다른 응답 의미를 갖기 시작하면 host별 동작이 갈라진다.

주의:
- 새 CLI state command를 추가하면 MCP tool, response contract golden, usage golden을 함께 갱신한다.
- MCP tool이 직접 외부 provider를 호출하지 못하는 경우에도 agent-facing contract에 호출 순서와 실패 조건을 명시한다.
- CLI JSON과 MCP response는 같은 core DTO를 공유한다.
- schema 변경은 golden test와 migration note를 남긴다.
- tool 이름과 field 이름은 안정적으로 유지한다.
- advertised schema가 unknown key를 허용했다면 canonical intent와 달라도 model fault로 분류하지 않는다. `advertised_valid`와 `canonical_valid`를 따로 기록한다.
- `additionalProperties`가 없는 implicit open object를 새로 만들지 않는다. 자유형 map은 schema owner가 `additionalProperties:true`를 명시해야 한다.
- unknown key 삭제, alias 적용, Unicode 수정, string/CSV/bool coercion 같은 silent repair로 malformed call을 성공시키지 않는다.
- raw argument drift는 capture-only probe에서 production handler보다 먼저 관측하고, 동일 signature가 재현되기 전에는 production validator나 tracked regression fixture로 승격하지 않는다.
- `failure_cause`는 typed evidence로만 올리고, evidence가 없거나 상충하면 `unknown`을 유지한다. 기존 반복 패턴 축인 `failure_class`를 덮어쓰지 않는다.

## 9. Shared skill drift

Codex용 skill과 Claude용 skill을 복사본으로 따로 두면 금방 내용이 갈라진다.

주의:
- `skills/<name>`을 원본으로 둔다.
- 기본 설치는 `~/.codex/skills/<name>`과 `~/.claude/skills/<name>`만 중앙 원본으로 연결한다.
- `.claude/skills/<name>` 같은 repo-local 연결은 적용 대상 repo에 커밋될 수 있으므로 명시적 project-local 모드에서만 만든다.
- 스킬 수정 후 user-level host 경로가 같은 원본을 가리키는지 확인한다.

## 12. 외부 도구 의존 재도입 금지

`agent-harness` 설치, 업데이트, self-verify, IssueOps readiness gate는 외부 도구 없이 재현 가능해야 한다. 외부 도구는 사용자가 별도로 설치한 경우에만 일반 파일/명령/MCP 경계에서 참고한다.

주의:
- 하네스 설치 경로에서 외부 도구를 clone/install/register/patch 하지 않는다.
- 외부 도구가 없거나 깨졌다는 이유로 core contract를 약화하거나 readiness gate를 통과시켜서는 안 된다.
- 외부 plugin cache를 하네스가 수정하는 shim을 추가하지 않는다. 문제는 해당 도구의 설치/문서/사용 경로에서 해결한다.
- 외부 도구의 vault, memory store, graph index, query-pack, lifecycle hook 의미를 agent-harness core에 복제하지 않는다.

draft-wiki는 별도 staging/export area다. `.agent-harness/draft-wiki/**`에는 사용자가 검토할 후보 Markdown만 둔다. `agent-harness project draft-wiki promote --confirm`은 승인된 draft를 repo-local `exported/` 디렉토리로 이동하고 `export.log`를 append할 뿐, 외부 wiki ingest/lint/index/query-pack을 완료한 것으로 보고하지 않는다.

draft-wiki queue는 hook 휴리스틱이 자동 생성하지 않는다. UserPromptSubmit은 메인 에이전트에게 장기 재사용 가치 판단 책임과 명시 queue 명령만 알려주고, 메인 에이전트가 의미 있는 후보라고 판단한 경우에만 `agent-harness project draft-wiki queue --stdin`(heredoc 권장) 또는 `--input`으로 적재한다. `agent-harness worker draft-wiki`가 나중에 `agy -p`를 argv로 호출해 draft를 쓴다. hook stdout에는 host-compatible no-op shape를 유지하고, queue/draft 생성 여부는 명시 queue command, queue file, draft file, worker result로 검증한다.

## 14. Codex vs Claude Code hook rendering drift

Codex and Claude Code accept similar UserPromptSubmit JSON, but they do not render it the same way.

주의:
- Codex shows `hookSpecificOutput.additionalContext` in the TUI `hook context:` row, so anything injected for the model is also visible to the user and may be newline-collapsed.
- Claude Code can use `systemMessage` as the user-visible channel while keeping `additionalContext` as model-facing context.
- Do not assume a hook field is hidden just because another host hides it. Verify the installed host runtime/schema before changing hook output.
- Codex 0.144.1 rejects PreToolUse `hookSpecificOutput.permissionDecision="ask"` with `unsupported permissionDecision:ask`. Generic Codex ask-style gates fall back to `decision="block"`; kubectl live-access uses the bounded one-shot flow in §15 (`issueops-lifecycle.md`). Hosts with native ask support keep `permissionDecision="ask"`.
- Codex `hook returned invalid <event> JSON output` means hook stdout looked like JSON but failed strict serde parsing (`deny_unknown_fields`; unknown top-level field, unknown enum value, or truncated/multi-object stdout). It is NOT a generic failure label — 원인 후보를 그 세 가지로 좁히고, 용의 훅 stdout을 `... | cat | wc -c` 파이프 하류에서 재현해 확인한다. Node/Bun 기반 외부 훅은 stdout이 파이프일 때 큰 출력을 512B에서 자르고 종료할 수 있다(2026-07-08 incident 참조). agent-harness Go 훅은 동기 write라 이 문제가 없다.
- For Codex, keep the project-doc catalog in `additionalContext` because the agent needs it, but avoid route/action/profile/pending-upkeep status prose there.
- Keep project-doc frontmatter descriptions concise English metadata; `project bootstrap` and `project bootstrap --sync` use this canonical metadata, so verbose descriptions multiply across every target repo.

## 17. MCP tool-use risks

- Broad tool descriptions make agents over-call tools or pass wrong arguments.
- Always injecting all project documents at session start wastes context and can hide task-specific evidence.
- Writable tools need explicit write semantics; prefer dry-run or append-only behavior.
- Tool output is evidence, not proof: verify file existence, warnings, and command/test results before claiming completion.

## 20. Stop hook output: `continue:false` hard-stops; use `decision:block` + `reason` to continue in-turn

A Stop hook that wants the agent to *recover and keep going* (for example, to present the missing numbered choices) must NOT set `continue:false`. Doing so halts the agent and surfaces the reason to the user, instead of letting the agent act on it in-turn.

주의:
- Verified against host binaries. Claude `2.1.162` embedded hook docs: `continue` — "Set to `false` to block/stop (default: true)", `stopReason` — "Message shown when `continue` is false". Codex `0.137.0` `stop.command.output` schema: `continue` (default true), `decision` = `BlockDecisionWire(["block"])`, `reason` with the note "Claude requires `reason` when `decision` is `block`". Both hosts mirror the same schema.
- `continue:false` is a hard stop and takes precedence over `decision`. To drive an IN-TURN continuation, return `decision:"block"` + `reason` and leave `continue:true` (or omit it). `runHookStop`'s next-action judgement relay branch already does this; the `--enforce-numbered-next-actions` block branch wrongly sent `continue:false`, so the agent "just stopped" and the user had to prompt it manually (observed 2026-06-04, fixed in `cmd/harness/hook_user_prompt.go`).
- When the block branch uses `continue:true`, guard it with `stop_hook_active`: hosts set that flag true on a Stop that is itself a continuation of a prior stop-hook block. Allow the stop (no-op `{}` output) while it is true so a non-complying agent cannot loop forever.
- 모든 `decision:"block"` Stop branch는 `stop_hook_active` continuation과 `자동진행하지 않음` exit를 평가한 뒤에만 재차단할 수 있다. Durable decision state가 그대로라는 이유로 이 guard보다 먼저 무조건 반환하면 같은 hook episode가 영구 재진입한다. 해당 branch 안에서 즉시 `{}`를 반환해야 아래의 독립 relay가 다시 block하지 않는다. 최초 no-auto no-op, ordinary fresh block, relay-enabled choice continuation no-op, 다음 독립 episode 재알림을 한 회귀 테스트로 고정한다.
- Stop hooks accept only the stop-control schema (`continue`/`decision`/`reason`/`stopReason`/`systemMessage`/`suppressOutput`). Injecting `hookSpecificOutput.additionalContext` on Stop makes Codex report "invalid stop hook JSON output"; use a no-op `{}` payload when not blocking.
- The ONLY Stop-hook output reliably surfaced to the user is `decision:"block"` + `reason` — it renders as "Stop hook feedback" AND re-invokes the agent in-turn. Two channels were observed to produce NO visible notice (2026-06-04): a non-blocking `{"systemMessage": ...}` (turn allowed to end), AND `{"continue": false, "stopReason": ...}` — despite the doc claiming "systemMessage — display to the user (all hooks)" and "stopReason — shown when continue is false". Do not rely on either to notify the user from a Stop hook.
- Claude Code labels a successful Stop `decision:"block"` relay as `hook_blocking_error` in the transcript attachment and can surface it as `stop-hook-error` in stream/UI output. Do not treat that label alone as an agent-harness hook process failure. Check the hook command exit code/stderr plus the follow-up `stop_hook_summary`: an intended next-action relay has `preventedContinuation:false`, `level:"suggestion"`, and an empty failure stderr even though the display name says "error". Treat it as a real failure only when the process failed, stderr names a schema/runtime error, or continuation was actually prevented.
- Consequence: you cannot both stop-and-wait AND show the user a message via raw Stop output — the only visible channel (`decision:"block"`) continues the agent. So when a Stop hook reaches a recoverable review point, return `decision:"block"` + a `reason` that instructs the agent to act on the observed facts. The follow-up Stop carries `stop_hook_active=true`; missing-choice recovery still no-ops on that follow-up to avoid loops.
- So the Stop outcomes are: (1) recover/continue in-turn → `decision:"block"` + `reason`; (2) next-action judgement relay → `decision:"block"` + observed facts for the main agent; (3) silent no-op → `{}`. `continue:false` is a hard stop that suppresses the visible feedback, so avoid it for notifications.
- The Stop hook should only treat numbered lines inside an explicit `선택지:`/`Options:`/`Next actions:` section as next-action choices. Explanatory numbered lists can contain words like `추천` and `자동진행` and must not be parsed as next-action choices.
- The Stop hook is not a judge, scorer, classifier, or safety gate. It must not claim "자동진행 후보", calculate scores/thresholds/confidence, classify destructive/safe/reversible/eligible choices, or decide whether the action should run. Its job is only to say that a next-action judgement point was reached and relay inspectable facts such as choice count, recommendation count, and recommended text. The main agent owns safety, reversibility, user-intent alignment, and proceed-or-ask judgement from current context, and must state that judgement in the recovery response: either why it is auto-proceeding now, or why it is not auto-proceeding and needs user confirmation. If it auto-proceeds, the result report still needs a `선택지:` section so the next action boundary remains explicit.
- A main-agent `no-auto-proceed` judgement is sticky. If the agent says it will not auto-proceed at a Stop-hook next-action boundary, an automated `/goal`/goal-continuation prompt must not immediately reinterpret the active objective as permission to resume the same action. Resume only after an explicit user choice or a new user instruction. Observed 2026-06-06: the agent said it would stop for diff review, then a goal continuation message arrived and the agent resumed implementation, contradicting the prior judgement.
- A `no-auto-proceed` Stop-hook recovery response should be allowed to stop without adding a new `선택지:` block. Repeating the choices in that response creates a fresh next-action judgement point and can produce the exact "recommend -> no-auto-proceed -> recommend" loop. The missing-choice guard should require choices for ordinary final responses, but no-op when the final response explicitly says `자동진행하지...` / `no-auto-proceed`.
- 사람의 입력만 기다리는 판단 지점에서는 agent/background wait 도구를 호출하지 않는다. 현재 응답을 끝내고 다음 실제 사용자 turn에서 재개한다. 대기 도구로 turn을 붙잡는 것은 종료 경로가 아니며, Stop 재진입 결함과 결합하면 토큰만 소비하는 무한 대기가 된다.
- supervised IssueOps owner는 worker worktree에서 `scripts/install-native.sh` 또는 `agent-harness install`/`install-native`/`update`/`bootstrap`의 실제 실행으로 사용자 범위 통합을 교체할 수 없다. owner 빌드가 `~/.codex/hooks.json` 등을 가리키게 만들면 source checkout의 수정과 런타임 provenance가 분리되어 이미 고친 Stop 결함도 다시 활성화된다. `--dry-run` 검증은 허용하고 실제 설치·업데이트는 source checkout에서만 수행한다.
- UserPromptSubmit must not clear `stop-next-action-relay.json` for automated goal-continuation or Stop-feedback prompts. The relay file is the duplicate-suppression guard for repeated `선택지:` responses after a no-auto-proceed judgement; clear it only for an explicit next-action instruction or real progress such as PostToolUse. Observed 2026-06-10: a no-auto-proceed response with choices could be relayed repeatedly when an automated continuation cleared the relay before another Stop.
- New installs use only `--relay-next-action-judgement` for that relay path. Do not reintroduce auto-proceed aliases or hook-side scoring paths.
- `stop_hook_active` must not suppress main-agent judgement when the recovery response now includes valid next-action choices. It should suppress only missing-choice recovery loops. Otherwise the agent can present `선택지:` after a block and then silently stop instead of either proceeding or explaining why it needs user confirmation.

## 23. Skill validation must not depend on host-managed system skill copies

Codex and Claude system skills can be re-materialized by the host and can depend
on optional host-side Python packages. Do not use
`~/.codex/skills/.system/.../quick_validate.py` or a marketplace plugin copy as
the required agent-harness skill validation gate.

주의:
- Use `python3 scripts/validate-skill.py skills/<skill-name>` for
  agent-harness skill metadata validation.
- Treat upstream `openai/codex` `quick_validate.py` fixes as quality pointers,
  not agent-harness completion dependencies.
- If a host-managed validator fails because PyYAML is unavailable, that may be
  valid upstream evidence, but it must not block local agent-harness
  verification when the repo-owned validator passes.

## 29. Slack List create payload와 readback schema는 다를 수 있다

`slackLists.items.create` 입력 schema와 `slackLists.items.list` readback schema를 같은 것으로 가정하면 live List write가 실패한다. 2026-07-01 Engelbart live E2E에서 link 컬럼을 readback 모양인 `originalUrl`로 보냈더니 `missing required field: original_url`로 실패했다. 같은 link는 readback에서는 `originalUrl`로 보이지만 create payload는 `original_url`을 요구한다.

주의:
- Slack List link 컬럼 생성 payload는 `{"link":[{"original_url":"https://..."}]}`를 사용한다. readback의 `originalUrl`을 그대로 create payload로 재사용하지 않는다.
- 회의록 List의 `이름` 컬럼은 날짜를 포함하지 않는 topic-prefix 제목을 쓴다. 날짜는 `회의일` 컬럼에 따로 들어간다. 예: `[AI] Vertex BYOK 비용 비교 회의`, `[배포] TC NCP 마이그레이션 및 플랫폼 정책 회의`.
- Canvas 제목 규칙(`YYYY-MM-DD [Topic] Title`)과 List row 제목 규칙(`[Topic] Title`)을 혼동하지 않는다.
- Raw Slack Web API `canvases.create/edit`의 `document_content` 지원 목록은 `quote block`은 포함하지만 `callout`은 명시하지 않는다. Connector의 Canvas-flavored markdown은 callout을 문법으로 받을 수 있어도, raw Web API 경로에서 `::: {.callout}`이 일반 문단으로 보이면 quote block(`> ...`) 또는 검증된 connector 경로로 대체한다.
- 팀 공용 Slack List 테스트 write는 삭제/재생성해도 알림이나 흔적이 남을 수 있다. 테스트 항목은 `[TEST] ...`처럼 명확히 표시하고, 사용자 승인 없이 rename/delete/recreate 하지 않는다.
- Slack List schema regression test는 아직 구현하지 않았다. Live Slack 쓰기 부작용 때문에 자동 테스트로 옮기기 전에는 connector fixture 또는 승인된 isolated test List가 필요하다.

## 기본 context hook 축소에서 third-party group을 지우지 말 것

기본 설치 표면을 `SessionStart`/`PostCompact`로 줄일 때 managed hook만 지워야 한다. known lifecycle event 전체를 비우면 unrelated host integration까지 제거하고 Codex trust key가 불필요하게 drift한다.

- upgrade는 모든 known event에서 agent-harness command group만 제거한 뒤 third-party group의 상대 순서를 보존한다.
- 기본 surface 밖 `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PreCompact`, `Stop`에는 managed group이 남아서는 안 되며, empty event key만 제거한다.
- child-host smoke는 exact two-event configuration과 simple native episode에서 `PreToolUse`가 관찰되지 않음을 함께 검증한다. PostCompact actual delivery는 compaction 발생이 host-controlled이므로 installed config contract로 검증한다.

## Codex co-resident hook merge에서 matcher 배열 위치를 바꾸지 말 것

Codex hook trust는 command 내용만이 아니라 `source:event:matcher-index:hook-index` key의 `trusted_hash`에 결합된다. installer가 agent-harness 그룹을 제거한 뒤 끝에 append하면 co-resident Orca hook과 배열 위치가 바뀌고, 두 command가 모두 상대방의 stored hash와 비교되어 `modified`가 된다.

- 기존 agent-harness 그룹은 첫 발견 위치에서 in-place replacement하고, 유효한 제3자 그룹의 상대 순서를 보존한다.
- agent-harness 그룹이 없을 때만 append하고, 중복 agent-harness 그룹은 한 개로 축약한다.
- install JSON만으로 runtime hook trust를 주장하지 않는다. Fresh native session의 실제 SessionStart/PreToolUse smoke와 설치된 hook readback을 함께 확인한다.
- automation에서 `--dangerously-bypass-hook-trust`를 사용한 결과는 command 동작 증거일 뿐 정상 trust 상태의 증거가 아니다.
