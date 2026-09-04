# 2026-07-14 — Evidence-first cross-host tool contract hardening

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: `.issueops/issues/_unnumbered/cross-host-tool-contract-conformance.md`
- Summary: Codex, Claude Code, GJC의 tool-call drift는 capture-only benchmark로 먼저 측정하고 재현된 경우에만 하나의 production MCP argument contract를 강화한다.
- Decision:
  - representative schema 3개와 preregistered 10-case deterministic baseline을 유지한다. Live P0는 host별 임시 config/plugin과 one-tool probe를 사용하며 production handler를 호출하지 않는다.
  - advertised schema validity와 closed canonical-intent validity를 별도 보고한다. `failure_class`는 occurrence pattern으로 유지하고 causal ownership은 typed evidence 기반 `failure_cause`로 분리한다.
  - 동일 host/schema/normalized diagnostic signature가 최소 두 번 재현된 `authorize_hardening` gate에서만 advertised schema closure, SDK/legacy validator, strict typed arg access를 한 rollback unit으로 적용한다. Per-host production semantics와 silent repair는 두지 않는다.
  - behavioral regression fixture에는 redacted arguments, schema/signature hash, handler call count, final result/state digest만 저장한다. transcript, chain-of-thought, credential, absolute home path는 저장하지 않는다.
  - 조직 adoption scorecard는 두 번째 human operator opt-in, data scope/retention 승인, review rework·incident·completed-task quality outcome proxy 합의, host cost source와 baseline 확보가 모두 끝날 때까지 구현하지 않는다.
- Consequences: 기본 self-verify는 deterministic baseline만 실행한다. Live initial/reproduction/context-pressure/post-enforcement batch는 각각 별도 외부 비용 경계이며, one-off observation이나 환경 실패로 production 계약을 바꾸지 않는다. Scheduler, RL trainer, 조직 dashboard는 이 결정의 범위가 아니다.
- Verification: `issueops contract conformance baseline --json`, capture-only MCP round-trip tests, three-host fake-runner isolation tests, self-verify failure-cause/trace/history compatibility tests, contract response goldens.
