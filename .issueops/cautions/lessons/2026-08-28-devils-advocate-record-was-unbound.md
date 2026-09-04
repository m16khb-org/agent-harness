---
name: cautions/lessons/2026-08-28-devils-advocate-record-was-unbound.md
description: Dated lesson — devil's-advocate 판정 기록이 플랜 버전·리뷰어 출처·이력을 담지 않아 게이트가 실제 검토 없이도 통과했고, 하네스 데이터로는 design-review의 효과를 측정할 수 없었다.
---

# 2026-08-28 — devil's-advocate 기록이 플랜에 묶이지 않아 게이트가 연극 가능했다

Family index: [CAUTIONS.md](../../CAUTIONS.md).


- Kind: `caution`
- Source: design-review 실효성 감사, Claude Code session 2026-08-28 (트랜스크립트 ~20 사이클·24 라운드 실측)
- Summary: `devils_advocate_review` 게이트는 "기록이 존재하고 stop/revise가 waive됐다"만 검증했다. 기록에 검토한 플랜의 digest·리뷰어 출처·이전 라운드가 없어서, 판정 뒤 플랜을 고치고 `--waive "전부 반영"`으로 닫거나(10/21건), 형제 사이클의 판정을 복사하거나(io-0c35f69e00b4), 인라인으로 수행한 검토를 pass로 기록해도(2건) 게이트를 통과했다. DB는 11/11 pass였지만 트랜스크립트에는 revise 21건·stop 1건이 있었다.
- Context: 감사는 "design-review가 실제로 유효한가"를 물었다. 판정 자체는 유효했다 — 1차 revise율 약 83%, 지적은 file:line 근거가 있는 실결함(soft-delete 부활, fail-open 과금 경로, redaction 계약 역전), 저자 반박 0건. 유효하지 않은 것은 **기록**이었다. 하네스 자체 데이터로는 이 질문에 답할 수 없었고, 트랜스크립트를 직접 읽어야 했다.
- Resolution: ADR [2026-08-28 devil's-advocate plan binding](../../adr/decisions/2026-08-28-issueops-devils-advocate-plan-binding.md). 기록에 `reviewed_plan_digest`·`reviewer_context`·`history`를 추가하고, implement 진입과 첫 owner 준비에서 digest 불일치를 `devils_advocate_review_stale`로 거부한다. `pass`도 finding 1건 이상을 요구한다. 스킬은 3라운드부터 delta 검토, 동기 반환, `--waive`는 override 전용으로 규정한다.
- Evidence:
  - `~/.local/state/issueops/issueops_v1/harness.db` `records` 테이블: DA 기록 11건 전부 `pass`, regress 1회. 트랜스크립트의 `devils-advocate review` 명령 32건: revise 21(waive 10)·pass 10·stop 1.
  - 같은 분에 찍힌 생성·design review·DA 기록: io-8b92f27d9297("Brooks final Verdict proceed; only canonical worktree path changed"), io-0c35f69e00b4(io-ebd9091d293c와 finding 9건 동일, 3분 차이).
  - 인라인 실행 기록: io-ebd9091d293c finding #9 "사용자 세션 지시로 서브에이전트를 사용하지 않아 design-review를 인라인으로 수행했다", 세션 706b1fc6 waiver rationale.
  - 통제 실험(2026-08-28): 같은 플랜의 정상본(GREEN)과 결함 삽입본(RED: 장식용 digest, 기본값 subagent, 리뷰어 레지스트리, 3병렬 1시간)을 독립 design-review 2개에 넣었다. RED는 심은 결함 4건을 전부 지적했고, GREEN은 저자가 놓친 실결함(lease `Decode`의 `DisallowUnknownFields`로 새 필드가 `invalid state`가 됨, delegation 자식 영구 차단, ai-slop-clean 재사용)을 잡아 `revise`를 냈다. 2라운드 delta 검토는 10건 해소를 확인하고 새 차단 결함 1건(implement 중 owner 교체 경로까지 게이트가 번짐)을 잡았다. 라운드당 3–10분, 96k–108k 출력 토큰.
  - `internal/adapter/issueops/devilsadvocate/devils_advocate.go`, `internal/adapter/issueops/issueops_readiness.go`, `internal/adapter/issueops/issueops_artifact_stage.go`, `internal/contract/issueopslease/stable_v1.go`
- Rule: 리뷰 판정을 게이트로 쓰려면 기록이 **무엇을 검토했는지**(대상의 content digest), **누가 어떻게**(출처), **이전에 무엇을 냈는지**(이력)를 담아야 한다. 존재 여부만 검증하는 게이트는 복사·인라인·자기증명으로 통과되며, 그 게이트의 효과는 하네스 데이터로 측정할 수 없게 된다. `ImplementationReview.reviewed_fingerprint`와 같은 stale 거부 선례를 재사용하고, 새 필드는 lease 미러(`stable_v1.go`)와 같은 커밋에서 갱신한다. Evergreen 규칙: [issueops-lifecycle.md](../issueops-lifecycle.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
