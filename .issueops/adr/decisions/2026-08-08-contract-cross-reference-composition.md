# 2026-08-08 — contract 사이 참조는 계약 조합이다

← [ADR index](../../ADR.md)

- Source: GitHub #234
- Decision: `contract_must_not_import_internal`은 contract가 **구현 계층**
  (`domain`·`application`·`adapter`·`cmd`·`port`)을 import하는 것만 막는다. contract
  package 사이 참조는 위반이 아니다.
- Rationale: DTO가 다른 capability의 DTO를 필드로 갖는 것은 계약 조합이지 구현 의존이
  아니다. 이 저장소는 이미 그 방향을 쓰고 있다 —
  `contract/issueopslease -> contract/state`,
  `contract/issueopscompletion -> contract/issueopslease`,
  `contract/issueopspreparation -> contract/issueopslease`. 그런데 이들은
  `isFoundationOwner` 화이트리스트 밖이라 검사되지 않았을 뿐이고, 목록 안의
  `contract/lifecycle`은 같은 종류의 참조가 막혔다. **같은 방향이 package에 따라
  허용되고 금지되는 것은 규칙이 의도한 바가 아니다.**
- Ownership: `evaluateOwnershipEdges`의 일반 contract rule만 바뀐다. vertical별
  `publication_contract_must_not_import_internal`,
  `leasevertical_contract_must_not_import_production_issueops`는 그대로 유지되므로
  IssueOps 수직 마이그레이션의 좁은 계약 경계는 영향을 받지 않는다.
- Consequences: `ProjectLifecycleStatePlan`처럼 다른 capability의 DTO를 품는 타입을
  contract로 올릴 길이 열린다. 그 타입들이 `lifecycle`·`doctor`·`projectbootstrap`의
  legacy edge를 막고 있었다.
- Rejected: DTO를 capability마다 중복 선언하는 안(같은 계약이 두 곳에서 갈라진다),
  `isFoundationOwner` 목록에서 `contract/lifecycle`을 빼는 안(그 package가 받아야 할
  다른 엄격한 규칙까지 함께 사라진다).
