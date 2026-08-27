# Gates: 480 이슈별 산출물을 .agent-harness/issues/<번호>/ 한 폴더로 모은다

- [x] G1: DiscoverGateFiles가 .agent-harness/issues/*/gates.md를 canonical 1순위로 읽고 옛 후보를 유지한다
  CHECK: go test ./internal/adapter/gates -run TestCheckDiscovers -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/gates	0.321s
- [x] G2: 현재 사이클의 linked issue 번호가 새/옛 게이트 경로에 동시에 있으면 duplicate_issue_artifact:<n>로 fail-closed, 번호 없으면 미판정
  CHECK: go test ./internal/adapter/issueops/gatesgate -run Duplicate -count=1
  EXPECT: ok
  EVIDENCE: ok  	agent-harness/internal/adapter/issueops/gatesgate	8.760s
- [x] G3: 스킬·문서·gitignore가 새 레이아웃을 한 번만 규정하고 검사기를 통과한다
  CHECK: python3 -c "import subprocess as S;r=[S.run(c,capture_output=True,text=True).returncode for c in (['python3','scripts/validate-skill.py','skills/issueops'],['python3','scripts/validate-skill.py','skills/von-neumann'],['python3','scripts/validate-skill.py','skills/fagan'],['python3','scripts/validate-skill.py','skills/review-agent-feedback'],['python3','scripts/verify-skill-shell.py'])];g=open('.gitignore').read();print('docs-ok' if not any(r) and 'issues/*/review/' in g and 'agent-harness/tmp/' in g else ('fail',r))"
  EXPECT: docs-ok
  EVIDENCE: docs-ok
- [x] G4: 번호 플랜 링크가 새 경로로 리라이트되어 옛 번호 플랜 경로 참조가 0건이다
  CHECK: python3 -c "import subprocess;out=subprocess.run(['rg','-l','-e','agent-harness/plans/[0-9]+-','-e','agent-harness/plans/issue-[0-9]+','--type','md','.'],capture_output=True,text=True).stdout;print('links-ok' if not out.strip() else out)"
  EXPECT: links-ok
  EVIDENCE: links-ok
- [x] G5: 이동 전후 새로 빌드한 바이너리의 gates check 게이트 ID·상태 집합이 같다 (수동: 이동 전 스냅샷과 이동 후 출력 비교)
  EVIDENCE: 2026-08-27 ./bin/agent-harness gates check --json 이동 전(.agent-harness/gates/477-*.md + issues/480/gates.md, 14 rows) vs 이동 후(issues/477/gates.md + issues/480/gates.md, 14 rows) — 경로 정규화 후 (file,id,state) 집합 동일, diff [] []
- [x] G6: 골든·전체 배터리·self-verify QA gate 통과
  CHECK: python3 -c "import subprocess as S;r=[S.run(c,capture_output=True,text=True).returncode for c in (['go','test','./cmd/harness/contractgolden','./cmd/harness/harnessapp','-run','Golden','-count=1'],['go','vet','./...'])];print('battery-ok' if not any(r) else ('fail',r))"
  EXPECT: battery-ok
  EVIDENCE: battery-ok; ./bin/agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false → 25/26 steps, QA gate 통과, 유일한 실패는 로컬 Omo MCP 설정 부재의 native integration(기존 lesson 2026-08-27과 동일); go test ./... -count=1 전부 ok
