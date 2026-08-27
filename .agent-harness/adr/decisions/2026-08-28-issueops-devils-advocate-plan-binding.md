# 2026-08-28 — IssueOps devil's-advocate verdicts are bound to the reviewed plan digest

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: brooks 실효성 감사(2026-08-28, 트랜스크립트 ~20 사이클·24 라운드 실측) → plan `.agent-harness/plans/brooks-review-binding.md`, brooks 3라운드 검토
- Summary: `IssueOpsDevilsAdvocateReview`는 검토한 플랜의 sha256(`reviewed_plan_digest`), 리뷰어 출처(`reviewer_context`), 같은 plan phase의 이전 라운드(`history`)를 담는다. implement 진입과 첫 owner 준비 preflight는 digest가 현재 플랜과 다르면 `devils_advocate_review_stale`로 fail-closed한다. `pass`도 finding 1건 이상을 요구한다.
- Context: 하네스 DB의 devil's-advocate 기록은 11/11 `pass`였지만 트랜스크립트에는 revise 21건·stop 1건이 있었다. 원인은 기록 계약이었다 — (1) 판정이 플랜 버전에 묶이지 않아 revise 뒤 플랜을 고치고 `--waive "전부 반영"`으로 닫은 사례가 10/21건이고, 사이클 생성·design review·DA 기록이 같은 분에 찍힌 복사 판정(io-8b92f27d9297, io-0c35f69e00b4)이 게이트를 통과했다. (2) 인라인 실행과 서브에이전트 실행을 기록에서 구분할 수 없었다(인라인 pass 2건). (3) `Record`가 덮어써 revise 라운드가 사라졌다. 즉 게이트는 "기록이 존재한다"만 검증했고, brooks가 무엇을 잡았는지 하네스 데이터로 답할 수 없었다.
- Decision:
  1. 계약 — `reviewer_context`(subagent|inline, 기록 시 필수, **감사 필드**: `ImplementationReview.reviewer_*`와 같은 원칙으로 게이트 조건이 아니다), `reviewed_plan_digest`, `history[]`. lease 미러(`issueopslease/stable_v1.go`)를 같은 커밋에서 갱신한다 — `Decode`는 `DisallowUnknownFields`라 미러가 빠지면 execution 명령 전체가 `invalid state`가 된다.
  2. 기록 — digest는 링크된 플랜 파일(`readLinkedPlanIdentity`, owner preflight와 같은 resolver) 우선, 파일이 없으면 staged plan artifact. 둘 다 없으면 기록을 거부한다. 이전 라운드는 `history`에 append한다(regress가 review를 지우면 함께 사라진다 — stop 라운드는 regress 선행조건인 원격 이슈 반영과 `Decisions`에 남는다).
  3. 게이트 — implement 진입(`IssueOpsImplementationReadiness`)과 첫 owner 준비(`RequireStagedExecutionOwnerPlan`, phase가 implement 이전일 때만)에서 digest 불일치·부재를 `devils_advocate_review_stale`로 거부한다. planner 정적 게이트는 digest 존재만 본다. `IssueOpsAISlopCleanReadiness`와 implement 이후 owner 교체(replacement/reseed)는 plan binding을 보지 않는다 — 강제 범위는 "implement 진입 전까지"다. delegation이 합성한 자식 판정(`reviewer_pattern: delegated-parent-review`)은 부모 판정을 상속하므로 면제한다.
  4. `pass`는 finding ≥ 1(무엇을 공격했고 왜 실패했는지). CLI는 `--reviewer-context`가 필수다.
  5. 운용 규칙(스킬) — 1·2라운드 전체 검토, 3라운드부터 delta 검토; 판정은 동기 반환; `--waive`는 override의 의미이며 "반영했다"의 의미로 쓰지 않는다.
- Consequences: 기존 레코드는 digest가 없어 implement 진입에서 stale로 막힌다(재기록 필요, 자동 승격 없음). revise 뒤 waive로 닫던 경로 대신 최종 플랜 delta 재검토(관측 2–4분)가 사이클당 1회 늘어난다. `response_contracts.golden.json`·`usage.golden.txt` 갱신. 정직한 한계: digest 바인딩은 "최종 플랜에 새 기록"을 강제할 뿐 "독립 검토였다"를 강제하지 못한다 — 저자가 최종본에 `--verdict revise --waive`를 다시 치면 통과하며, 이 변경은 그 우회를 감사 가능한 명시적 기록(digest 일치 + waived + rationale + reviewer_context)으로 만든다.
- Alternatives / rejected options:
  - `reviewer_context`를 게이트 조건으로(inline은 waiver 필수) — 기각: 자기신고를 게이트하지 않는 기존 원칙(`types.go` ImplementationReview 주석)과 충돌하고, 플래그만 바꾸면 우회되므로 분기만 늘린다.
  - `revise` 전용 `resolved` 상태 — 기각: digest 바인딩이 같은 목적을 상태 추가 없이 달성한다.
  - 리뷰어 레지스트리/backend, `devils-advocate list` 명령, `reviewer_ref`, `reviewed_plan_path`, `round` 정수 — 기각: 호출자 없는 일반화이거나 기존 필드와 중복(`PlanPath`는 링크 후 교체 불가, round는 `len(history)+1`).
  - regress 시 review를 `RegressEvent`에 복사 — 보류: stop 라운드는 원격 이슈 섹션과 `Decisions`에 이미 남는다. revise 이력만으로 감사 목적이 충족되는지 실사용 뒤 판단한다.
