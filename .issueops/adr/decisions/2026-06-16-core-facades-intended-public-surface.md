# 2026-06-16 — internal/core *_facade.go는 의도된 공개 표면으로 유지

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: issueops optimization P5

- Summary: 최적화 P5에서 facade의 순수 위임을 직접 import로 되돌리는 대신, facade를 의도된 안정적 공개 표면으로 유지하고 경계 규칙을 문서화하기로 결정했다.
- Context: 최적화 계획 P5는 internal/core/*_facade.go의 pure passthrough alias/one-line delegate를 owning package 직접 import로 되돌려 facade 표면을 축소하라고 제안했다. cmd/는 core를 단일 표면으로 import하고 core 내부 subpackage(core/issueops 등)는 직접 import하지 않는다.
- Decision: facade(issueops/workflow/utility/policy/project_doc/state_trace/draft_wiki/issueops_remote)를 의도된 공개 표면으로 유지한다. 허용 내용은 type alias 재노출, 타입 변환, 다중 subpackage 조합, boundary enforcement이며, 순수 1-line 위임도 표면 안정성/디커플링을 위해 허용한다. 규칙은 internal/core/doc.go에 codify했다.
- Consequences: facade에 새 도메인 로직 추가 금지(조합/변환/enforcement만). facade 함수가 그 이상으로 커지면 owning subpackage로 로직 이동. 향후 facade drift 방지를 위해 doc.go 규칙을 따른다.
- Evidence:
  - 133개 facade 심볼 전수조사 결과 미사용(dead) export 0개
  - utility_facade.SummarizeHookFailureStats는 hookfailure+hookmetrics를 조합하는 실제 boundary 로직
  - workflow_facade.RunReadOnlyWorkerJob/projectProfilesToLifecycle는 타입 변환 boundary
  - go build/vet/test ./internal/core 통과
- Alternatives / rejected options:
  - P5 원안대로 pure passthrough를 cmd 직접 subpackage import로 되돌리기 — cmd가 core 내부 구조에 결합되고, dead export가 0이라 표면 축소 이득도 없어 거부
  - facade 내부 unexported 위임(containsAny 등) 제거 — 내부 indirection만 줄고 core 내부 caller churn 발생, 한계 가치라 보류
