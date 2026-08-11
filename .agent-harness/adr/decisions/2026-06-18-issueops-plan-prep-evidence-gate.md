# 2026-06-18 — IssueOps plan-prep evidence gate

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: agent-harness issueops cycle (branch 12-issueops-planprep-evidence-gate)
- Summary: `plan` phase 진입에 결정사항 조회·유사이슈 scoring·웹서치 근거를 fail-closed 게이트로 강제하되, intent class가 trivial이면 스킵하고 각 항목은 사유로 면제(waive)할 수 있게 했다.
- Context: 기존 `IssueOpsPlanReadiness`는 intent contract + issue_url만 검사해, 이슈 생성 이전 정보수집(ADR/decision 조회, related-issue scoring, berners-lee 웹서치)이 권고에만 머물고 강제되지 않았다.
- Decision: record에 `IssueOpsPlanPrep` sub-record(prior_decisions/related_issues/web_research; 각 evidence|waived)를 추가하고, intent contract에 `IntentClass`를 추가했다. `IssueOpsPlanReadiness`가 non-trivial class에서 세 항목을 검사한다(missing 키 `plan_prep_decisions`/`plan_prep_related_issues`/`plan_prep_web_research`). 기록은 `issueops plan-prep record` CLI/MCP(`issueops_plan_prep_record`)가 담당하고, intent class는 `intent record --intent-class`로 명시한다. 미기록 class는 `standard`로 정규화해 기본 강제한다.
- Consequences: non-trivial 사이클은 plan 진입 전 세 증거(또는 면제 사유)를 남겨야 한다. design review는 plan-prep을 전제하지 않는다(plan phase 내부 활동이라 plan 진입 시 이미 강제됨) — `RecordDesignReview`는 `plan_prep_*` missing을 무시한다. CLI/MCP/contract/SKILL.md/issue-preflight를 같은 변경에서 갱신했다.
- Alternatives / rejected options:
  - intent contract에 3개 필드 직접 추가 — '의도 계약'과 '증거 수집'이 한 곳에 뒤섞여 거부하고 별도 PlanPrep sub-record 채택
  - 무조건 강제(waive 불가) — trivial/순수 내부 작업까지 막혀 마찰이 커 거부하고 사유 기반 면제 채택
  - design review까지 plan-prep 강제 — 수많은 design 검증 테스트가 깨지고 실제 흐름상 이중 검사라 거부하고 plan-phase 진입에만 강제
