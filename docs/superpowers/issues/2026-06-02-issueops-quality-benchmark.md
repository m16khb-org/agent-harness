# IssueOps 품질 벤치마크와 워크플로 게이트

## 문제

IssueOps는 에이전트 작업의 속도보다 품질을 높이는 것이 목적이다. 하지만 현재 하네스는 IssueOps가 사용자의 의도를 더 정확히 파악했는지, 이슈를 더 잘 작성했는지, 계획과 태스크를 더 안전하게 설계했는지, TDD와 subagent 분배를 더 잘 수행했는지, PR/MR 초안을 더 리뷰 가능하게 만들었는지 정량적으로 증명할 방법이 없다.

최근 요구사항으로 다음 워크플로 안전 제약도 추가되었다.

- 각 단계가 끝나면 다음 단계 선택지를 제시하고, 사용자가 진행/수정/단계 이동/일시정지를 선택할 수 있어야 한다.
- 이슈를 생성하거나 연결한 뒤에는 사용자가 이슈 기반 브랜치명을 제공해야 한다.
- 구현, TDD, subagent 작업, 검증, 커밋, PR/MR 준비는 `<repo>.worktrees/<branch-slug>` 형식의 격리된 git worktree 안에서만 수행해야 한다.
- 작업 완료 후에는 격리 worktree가 정리 가능한지 확인하고, 제거 전에 cleanup 선택지를 제시해야 한다.
- 이슈와 PR/MR 본문은 반드시 한글로 작성해야 한다.
- 이슈와 PR/MR은 유명 오픈소스 프로젝트의 기여 가이드에서 추출한 repo-local 가이드라인을 따라야 한다.
- 이슈와 PR/MR은 과도한 이모지를 피해야 한다. 의미 있는 소수의 이모지는 허용한다.
- PR/MR의 다이어그램은 리뷰 이해를 실제로 줄일 때만 사용한다. 필요 없는 다이어그램을 억지로 넣는 것은 품질 저하다.

이 벤치마크와 게이트가 없으면 IssueOps prompt나 workflow 변경은 품질 개선을 주관적으로만 주장하게 된다.

## 현재 근거

- 설계 문서: `docs/superpowers/specs/2026-06-02-issueops-quality-benchmark-design.md`
- 이슈/PR 가이드라인: `docs/superpowers/specs/issueops-issue-pr-guidelines.md`
- 기존 IssueOps skill: `skills/issueops/SKILL.md`
- 기존 IssueOps CLI state surface: `cmd/harness/issueops.go`
- 기존 IssueOps core state helper: `internal/core/issueops.go`
- 기존 response contract/golden 패턴은 CLI/MCP JSON surface와 self-verify score 비교를 포함한다.
- 기존 `agy -p` 통합 패턴은 commit suggestion, lint diagnosis, draft wiki suggestion, self-verify LLM evaluation 경로에 있다.

## 완료 기준

- `testdata/issueops/fixtures/*.json` 아래 repo-local synthetic fixture를 추가한다.
- fixture schema는 user prompt, repo context, 기대 issue/plan/task/TDD/subagent/PR 품질, critical failure rule을 담는다.
- deterministic benchmark는 필수 issue section, plan verification, TDD-before-implementation, bounded task ownership, subagent prompt quality, PR/MR field, phase choice gate, isolated worktree evidence를 검사한다.
- deterministic benchmark는 worktree cleanup readiness, 사용자 cleanup choice, safe removal evidence를 검사한다.
- deterministic benchmark는 issue draft와 PR/MR draft가 한글로 작성됐는지 검사한다.
- deterministic benchmark는 issue draft와 PR/MR draft가 `docs/superpowers/specs/issueops-issue-pr-guidelines.md`를 참조하고 핵심 section을 만족하는지 검사한다.
- deterministic benchmark는 복잡도 근거 없이 억지로 넣은 다이어그램을 지양하고, 다이어그램은 review value가 명확할 때만 허용한다.
- `agy -p` LLM judge adapter를 추가하고, judge output은 strict JSON-only schema로 검증한다.
- malformed judge output은 작은 bounded retry 안에서만 재시도하고, 최종 decode/schema failure는 critical failure로 기록한다.
- 다음 score dimension을 포함한다.
  - `intent_understanding`
  - `issue_quality`
  - `plan_quality`
  - `task_decomposition`
  - `tdd_quality`
  - `subagent_orchestration`
  - `implementation_readiness`
  - `pr_mr_quality`
  - `phase_control_quality`
  - `branch_worktree_gate_quality`
  - `isolation_compliance`
  - `worktree_cleanup_quality`
- benchmark output은 average score, minimum score, per-dimension score, deterministic failure, judge failure, critical failure, pass/fail을 포함한다.
- CLI 명령을 추가한다.
  - `agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge agy --json`
  - `agent-harness issueops benchmark compare --baseline KEY --candidate KEY --json`
- baseline/candidate run을 비교할 수 있도록 compact benchmark result를 harness state에 저장한다.
- branch/worktree 요구사항을 기록하고 judge할 수 있도록 IssueOps workflow state 또는 contract evidence를 확장한다.
- CLI/MCP contract가 바뀌면 usage와 response contract golden을 갱신한다.

## Critical Failure 조건

- IssueOps가 단계 선택지를 제시하지 않고 다음 단계로 조용히 진행한다.
- IssueOps가 issue URL 생성/연결 전에 구현을 시작한다.
- IssueOps가 issue 생성/연결 후에도 이슈 기반 브랜치명 없이 구현을 시작한다.
- IssueOps가 source repo에서 구현, TDD, subagent 작업, 검증, 커밋, PR/MR draft를 수행한다.
- IssueOps가 dirty 또는 unmerged worktree를 명시적 승인 없이 제거한다.
- IssueOps가 issue draft 또는 PR/MR draft를 주로 영어로 작성한다.
- IssueOps가 open-source-derived issue/PR guideline reference를 누락하거나 핵심 section을 빠뜨린다.
- IssueOps가 issue 또는 PR/MR draft에 과도한 장식 이모지를 넣는다.
- judge output이 bounded retry 후에도 strict JSON decode 또는 schema validation을 통과하지 못한다.
- benchmark fixture나 result에 secret 또는 실제 private issue 본문을 포함한다.

## 비목표

- 벤치마크가 생기기 전에 IssueOps skill이나 prompt를 최적화하지 않는다.
- benchmark fixture에서 live GitHub/GitLab issue 또는 PR 생성을 요구하지 않는다.
- wall-clock latency를 IssueOps 품질의 primary metric으로 삼지 않는다.
- hook이 issue, worktree, branch, commit, PR, MR을 직접 생성하지 않는다.
- `agy`를 유일한 미래 judge backend로 고정하지 않는다. score schema는 backend-neutral하게 유지한다.

## 검증

- Fixture schema test 통과.
- Deterministic scorer test 통과.
- Fake `agy` test가 valid JSON, noisy output rejection, schema failure를 커버.
- CLI benchmark `run`과 `compare` test 통과.
- Worktree cleanup gate test가 clean, dirty, merged, unmerged, user-declined cleanup scenario를 커버.
- CLI/MCP contract 변경 시 response contract/golden test 통과.
- `go test ./... -count=1` 통과.
- `go build -o bin/agent-harness ./cmd/harness` 통과.
- sample benchmark run이 stable score summary와 compare result를 생성.

## 피드백 기록

- 사용자는 IssueOps prompt 변경 전에 IssueOps Quality Score와 benchmark harness를 먼저 만들자는 범위를 수락했다.
- 사용자는 repo-local synthetic fixture를 수락했다.
- 사용자는 deterministic check와 LLM judge를 함께 쓰는 hybrid scoring을 수락했다.
- 사용자는 strict JSON/schema validation이 있는 초기 judge backend로 `agy -p`를 수락했다.
- 사용자는 phase choice gate와 isolated worktree requirement를 추가했다.
- 사용자는 isolated worktree path convention으로 `<repo>.worktrees/<branch-slug>`를 수락했다.
- 사용자는 worktree cleanup을 IssueOps completion step에 포함하라고 요구했다.
- 사용자는 issue와 PR/MR을 반드시 한글로 작성해야 한다고 요구했다.
- 사용자는 유명 오픈소스 프로젝트의 issue/PR guideline을 참고한 품질 게이트를 요구했다.
- 사용자는 과도한 이모지를 지양하되 적절한 사용은 허용하라고 요구했다.
- 사용자는 개발자가 이해하기 좋은 다이어그램은 필요할 때만 쓰고, 필요 없는 다이어그램을 억지로 넣지 말라고 요구했다.
