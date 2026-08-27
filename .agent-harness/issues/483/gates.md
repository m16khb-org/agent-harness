# Gates: 483 gates_incomplete 판정을 현재 사이클 이슈 원장으로 한정

- [x] G1: scopeLedgers가 자기 번호 폴더·동번호 legacy·익명 원장만 판정하고 021/210/다른 번호는 제외한다
  CHECK: go test ./internal/adapter/issueops/gatesgate -run TestScopeLedgers -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops/gatesgate	0.284s
- [x] G2: 다른 이슈의 미완 원장은 readiness를 막지 않고 gates_skipped 집계 한 줄로 남으며, 자기·익명 미완 원장은 계속 막고, 번호 없는 레코드는 전부 판정한다
  CHECK: go test ./internal/adapter/issueops/gatesgate -run StrictPRReadiness -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops/gatesgate	14.336s
- [x] G3: DiscoverGateFiles와 gates check CLI는 불변이다
  CHECK: go test ./internal/adapter/gates -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	1.308s
- [x] G4: issueops 스킬과 CONVENTIONS에 규칙이 기록되고 검사기를 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
- [x] G5: 전체 배터리(go test ./..., vet, gofmt, 골든) 통과 (수동)
  EVIDENCE: 2026-08-27 gofmt -l 0; go vet ./... ok; go test ./... -count=1 전부 ok; contractgolden/harnessapp Golden 통과(골든 무변경)
