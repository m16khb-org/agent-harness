---
name: AGENT_WORKFLOW.md
description: Agent start, execution, verification, and completion flow.
---

# Agent Workflow

## Start

1. `AGENTS.md`를 먼저 읽는다.
2. 세션 시작 시 `.agent-harness/CONSTITUTION.md`를 기본 원칙으로 확인한다.
3. 작업 종류에 맞는 `.agent-harness/` 문서를 확인한다.
4. 현재 파일과 명령 출력으로 문서의 추정 항목을 검증한다.

## Work

`AGENTS.md`의 Simplicity First/Surgical Changes 원칙을 기본으로 하고, 이 프로젝트에서는 다음 기록·안전 규칙을 추가한다.

- 기존 사용자 변경을 덮어쓰지 않는다.
- 새 dependency, 배포, destructive action은 명시 지시나 강한 근거가 있을 때만 진행한다.
- 문서가 현재 코드/사용자 컨센서스와 어긋나면 MCP `project_docs_read`로 현재 SHA를 확인하고 `project_docs_update`로 한 문서씩 갱신한다.
- 구조 선택이나 대안 기각 사유가 생기면 MCP `project_docs_record(kind=adr)`로 `.agent-harness/ADR.md`에 남긴다.
- 반복 실패, false case, 위험한 운영 주의는 MCP `project_docs_record(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 남긴다.

## Verify

`AGENTS.md`의 Goal-Driven Execution 원칙을 기본으로 하고, 이 프로젝트에서는 다음 검증 라우팅을 추가한다.

- 테스트를 작성/수정할 때는 `.agent-harness/TESTING.md`의 good/bad test 기준을 먼저 확인한다.
- CLI/MCP/API 문서 계약을 바꾸면 golden/schema/smoke 검증을 함께 실행한다.
- 코드·계약 변경을 완료 보고하기 전에는 가능한 경우 `agent-harness verify-work --json -- <read-only verification command>` 또는 `agent-harness guard check --json` 결과를 evidence로 만든다. Hook은 이를 대신 실행하지 않는다.
- 실행한 테스트/빌드/정적검사 결과와 실행하지 못한 검증의 이유는 완료 보고에 포함한다.

## Finish

- 커밋이 필요하면 `.agent-harness/COMMIT_POLICY.md`를 따른다.
- 해결한 false case나 구조 결정은 필요한 경우 MCP `project_docs_record`로 기록한다.

## Hook context injection and lifecycle observation

Codex/Claude host가 지원하면 `agent-harness hook ...`가 시점을 나눠 context를 주입하거나 lifecycle state를 관찰한다. 모두 자동 실행 명령이 아니라 agent가 필요한 문서/MCP를 판단하기 위한 reminder이며, hook은 차단/대행 실행을 하지 않고 project docs/API docs/CAUTIONS/ADR 갱신 후보만 제안한다.

- **SessionStart / PostCompact**: 안정적인 project-doc catalog를 주입한다(`hook session-start`, `hook post-compact`). Codex는 `--host codex`로 설치되어 `additionalContext`에 user-visible catalog를 싣고, Claude Code는 `systemMessage`(pretty catalog)와 compact `additionalContext`를 분리한다. Active supervised handoff에서는 SessionStart가 native host/session과 exact claim command를 추가하지만 상태를 변경하지 않는다. PostCompact는 compaction으로 사라진 SessionStart catalog를 lifecycle reminder와 함께 재주입한다.
- **UserPromptSubmit**: 매 지시마다 catalog가 아니라 dynamic per-turn hint(routing, actions, profile, pending upkeep, rule)만 싣는다(`hook user-prompt`). 안정적 catalog는 더 이상 per-turn으로 주입하지 않는다(`cmd/harness/hook_user_prompt.go:69-83`, `internal/core/hook_prompt.go:104-113`).
- **PreToolUse**: tool 실행 직전의 빠른 preflight 지점이다(`hook pre-tool-use`). 기본 host stdout은 `{}`이고 raw `--json`은 allow/no-op 진단을 반환한다. Active supervised handoff는 예외적으로 persisted attempt/context/native host-session/worktree ownership이 불일치하는 mutation을 fail closed로 막는다. 이 block schema는 Codex, Claude, GJC native adapter별로 검증한다.
- **PostToolUse**: 성공한 tool 실행 후 경량 관찰 지점이다(`hook post-tool-use`). lifecycle-relevant 파일을 실제로 수정할 수 있는 tool/명령만 user-state queue에 남기고, read-only 조회 결과나 shared docs를 직접 수정하지 않는다.

IssueOps에서 hook 경계는 더 좁다. `agent-harness issueops ...` CLI/MCP가 `problem -> grill -> issue -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> cleanup` 상태 머신과 durable record를 소유한다. Hook은 worktree edit target, Korean remote artifact, VCS issue-linking metadata, PR/MR base branch, label/assignee, numbered next-action format처럼 빠르고 결정적으로 inspect 가능한 위반만 막는다. Hook은 issue 생성, 파일 편집, 테스트 실행, background wait, branch/worktree 준비, PR/MR 생성, review reply, merge, cleanup을 직접 수행하지 않는다.

IssueOps 자동 루프는 missing gate를 읽고 해당 state owner command를 실행한 뒤 readiness를 다시 확인한다. 예를 들어 `intent_contract`는 `issueops intent record`, `plan_prep_*`는 `issueops plan-prep record`, `branch_prepare`는 `issueops branch prepare`, `worktree_tools_*`는 `issueops worktree prepare-tools`, `compatibility_review`와 하위 compatibility key는 `issueops compatibility review`, `execution_decision`은 `issueops execution decide`, `design_review`는 `issueops design review`, `plan_path`는 `issueops link-plan`, `domain_review`는 `issueops domain-review record`, `split_decision`은 provider-native child(`issueops link-child`) 또는 no-split scope decision(`issueops decision add --kind scope`), child cycle readiness는 `issueops child status`/`issueops child accept`, workpool readiness는 `workpool status`/`workpool accept`, cleanup/verification evidence는 `issueops ai-slop-clean record`, `feedback_resolution`은 `issueops feedback resolve`, `contract_feedback_issue_update`는 remote issue body update 후 `issueops feedback mark-issue-updated`가 소유한다.

IssueOps worktree 세션을 resume하거나 `issueops worktree prepare-tools`를 실행한 뒤에는 출력된 `export HARNESS_EXPECTED_WORKTREE=<worktree>`를 현재 shell/session에 반영한다. 이후 편집은 worktree 절대경로, `git -C "$HARNESS_EXPECTED_WORKTREE"`, worktree-rooted `rg`/test 명령으로 수행하고, source checkout과 worktree의 `git status --short`를 따로 확인한다.

선택적 Orca 실행은 `issueops worktree prepare --orchestrator auto|orca|inline`로만 선택한다. `auto`의 probe-only 실패는 기존 inline bind/heartbeat를 그대로 유지하고 `execution_handoff`를 남기지 않는다. Orca가 선택되면 coordinator가 prepare/start, fresh worker가 claim/heartbeat/finish, coordinator가 evidence 재검증/accept를 소유한다. 외부 create/dispatch 호출 전 `pending_operation`이 먼저 저장되며, 모호한 실패에서는 create를 재시도하거나 inline으로 전환하지 않고 `issueops handoff recover --action reconcile`로 정확히 하나의 identity만 복구한다.

Delegated child cycle은 parent가 `issueops execution decide`에 sub-agent pattern slug, 기대 이득, tradeoff, fallback, verification을 기록한 뒤에만 시작한다. Child session은 `issueops resume --id <child-id>`로 재개하고, parent와 child가 같은 repo를 보더라도 scoped binding이 각자의 expected worktree를 분리해야 한다. Child는 자기 worktree에서 heartbeat/status를 갱신하고, parent main agent가 결과를 검증해 accept/reject를 기록한다.

Workpool worker loop는 claim -> isolated worktree 준비 -> heartbeat -> task 수행 -> result submit -> main-agent accept/reject 순서다. Worker liveness는 PID가 아니라 lease timestamp로 판단하며, heartbeat가 실패하거나 lease를 잃으면 worker는 멈추고 새 claim을 하지 않는다. Pool의 분해·우선순위·최종 safety/reversibility/user-intent 판단은 main agent가 유지한다.

phase 진입은 fail-closed다: `grill` 진입은 problem 완료(`intent_contract`)를, `plan` 진입은 grill 완료(`issue_url`+`branch`+`plan_prep`+`split_decision`+`domain_review`)를 요구한다. `phase_ledger`는 phase 전이 시 entered_at/completed_at를 stamp하고, 없으면 `issueops status --json`이 sentinel timestamp로 파생해 보여준다 — resume 시 이 ledger와 missing artifact로 어느 phase부터 이어갈지 판단한다. plan/compatibility-review에서 `brooks` devil's-advocate(sub-agent-only)가 `stop`을 내면 `issueops regress --id --reason`(MCP `issueops_regress_for_replan`)으로 `grill`로 회귀해 재조사·재계획한다(design 승인 무효화 + plan/compat ledger stale 표기). 안전성, 되돌릴 수 있음, 사용자 의도 정합성, sub-agent tradeoff 판단이 흔들리는 지점은 hook이 판단하지 않고 main agent가 세 가지 선택지로 멈춘다.

## MCP Usage Rule

- MCP는 모델 기억 대신 현재 repo 상태, 문서 라우팅, 정책 판정, state checkpoint, durable record가 필요할 때 사용한다.
- 단순 추론이나 이미 열린 파일의 요약에는 MCP를 쓰지 않는다.
- 작업 시작 시 가능한 경우 `project_docs_route`에 현재 task를 넣어 필요한 문서만 고른다.
- 문제가 발생했고 해결했다면 `project_docs_record(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 기록한다.
- 구조 결정이나 대안 기각 사유가 생겼다면 `project_docs_record(kind=adr)`로 `.agent-harness/ADR.md`에 기록한다.
- 많은 도구를 한 번에 쓰기보다 route/read/update/check/record처럼 의도가 분명한 도구를 좁게 사용한다.
- tool 결과는 경로, `exists`, warning, 검증 증거를 확인한 뒤 작업에 반영한다.

## .agent-harness Upkeep via MCP

최초 bootstrap 이후 `.agent-harness` 문서는 고정 산출물이 아니라 에이전트가 작업 증거와 사용자 컨센서스를 반영해 최신화하는 운영 문서다.

- 작업 시작: `project_docs_route`로 필요한 문서만 고른다.
- 문서 갱신: `project_docs_read`로 현재 content/SHA를 읽고, 보존할 내용과 새 근거를 합쳐 `project_docs_update(confirm=true)`로 한 문서씩 갱신한다.
- false case/반복 문제: `project_docs_record(kind=caution)`으로 append한다.
- 결정/대안 기각: `project_docs_record(kind=adr)`로 append한다.
- 불확실한 사실은 단정하지 말고 `Unknown / not confirmed`와 검증 방법을 남긴다.

## Evidence-backed MCP Heuristics

- Tool 선택은 자연어 설명 품질에 민감하므로 tool description은 목적, 사용 조건, 쓰기 여부, 반환 구조를 명확히 유지한다.
- 자주 필요한 문서 전체를 세션에 항상 주입하지 말고, task별 라우팅으로 context를 줄인다.
- 쓰기 도구는 append-only 또는 dry-run 기본값처럼 실패 반경을 제한한다.

## API documentation gate

- Endpoint/controller/DTO/schema/OpenAPI changes require the API documentation gate before completion.
- Prefer `agent-harness api-doc static-check` or MCP `api_doc_static_check`, then `api_doc_review` to render the host-agent prompt/schema and record the supplied review result; both default to staged API candidate files so legacy Swagger/OpenAPI debt is not failed all at once.
- For NestJS Swagger projects, the gate must catch missing `@ApiOperation`, missing/invalid operation descriptions, missing `@ApiParam`/`@ApiHeader`, missing 400/401 responses, and DTO `@ApiProperty`/`@ApiPropertyOptional`/`@IsOptional` mismatches.


## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.
