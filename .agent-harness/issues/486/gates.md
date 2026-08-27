# Gates: 486 gates check는 EXPECT가 있어도 exit 0을 요구한다

- [x] G1: 출력이 EXPECT와 일치해도 exit≠0이면 unmet이고 check_error에 exit code가 남는다
  CHECK: go test ./internal/adapter/gates -run TestCheckRequiresExitZero -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	0.243s
- [x] G2: 기존 EXPECT·타임아웃·정책 거부 테스트가 그대로 통과한다
  CHECK: go test ./internal/adapter/gates ./internal/adapter/issueops/gatesgate -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	1.324s | ok  	agent-harness/internal/adapter/issueops/gatesgate	12.917s
- [x] G3: 실환경(clean tree 선행) — 임시 원장 exit 1 + EXPECT ok → unmet; 저장소 원장 477(8/8)·480(6/6)·484(6/6) met 수 불변 (수동)
  EVIDENCE: 2026-08-27 워크트리(추적 변경은 이 이슈의 4파일뿐) ./bin/agent-harness gates check: 임시 원장 G1 python3 -c "print('ok'); raise SystemExit(1)" → unchecked 'exit code 1: ok', G2 exit 0 → met; 저장소 원장 477 8/8, 480 6/6, 484 6/6 complete
- [x] G4: issueops 스킬과 CAUTIONS에 exit 0 규칙 기록
  CHECK: python3 scripts/validate-skill.py skills/issueops
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
- [x] G5: 전체 배터리·골든 통과 (수동)
  EVIDENCE: 2026-08-27 gofmt -l 0; go vet ./... ok; go test ./... -count=1 전부 ok; contractgolden/harnessapp Golden 통과(무변경)
