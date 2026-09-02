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
- 문서가 현재 코드/사용자 컨센서스와 어긋나면 MCP `project_docs_read`로 현재 SHA를 확인하고 `project_docs_revise`로 한 문서씩 갱신한다.
- remote VCS 작업은 canonical worktree의 선택 문서 `.agent-harness/VCS.md`를 먼저 읽는다. 문서에 없는 provider recipe를 실제로 성공시켰다면 같은 worktree에서 `project_docs_read` 후 `project_docs_revise` SHA-CAS로 갱신한다. GitLab/GitHub 모두 기록할 수 있지만 secret, 개인 tool 경로, server namespace, 추측한 MCP 이름은 남기지 않으며 OpenWiki 자동 update를 실행하지 않는다.
- 구조 선택이나 대안 기각 사유가 생기면 MCP `project_docs_append(kind=adr)`로 `.agent-harness/ADR.md`에 남긴다.
- 반복 실패, false case, 위험한 운영 주의는 MCP `project_docs_append(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 남긴다.

## Verify

`AGENTS.md`의 Goal-Driven Execution 원칙을 기본으로 하고, 이 프로젝트에서는 다음 검증 라우팅을 추가한다.

- 테스트를 작성/수정할 때는 `.agent-harness/TESTING.md`의 good/bad test 기준을 먼저 확인한다.
- CLI/MCP/API 문서 계약을 바꾸면 golden/schema/smoke 검증을 함께 실행한다.
- 코드·계약 변경을 완료 보고하기 전에는 가능한 경우 `agent-harness verify-work --json -- <read-only verification command>` 또는 `agent-harness guard check --json` 결과를 evidence로 만든다. Hook은 이를 대신 실행하지 않는다.
- 실행한 테스트/빌드/정적검사 결과와 실행하지 못한 검증의 이유는 완료 보고에 포함한다.

## Finish

- 커밋이 필요하면 `.agent-harness/COMMIT_POLICY.md`를 따른다.
- 해결한 false case나 구조 결정은 필요한 경우 MCP `project_docs_append`로 기록한다.

## Hook context injection and lifecycle observation

Codex/Claude 기본 설치는 `agent-harness hook session-start`를 context-only로 사용한다. 이 installed hook은 필요한 project docs를 판단할 수 있게 static catalog만 주입하며, IssueOps authority나 lifecycle state를 관찰·갱신하지 않는다.

- **SessionStart**: 안정적인 project-doc catalog를 `startup`·`resume`·`clear`·`compact` 모든 source에서 주입한다(`hook session-start --host codex|claude`). 두 host 모두 압축 뒤 `SessionStart`를 `source:"compact"`로 다시 실행하므로 압축 후 컨텍스트도 이 hook이 담당한다. Codex는 `additionalContext`에 user-visible catalog를 싣고, Claude Code는 `systemMessage`(pretty catalog)와 compact `additionalContext`를 분리한다. IssueOps list/read, lifecycle reminder, runtime diagnostic, telemetry, SQLite maintenance, worker recovery, state write를 수행하지 않는다.
- **post-compact**: Omo 확장이 `session_compact` 이벤트에서 `hook post-compact --json`으로 같은 catalog를 읽는다. Codex/Claude에는 등록하지 않는다(두 host의 `PostCompact` 출력은 사용자 표시 전용이다). 다른 hook subcommand는 없다.

IssueOps에서 authority는 `agent-harness issueops ...` CLI/MCP 하나다. 이 경로가 durable phase enum `problem -> grill -> plan -> compatibility-review -> implement -> ai-slop-clean -> feedback -> pr -> done`과 durable record를 소유한다. 원격 이슈 연결(remote issue linkage)은 `grill`과 `plan` 사이의 작업 흐름 단계이며, cleanup은 `done` 뒤의 후처리다. 사용자에게 보이는 작업 흐름에는 이 두 단계가 포함되지만, 둘 다 phase enum에는 포함되지 않는다. Default hooks는 issue 생성, 파일 편집, 테스트 실행, background wait, branch/worktree 준비, PR/MR 생성, review reply, merge, cleanup을 직접 수행하지 않으며 IssueOps record도 읽지 않는다.

IssueOps 자동 루프는 missing gate를 읽고 해당 state owner command를 실행한 뒤 readiness를 다시 확인한다. 예를 들어 `intent_contract`는 `issueops intent record`, `plan_prep_*`는 `issueops plan-prep record`, `branch_prepare`는 `issueops branch prepare`, `design_review`는 `issueops design review`, canonical worktree/lease는 `issueops execution prepare`, current holder와 복구 안내는 `issueops execution status`, compatibility는 `issueops compatibility review`, `plan_path`는 `issueops link-plan`, `domain_review`는 `issueops domain-review record`, split/child readiness는 각각의 owner command, cleanup/verification evidence는 `issueops ai-slop-clean record`, `project_docs_review`는 `issueops project-docs-review record`, `schema_evidence`는 `issueops schema-evidence record`, `feedback_resolution`은 `issueops feedback resolve`, contract-changing feedback은 remote issue body update 후 `issueops feedback mark-issue-updated`가 소유한다.

`issueops execution prepare`를 confirm한 뒤에는 반환된 canonical path를 `HARNESS_EXPECTED_WORKTREE`에 반영한다. 이후 편집은 그 절대경로, `git -C "$HARNESS_EXPECTED_WORKTREE"`, worktree-rooted CodeGraph/`rg`/test 명령으로 수행하고, source checkout과 worktree의 `git status --short`를 따로 확인한다.

선택적 Orca 실행은 `issueops execution prepare --mode auto|direct|orca`로 선택한다. Preview와 confirm 요청은 `--confirm` 외에는 동일해야 한다. Orca가 첫 external mutation 전에 absent/unready이면 `auto`는 direct를 선택한다. Orca가 선택되면 private issue/context packet과 fully rendered prompt를 봉인하고 fresh native owner를 launch한다. Owner는 status가 렌더한 `issueops execution claim --claim-current-token` 명령에서 두 digest를 검증한 뒤에만 쓴다. CLI가 현재 generation의 private token path를 내부에서 결정하므로 token path나 값은 owner prompt에 넣지 않는다. 외부 mutation 전에는 pending intent를 durable state에 먼저 저장하며, 모호한 결과는 create를 재시도하거나 direct로 전환하지 않고 `issueops execution reconcile`로 정확히 하나의 결과를 확인한다. 구현, 검증, generation-fenced PR/MR 생성, `issueops execution complete`는 active holder가 canonical worktree에서 수행한다. Complete는 `done`과 lease release를 원자적으로 기록하며 merge와 cleanup은 하지 않는다.

완료된 execution을 새 generation과 새 HEAD로 복구할 때는 status/preview가 제공한 typed reseed만 사용한다. Completion-bearing reseed는 이전 영수증을 append-only `completion_history`로 보존하고 current completion, done phase, HEAD-bound proof를 같은 CAS에서 재개 상태로 바꾼다. Future completion receipt는 generation을 직접 stamp한다. Generation이 없는 legacy receipt는 현재 lease나 timestamp로 origin을 추론하지 말고 durable incident evidence로 확인한 `--completion-generation N`을 preview와 reseed에 동일하게 전달한다. 누락·충돌은 mutation 전에 fail-closed한다. 새 owner는 status의 exact `resume`/`claim`을 실행하고 `implement`부터 AI-slop, implementation review, `pr`, completion까지 정상 순방향 gate를 다시 통과한다. History는 감사 기록이지 현재 generation의 retry evidence가 아니며 state 파일 수동 편집으로 제거하지 않는다.

Delegated child cycle은 parent plan/evidence가 sub-agent pattern slug, 기대 이득, tradeoff, fallback, verification을 기록한 뒤에만 시작한다. Parent와 child가 같은 repo를 보더라도 각자의 exact lifecycle ID, execution generation, native actor, canonical worktree가 분리되어야 한다. Child는 자기 worktree와 execution lease 안에서만 작업하고, parent main agent가 결과를 검증해 accept/reject를 기록한다.

독립적인 일시 fan-out은 host의 native subagent concurrency controls를 사용한다. Durable delegated work는 IssueOps child cycle, isolated canonical worktree, generation-fenced execution ownership, 그리고 parent accept/reject validation을 사용한다.

phase 진입은 fail-closed다: `grill` 진입은 problem 완료(`intent_contract`)를, `plan` 진입은 grill 완료(`issue_url`+`branch`+`plan_prep`+`split_decision`+`domain_review`)를 요구한다. `phase_ledger`는 phase 전이 시 entered_at/completed_at를 stamp하고, 없으면 `issueops status --json`이 sentinel timestamp로 파생해 보여준다 — resume 시 이 ledger와 missing artifact로 어느 phase부터 이어갈지 판단한다. plan/compatibility-review에서 `brooks` devil's-advocate(sub-agent-only)가 `stop`을 내면 `issueops regress --id --reason`으로 `grill`로 회귀해 재조사·재계획한다(design 승인 무효화 + plan/compat ledger stale 표기). 판정 기록은 검토한 플랜의 sha256(`reviewed_plan_digest`)에 묶이므로, 판정 뒤 플랜을 고쳤으면 최종 플랜으로 brooks를 다시 돌려 기록해야 implement에 들어갈 수 있다(`devils_advocate_review_stale`; implement 진입 이후의 플랜 편집은 게이트 대상이 아니다). 안전성, 되돌릴 수 있음, 사용자 의도 정합성, sub-agent tradeoff 판단이 흔들리는 지점은 hook이 판단하지 않고 main agent가 세 가지 선택지로 멈춘다.

## MCP Usage Rule

- MCP는 모델 기억 대신 현재 repo 상태, 문서 라우팅, 정책 판정, state checkpoint, durable record가 필요할 때 사용한다.
- 단순 추론이나 이미 열린 파일의 요약에는 MCP를 쓰지 않는다.
- 작업 시작 시 가능한 경우 `project_docs_route`에 현재 task를 넣어 필요한 문서만 고른다.
- 문제가 발생했고 해결했다면 `project_docs_append(kind=caution)`으로 `.agent-harness/CAUTIONS.md`에 기록한다.
- 구조 결정이나 대안 기각 사유가 생겼다면 `project_docs_append(kind=adr)`로 `.agent-harness/ADR.md`에 기록한다.
- 많은 도구를 한 번에 쓰기보다 route/read/update/check/record처럼 의도가 분명한 도구를 좁게 사용한다.
- tool 결과는 경로, `exists`, warning, 검증 증거를 확인한 뒤 작업에 반영한다.

## .agent-harness Upkeep via MCP

최초 bootstrap 이후 `.agent-harness` 문서는 고정 산출물이 아니라 에이전트가 작업 증거와 사용자 컨센서스를 반영해 최신화하는 운영 문서다.

- 작업 시작: `project_docs_route`로 필요한 문서만 고른다.
- 문서 갱신: `project_docs_read`로 현재 content/SHA를 읽고, 보존할 내용과 새 근거를 합쳐 `project_docs_revise(confirm=true)`로 한 문서씩 갱신한다.
- false case/반복 문제: `project_docs_append(kind=caution)`으로 append한다.
- 결정/대안 기각: `project_docs_append(kind=adr)`로 append한다.
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

## Execution v1 workflow

After the provider-linked branch and exact base SHA are recorded, preview and
confirm `issueops execution prepare --mode auto`. GitHub Orca is the exception:
record the matching provider/issue identity and exact base SHA first, prepare
the local-only Orca branch, then create and record the linked branch before
plan linkage and implementation. Direct mode grants the calling native session
generation 1; Orca mode launches one owner that verifies the sealed
issue/context digests and asks the CLI to consume the current-generation private token.
The active holder performs planning, implementation, publication, and
completion from the canonical worktree. The source main worktree remains
available before, during, and after execution for unrelated work.

GitLab-linked execution은 특정 MCP server 이름이 아니라 semantic leaf
`glab_api`와 실제 schema로 snapshot capability를 찾는다. 개인 wrapper도 같은
leaf와 compatible schema를 노출하면 후보가 된다. `web_url`, `description`,
`state`를 linked authority/project/IID와 대조한 뒤 MCP에는 host-neutral
`issue_snapshot`을, CLI에는 exact mode `0600` file과
`--issue-snapshot-file`을 넘긴다. 후보 부재나 호출 실패 뒤 successful
exact-identity MCP evidence가 없을 때만 인자를 생략해 generic `glab api`
fallback을 사용한다. 이미 공급한 invalid evidence는 fallback하지 않는다. 성공 결과의
`issue_snapshot_source`가 `glab_mcp` 또는 `glab_cli`인지 확인한다.

Do not use source CWD or a generic session binding as a fence. Select the exact
lifecycle ID, generation, native process receipt, canonical worktree, and
persisted Orca resource. One active execution exists per record, so unrelated
cycles remain independent. Completion records `done` and releases the lease;
merge and destructive cleanup require separate authority.

## 이원 구조 흐름 요약

메인 세션(planner)은 조사→이슈→설계→artifact stage→execution prepare(orca)까지 수행하고 핸드오프하면 그 사이클에 대한 임무가 끝난다 — lease를 보유하지 않으므로 즉시 다른 사이클을 계획할 수 있다. 하위 세션(implementer)은 claim→staged plan link→`phase=implement`→artifact 기반 TDD→AI-slop evidence→`phase=ai-slop-clean`→project-doc 반영 판정(project-docs-review)과 스키마 실측 근거(schema-evidence, 스키마 변경 시)→planner급 brooks 리뷰(implementation-review pass)→atomic-commit-push→`phase=pr`→governed create-pr→execution complete로 종료한다. 이 exact 명령은 sealed owner packet이 제공하며 owner가 추론해서 보충하지 않는다. 휴먼 머지 후 정리는 reflect-completion→close-issue→cleanup finish 순서를 지킨다(OPERATIONS.md 참조).
