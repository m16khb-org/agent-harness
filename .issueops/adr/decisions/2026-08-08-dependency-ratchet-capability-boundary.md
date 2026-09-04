# 2026-08-08 — Dependency ratchet은 capability 경계만 센다

← [ADR index](../../ADR.md)

- Source: GitHub #234
- Decision: `legacyEdges`는 concrete-adapter edge 중 **capability 경계를 넘는
  것만** legacy로 센다. 같은 capability의 adapter package 사이 edge는 baseline에
  올리지 않는다. capability는 `internal/adapter/` 다음 경로 요소이며,
  `outbound`/`inbound`는 capability가 아니라 방향 분류이므로 그 다음 요소까지
  읽는다(`outbound/state`와 `outbound/sqlstore`는 서로 다른 capability다).
- Rationale: 하나의 adapter를 하위 package로 나누는 것은 계층 위반이 아니라 구현
  정리다. 이전 규칙은 `internal/adapter/issueops -> internal/adapter/issueops/linking`
  같은 내부 구조까지 adapter 간 결합으로 세어, package를 잘게 나눌수록 baseline이
  늘어나는 역유인을 만들었다. 52개 file을 한 package에 두면 ratchet이 조용해지고
  나누면 벌점을 받는 것은 ratchet이 측정하려던 바가 아니다.
- Ownership: 판정은 `isSameCapabilityAdapter`가 소유하고 `legacyEdges`만 사용한다.
  `evaluateEdges`의 forbidden rule은 바뀌지 않으므로
  `inbound_adapter_must_not_import_outbound_adapter`,
  `core_must_not_import_adapter_or_cmd`, `adapter_must_not_import_cmd`는 그대로
  즉시 실패한다.
- Consequences: baseline 226 → 181. 남은 181개는 cmd → adapter 116개와 capability를
  넘는 adapter 간 65개이며, 전자는 composition root 이동(#233), 후자는 port 역전으로
  해소한다. capability 내부 결합은 ratchet이 아니라 code review가 다룬다.
- Rejected: baseline에 45개를 그대로 두는 안(측정 대상이 아닌 것을 남겨 zero-baseline
  목표가 package 병합을 유도한다), capability 예외를 `evaluateEdges`까지 확장하는
  안(inbound가 같은 capability의 outbound를 직접 부르는 것을 허용하게 된다).
