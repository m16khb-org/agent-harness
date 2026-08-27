# Gates: 490 재타깃된 stacked PR 정리 dead-end 해소

- [x] G1: 준비 base가 원격에서 사라졌고 관측 base가 기본 브랜치면 통과하고, 잔존·비기본 브랜치·관측 실패는 각각 거부된다
  CHECK: go test ./internal/adapter/issueops -run MergedBase -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	0.431s
- [x] G2: 기존 cleanup finish 게이트와 fingerprint 규율이 그대로 통과한다
  CHECK: go test ./internal/adapter/issueops -run CleanupFinish -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops	2.742s
- [x] G3: cleanup CLI 주입과 상태 투영이 그대로다
  CHECK: go test ./cmd/harness/issueopscli/feedbackcleanup -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/cmd/harness/issueopscli/feedbackcleanup	2.276s
- [x] G4: issueops-cleanup 스킬 문서가 재타깃 판정을 기록하고 검사기를 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops-cleanup
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
- [x] G5: 실환경 — 고친 바이너리로 io-71af6dd82f0d preview 통과 후 apply로 워크트리·브랜치·레코드 삭제 (수동)
  EVIDENCE: 2026-08-27 ./bin/agent-harness issueops cleanup finish --id io-71af6dd82f0d --preview → missing 없음, retargeted_base {prepared_base: 484-gates-check-shell-chain, observed_base: main, default_branch: main, prepared_base_remote_absent: true}; apply → record_deleted/worktree_removed/branch_deleted 모두 true, git worktree list에 486 경로 없음, show-ref exit 1
- [x] G6: 전체 배터리·골든 통과 (수동)
  EVIDENCE: 2026-08-27 gofmt -l 0; go vet ./... ok; go test ./... -count=1 전부 ok; contractgolden/harnessapp Golden 통과(계약은 결과 필드 추가뿐이라 골든 무변경); 문서 검사기 ok
