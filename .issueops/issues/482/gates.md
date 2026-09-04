# Gates: 482 Orca 봉인 아티팩트 디렉터리를 레코드 필드로 영속해 이슈 폴더 아래로

- [x] G1: artifact_dir 필드가 봉인 경로를 결정하고 비어 있으면 legacy이며, 새 Workspace는 linked issue 번호로 채운다
  CHECK: go test ./internal/adapter/issueops -run SealedArtifactDir -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	0.299s
- [x] G2: materialize가 기록된 디렉터리에 0600으로 봉인하고 legacy 디렉터리를 만들지 않으며 재-materialize는 불변 계약대로 통과한다
  CHECK: go test ./internal/adapter/issueops -run MaterializeStagedArtifactsWritesInto -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	0.757s
- [x] G3: Orca prepare·owner packet이 이슈 폴더 아래에 봉인하고 PlanPath를 그 경로로 저장한다
  CHECK: go test ./internal/adapter/issueops -run MaterializesPlan -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	0.752s
- [x] G4: completion은 plan 부재를 MissingArtifacts로 남기고 렌더러가 한 줄을 출력한다
  CHECK: go test ./internal/adapter/issueops ./internal/adapter/provider/issuebody -run Missing -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	1.357s | ok  	issueops/internal/adapter/provider/issuebody	0.393s
- [x] G5: 상수 직접 사용이 남지 않는다
  CHECK: python3 -c "import subprocess;out=subprocess.run(['rg','-n','FromSlash(IssueOpsArtifactDir)','--glob','!*_test.go','internal','cmd'],capture_output=True,text=True).stdout;print('no-direct-const' if not out.strip() else out)"
  EXPECT: no-direct-const
  EVIDENCE: no-direct-const
- [x] G6: gitignore·CONVENTIONS 갱신과 문서 검사기 통과 (수동)
  EVIDENCE: 2026-08-27 .gitignore에 .issueops/issues/*/artifact/ 추가, CONVENTIONS 레이아웃 표에 artifact/ 행과 artifact_dir 규칙; uv run … scripts.check --mode check → ok true
- [x] G7: 전체 배터리와 골든(workspace artifact_dir) 통과 (수동)
  EVIDENCE: 2026-08-27 gofmt -l 0; go vet ./... ok; go test ./... -count=1 전부 ok(lease stable v1 shape·reconcile vertical·orca intent repository 포함); contractgolden/issueopsapp Golden 통과(골든 fixture는 artifact_dir 미설정이라 무변경)
