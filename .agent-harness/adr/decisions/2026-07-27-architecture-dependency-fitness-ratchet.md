# 2026-07-27 — Architecture dependency fitness ratchet

← [ADR index](../../ADR.md)

- Kind: `adr`
- Decision: `internal/architecture`에서 direct production import edge를 결정적으로 수집하고, unconditional layer rule은 즉시 차단하며 기존 infrastructure·adapter coupling은 정렬된 baseline으로만 허용한다.
- Rationale: 기존 import graph를 한 번에 이동하지 않고도 새 boundary regression과 baseline stale entry를 정확한 `importer -> imported` 진단으로 막는다.
- Rejected: 전체 transitive dependency graph 비교는 구현 세부사항 변화에 과민하고, lint rule만 사용하는 방식은 baseline new/stale edge의 reviewable contract를 제공하지 못한다.
- Consequences: legacy edge를 제거한 변경은 같은 review에서 baseline도 줄여야 하며, production runtime·CLI·MCP 계약은 이 test-only ratchet의 범위 밖이다.
