# Gates: 484 gates check 셸 문법 체인 거짓 met 차단

- [x] G1: 따옴표 밖 셸 문법이 든 CHECK는 실행 없이 unchecked이고 gates init도 거부한다
  CHECK: go test ./internal/adapter/gates -run ShellSyntax -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	0.391s
- [x] G2: 따옴표 안 셸 문자는 argv 텍스트로 실행된다
  CHECK: go test ./internal/adapter/gates -run QuotedShell -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	0.220s
- [x] G3: 리터럴 EXPECT는 줄 앵커로 판정되어 에러 줄의 우연한 토큰이 통과하지 않는다
  CHECK: go test ./internal/domain/gates -run ExpectMatches -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/domain/gates	0.216s
- [x] G4: issueops 스킬과 CAUTIONS에 규칙이 기록되고 검사기를 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
- [x] G5: 실환경 — 임시 원장의 체인 CHECK와 docs-ok 에러 출력이 unchecked이고 저장소 원장 issues/477(8 met)·issues/480(6 met, python -c 형태 포함) 판정이 불변이다 (수동)
  EVIDENCE: 2026-08-27 ./bin/agent-harness gates check 임시 원장: G1(&& 체인) unchecked 'CHECK contains shell syntax…', G2(docs-ok: error 출력) unchecked 'expect not matched', G3(python3 -c "import sys; print('links-ok')") met; 저장소 원장 issues/477 8/8, issues/480 6/6 complete
- [x] G6: 전체 배터리(go test ./..., vet, gofmt, 골든) 통과 (수동)
  EVIDENCE: 2026-08-27 gofmt -l 0; go vet ./... ok; go test ./... -count=1 전부 ok; go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden 통과(골든 무변경)
