# 2026-07-01 — IssueOps phase transition is a pure reducer over the record; impurity lives in wrappers

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: 12-factor-agents #12 (stateless reducer) 적용 조사 (research `.issueops/research/harness-engineering-12factor.md`), Brooks devil's-advocate 리뷰 후 doc-only로 축소
- Summary: IssueOps phase 전이의 판정은 이미 `IssueOpsRecord`만 읽는 순수 함수이고, 비결정·side-effect(clock/git/FS/session/disk)는 wrapper가 소유한다. 이 불변식을 CONVENTIONS "State machine reducer contract"로 명문화한다. 코드 변경은 없다.
- Context: 조사에서 12-factor #12(`(state,event)→state` 순수 변환으로 replay/복구 보장)를 적용 후보로 도출했다. 코드 검증 결과 readiness 게이트(`issueops_phase.go:46-112`)는 이미 record만 읽어 순수했고, 유일한 비결정 요소는 wall-clock(`:114`)·git/FS read(`:120-121`)·disk write(`:124`)·session unbind(`:31-32`)로 모두 wrapper에 국소화돼 있었다. ledger 결정성은 이미 `DeriveIssueOpsPhaseLedger` 테스트로 보장된다.
- Decision: (1) CONVENTIONS.md에 "State machine reducer contract" 섹션을 추가해 "판정은 record-순수, 비결정성은 wrapper 소유"를 규율로 고정한다. (2) 코드는 바꾸지 않는다 — 순수 함수 추출(`reducePhase`)은 **채택하지 않는다**.
- Consequences: 신규 상태머신은 판정 로직에 clock/IO를 섞지 않아야 한다. `AdvanceIssueOpsPhase`(`issueops`, `issueopscli`, `mcpcli`, `issueopsapp` golden, `lifecycle`, `hookcli`, `adapter/mcp` 파급의 최고-민감 함수, §28)는 동작 무변화 목적으로 건드리지 않는다. `.issueops/*.md` 편집이므로 `response_contracts.golden.json` docs_index를 재생성한다(§27).
- Alternatives / rejected options:
  - `reducePhase(record, to, now, head, fingerprint)` 순수 함수 추출 — rejected: Brooks 판정. clock은 phase 결정에 영향을 주지 않아 replay 테스트가 잡는 버그가 없고(ledger 결정성은 이미 테스트됨), 라인 100 절단은 앞쪽 6개 동등-순수 게이트(`:46-99`)를 wrapper에 남겨 개념적 무결성을 깬다. 최고-민감 함수를 zero-behavior-change로 건드리는 것은 negative-EV.
  - 문서 없이 코드만 정리 — rejected: 규율(신규 상태머신 가이드)이 핵심 가치이고 거의 무비용이다.
