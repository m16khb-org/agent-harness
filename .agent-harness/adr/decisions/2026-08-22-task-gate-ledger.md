# Task gate ledger (unlazy-compatible gates)

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-22
- Status: accepted; path model amended 2026-08-27 for per-issue artifact folders

## Context

하네스의 기존 검증 표면(looprun, self-verify, IssueOps readiness)은 모두
**하네스가 소유한 상태**를 게이트한다. 그러나 에이전트가 태스크를
"다 했다"고 선언할 때 그 진위를 판정하는 표면은 없었다:

- loop는 반복 시도(verdict/evidence)를 기록하지만 수용 기준은 에이전트
  머릿속에 있다.
- IssueOps readiness는 아티팩트·리뷰·git 상태를 검사하지만 "이 사이클이
  실제로 하기로 한 결과물"과 대조하지 않는다.
- self-verify는 하네스 자체의 품질을 게이트한다(95점 루프).

unlazy v2(Leonxlnx/unlazy, MIT)는 이 빈칸을 "게이트 ledger"로 채운다. 작업
시작 전에 수용 기준을 gate ledger 파일로 쓰고, CHECK 명령 결과로만
체크박스를 채우며, EVIDENCE가 pending인 체크는 미충족으로 판정한다.
"prose는 prose를 강제할 수 없다. 파일과 명령이 강제한다"는 검증된 통찰은
agent-harness의 검증 우선 원칙과 같다.

## Decision

1. **unlazy 게이트 파일 형식을 그대로 채택한다.** `- [ ] G1: outcome` +
   `CHECK:`/`EXPECT:`/`EVIDENCE:` + `ABANDON:` 라인. 파서/판정기
   (`internal/domain/gates`)는 unlazy `gate-check.mjs`의 규칙을 준수한다:
   체크박스는 주장이고 EVIDENCE는 증명이다. checked+pending은 unchecked보다
   심각한 미충족이다. 기존 unlazy 스킬 사용자의 GATES.md가 하네스에서 그대로
   동작한다. 신규 generic `gates init`의 기본 경로는 scope에서 유도한
   `.agent-harness/gates/<scope-slug>.md`다. IssueOps는 병렬 worktree merge
   충돌을 피하고 artifact ownership을 분리하기 위해
   `.agent-harness/issues/<provider-issue-number>/gates.md`를 명시한다. 기존
   `.agent-harness/gates/issue-<n>.md`, root `GATES.md`, `gates/*.md`는 읽기
   호환 경로로 남는다.
2. **CHECK 명령은 raw shell이 아니라 command policy 경로로 실행한다.**
   unlazy의 `spawnSync(cmd, {shell: true})`와 달리, 하네스 게이트는
   `shelltoken`으로 argv를 토큰화한 뒤 `policy.EvaluateCommandPolicy` +
   `policy.RunCommand`(workspace 경계, env allowlist, secret redaction,
   timeout, audit log, shell interpreter 금지)를 통과해서만 실행한다.
   게이트를 채우기 위해 명령을 실행하더라도 정책 우회는 사후 승인하지
   않는다.
3. **파일 형식은 보존 파싱으로 다룬다.** 파서는 원본 라인 배열을 유지하고
   게이트 라인/EVIDENCE 라인만 교체하므로 CRLF, 산문, 사용자 주석이
   round-trip에서 소실되지 않는다(도메인 테스트로 고정).
4. **CLI와 MCP가 같은 contract DTO를 공유한다.** `agent-harness gates
   init|check|status|report|abandon`과 MCP `gates_init|gates_check|
   gates_status|gates_report|gates_abandon`는 같은 `internal/contract/gates`
   타입을 쓴다(스키마 버전 1). exit code는 unlazy와 호환된다: 0 전부 충족,
   1 미충족 잔존, 2 사용법 오류.
5. **IssueOps PR readiness에 게이트를 합성한다(gatesgate).** worktree에서
   canonical `.agent-harness/issues/<n>/gates.md`와 호환 gate ledger를 찾고,
   linked issue 번호가 있으면 해당 번호와 anonymous ledger만 판정한다. 같은
   번호의 canonical·legacy ledger가 중복되면 fail-closed하며, 미충족 게이트는
   `gates_incomplete:<file>`로 pr 진입을 막는다. 파일이 없으면 요구하지 않는다.
   게이트를 만드는 순간부터 완료를 구조적으로 강제하는 opt-in이다. loopgate가
   loop 상태를 readiness에 합성하는 것과 같은 구조다.
6. **크로스 capability adapter edge는 만들지 않는다.** gates adapter의 policy
   실행기와 gatesgate의 게이트 조회는 함수 변수로 주입하고 composition
   root(harnessapp)에서만 배선한다(audit/loopgate 패턴). legacy adapter edge
   래칫("전환 완료, 신규 edge 금지")을 지킨다.

## Alternatives considered

- **unlazy 스킬을 그대로 설치** — Node 의존, shell 실행, 하네스 정책
  경계 밖. 하네스 원칙(host adapter는 정책을 우회할 수 없다)과 충돌.
- **IssueOps 레코드에 게이트 필드 추가** — 게이트는 태스크 산출물이지
  IssueOps 스키마가 아니다. 파일 기반이어야 에이전트·사람·unlazy가 같은
  원본을 읽는다.
- **loop 확장** — loop는 "반복하며 검증"의 상태이지 "수용 기준 ledger"가
  아니다. 관심사가 다르고 이미 loop gate가 readiness에 있다.

## Consequences

- 게이트가 존재하는 IssueOps 사이클은 PR 전에 ledger가 완결돼야 한다
  (체크된 박스 + non-pending EVIDENCE, 또는 ABANDON).
- 서로 다른 IssueOps 사이클은 서로 다른 `.agent-harness/issues/<number>/gates.md`를 커밋하므로,
  병렬 worktree가 같은 root `GATES.md`를 add/add 또는 content conflict로
  충돌시키지 않는다.
- MCP tools/tools.golden/response_contracts.golden/omo 카탈로그 다이제스트가
  갱신된다(스키마 변경 절차 준수).
- unlazy 호환성은 형식 수준이다. unlazy의 Stop hook(강제 차단)은 채택하지
  않는다. 하네스가 hook 표면을 이미 소유하고 있으므로 정책·감사 경로 없는
  차단은 도입하지 않는다.
