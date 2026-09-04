# 2026-08-08 — port는 계약 어휘로 말한다

← [ADR index](../../ADR.md)

- Source: GitHub #234
- Decision: `port_must_not_import_internal`은 port가 **구현 계층**
  (`domain`·`application`·`adapter`·`cmd`)을 import하는 것만 막는다. port가
  contract를 참조하거나 port 사이를 참조하는 것은 위반이 아니다.
- Rationale: port는 인터페이스 계층이고, 인터페이스는 무엇을 주고받는지 말해야
  한다. 그 어휘가 계약 DTO다. 두 규칙(`contract`는 port를 못 봄, `port`는
  contract를 못 봄)이 함께 서면 **DTO가 어느 계층에도 속할 수 없는 사각지대**가
  생긴다. 실제로 `ExecutionActionRequest`가 `*port.ExecutionIssueSnapshotEvidence`
  를 필드로 갖는 바람에 contract로도 port로도 갈 수 없었고, 그 하나 때문에
  `issueopscli`·`executioncmd`·`mcpcli`·`issueopslease` 네 곳의 어댑터 의존이
  풀리지 않았다. 방향을 하나 열어야 하는데, contract가 port를 보는 것은 DTO가
  인터페이스를 물게 되므로 틀렸고, port가 contract를 보는 것이 헥사고날의 정의에
  맞는다.
- Consequence: port의 순수 DTO는 contract로 내려가고 port에는 인터페이스와
  그 별칭만 남는다. `TestOwnershipManifestStillRejectsPortToImplementation`이
  port -> domain/application/adapter 방향이 여전히 막히는지 고정한다.
