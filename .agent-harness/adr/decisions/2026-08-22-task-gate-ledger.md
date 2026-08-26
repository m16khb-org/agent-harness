# Task gate ledger (unlazy-compatible gates capability)

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-22
- Status: accepted

## Context

하네스의 기존 검증 표면(looprun, self-verify, IssueOps readiness)은 모두
**하네스가 소유한 상태**를 게이트한다. 그러나 에이전트가 하나의 태스크를
"다 했다"고 선언하는 순간의 진위를 판정하는 표면은 없었다:

- loop는 반복 시도(verdict/evidence)를 기록하지만 수용 기준 자체는 에이전트
  머리 속에 있다.
- IssueOps readiness는 아티팩트·리뷰·git 상태를 검사하지만 "이 사이클이
  실제로 하기로 한 결과물"과 대조하지 않는다.
- self-verify는 하네스 자체의 품질을 게이트한다(95점 루프).

unlazy v2(Leonxlnx/unlazy, MIT)는 이 빈칸을 "게이트 ledger"로 채운다: 작업
시작 전 수용 기준을 gate ledger 파일로 쓰고, CHECK 명령의 실행 결과로만
체크박스를 채우며, EVIDENCE가 pending인 체크는 미충족으로 판정한다.
"prose는 prose를 강제할 수 없다 — 파일과 명령이 강제한다"는 것이 검증된
통찰이고, 이는 agent-harness의 검증 우선 원칙과 정확히 같은 결론이다.

## Decision

1. **unlazy 게이트 파일 형식을 그대로 채택한다.** `- [ ] G1: outcome` +
   `CHECK:`/`EXPECT:`/`EVIDENCE:` + `ABANDON:` 라인. 파서/판정기
   (`internal/domain/gates`)는 unlazy `gate-check.mjs`의 규칙을 준수한다:
   체크박스는 주장이고 EVIDENCE가 증명이며, checked+pending은 unchecked보다
   심각한 미충족이다. 기존 unlazy 스킬 사용자의 GATES.md가 하네스에서 그대로
   동작한다. 신규 `gates init`의 기본 경로는 scope에서 유도한
   `.agent-harness/gates/<scope-slug>.md`이고, IssueOps는 병렬 worktree merge
   충돌을 막기 위해
   `.agent-harness/gates/issue-<provider-issue-number>.md`를 명시한다.
2. **CHECK 명령은 raw shell이 아니라 command policy 경로로 실행한다.**
   unlazy의 `spawnSync(cmd, {shell: true})`와 달리, 하네스 게이트는
   `shelltoken`으로 argv 토큰화하고 `policy.EvaluateCommandPolicy` +
   `policy.RunCommand`(workspace 경계, env allowlist, secret redaction,
   timeout, audit log, shell interpreter 금지)를 통과해서만 실행한다.
   "게이트를 채우려면 명령을 실행해야 한다"가 정책 우회의 사후 승인이
   되지 않도록 한다.
3. **파일 형식은 보존 파싱으로 다룬다.** 파서는 원본 라인 배열을 유지하고
   게이트 라인/EVIDENCE 라인만 교체하므로 CRLF, 산문, 사용자 주석이
   round-trip에서 소실되지 않는다(도메인 테스트로 고정).
4. **CLI와 MCP가 같은 contract DTO를 공유한다.** `agent-harness gates
   init|check|status|report|abandon`과 MCP `gates_init|gates_check|
   gates_status|gates_report|gates_abandon`는 같은 `internal/contract/gates`
   타입을 쓴다(스키마 버전 1). exit code는 unlazy 호환: 0 전부 충족,
   1 미충족 잔존, 2 사용법 오류.
5. **IssueOps PR readiness에 게이트를 합성한다(gatesgate).** worktree에
   `.agent-harness/gates/*.md` 또는 호환 gate ledger가 존재하면 미충족 게이트가 `gates_incomplete:<file>`
   missing으로 pr 단계 진입을 막는다. 파일이 없으면 아무것도 요구하지
   않는다 — 게이트를 만드는 순간부터 완료가 구조적으로 강제되는 opt-in.
   loopgate가 loop 상태를 readiness에 합성한 것과 같은 조립 구조다.
6. **크로스 케퍼빌리티 adapter edge는 만들지 않는다.** gates adapter의 policy
   실행기와 gatesgate의 게이트 조회는 함수 변수로 주입받고 composition
   root(harnessapp)만 배선한다(audit/loopgate 패턴). legacy adapter edge
   래칫("전환 완료, 신규 edge 금지")을 지킨다.

## Alternatives considered

- **unlazy 스킬을 있는 그대로 설치** — Node 의존, shell 실행, 하네스 정책
  경계 밖. 하네스 원칙(host adapter는 정책을 우회할 수 없다)과 충돌.
- **IssueOps 레코드에 게이트 필드 추가** — 게이트는 태스크 산출물이지
  IssueOps 스키마가 아니다. 파일 기반이어야 에이전트·사람·unlazy가 같은
  원본을 읽는다.
- **loop 확장** — loop는 "반복하며 검증"의 상태이지 "수용 기준 ledger"가
  아니다. 관심사가 다르고 이미 loop gate가 readiness에 있다.

## Consequences

- 게이트가 존재하는 IssueOps 사이클은 PR 전에 ledger가 완결돼야 한다
  (체크된 박스 + non-pending EVIDENCE, 또는 ABANDON).
- 서로 다른 IssueOps 사이클은 서로 다른 `.agent-harness/gates/issue-<number>.md`를 커밋하므로,
  병렬 worktree가 같은 root `GATES.md`를 add/add 또는 content conflict로
  충돌시키지 않는다.
- MCP tools/tools.golden/response_contracts.golden/omo 카탈로그 다이제스트가
  갱신된다(스키마 변경 절차 준수).
- unlazy 호환성은 형식 수준이다. unlazy의 Stop hook(강제 차단)은 채택하지
  않는다 — 하네스는 hook 표면을 이미 소유하고 있고 정책·감사 경로 없는
  차단은 도입하지 않는다.
