# #250 Orca resume contract plan

1. RED: actor-free status resume, complete explicit actor, partial actor, hook near-miss, owner-prompt distinction을 테스트한다.
2. GREEN: CLI가 actor flags 부재 시 native session·host process ancestry·cwd를 관측하고, hook은 exact actor-free resume만 typed control-plane으로 허용한다.
3. 계약 정렬: CLI usage, canonical catalog, golden, owner prompt, Karpathy parity 문서, IssueOps 운영 문서를 같은 계약으로 갱신한다.
4. 검증: focused suites, contract/response goldens, full/race/vet/build, exact status→resume 실동작, 독립 구현 리뷰를 통과한다.

Rollback은 이 child merge revert이며 IssueOps state bytes나 Orca runtime을 하향 변환하지 않는다.
