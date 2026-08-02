# #117 헥사고날 아키텍처 umbrella 최종화 계획

> 상태: generation 5 direct holder가 실행한다. 새 production capability를 추가하지 않는다.

## 목표

11개 native child가 모두 병합된 parent integration HEAD를 최종 검증하고,
`117-hexagonal-architecture-migration`에서 `main`으로 umbrella PR을 병합한다.
현재 `--disable hooks`로 실행된 부모 Codex 세션은 acceptance 근거에서 제외하고,
별도 fresh Codex·Claude 프로세스가 repository hook을 실제 로드하는지 다시 증명한다.

## 고정 기준

- `main`: `6894d946aa69460e40ea7b3392bb1499e0e6eaab`
- child #200 병합 뒤 parent HEAD: `eb4e498241c8ef2bf7959f8c313590268621ade1`
- parent는 `main`을 포함하며 `origin/main...HEAD`는 `0/92`이다.
- GitHub sub-issue 11개는 모두 `CLOSED`다.
- #200의 caller inventory는 `internal/core=41`, `internal/core/issueops=15`,
  `internal/port=28`, `cmd/harness/issueopscli=1`이고 삭제 후보는 0개다.
- `legacy_imports.txt`와 public/persisted/runtime contract는 유지한다.

## 성공 기준

1. #117 본문과 IssueOps state가 11개 child 완료 및 final umbrella 범위를 반영한다.
2. parent HEAD에서 production dependency graph와 contract golden이 통과한다.
3. fresh configured Codex는 `--enable hooks`로 repository hook을 로드하고 foreign
   mutation을 block하며 sentinel을 만들지 않는다.
4. fresh configured Claude는 repository hook 설정을 로드하고 foreign mutation을
   deny하며 sentinel을 만들지 않는다.
5. format, vet, build, full unit/race, deterministic self-verify 95점 gate가 통과한다.
6. final-head push·PR CI가 모두 통과한 뒤 umbrella PR을 `main`에 병합한다.
7. #117을 CLOSED/COMPLETED로 반영하고 parent branch/worktree/lifecycle을 정리한 뒤
   GitHub open issue와 open PR이 0개임을 확인한다.

## 실행 순서

### Task 1 — 계약과 원격 parent 동기화

- generation 5 holder와 canonical parent worktree를 확인한다.
- lifecycle intent, plan-prep, domain/design/compatibility review를 final umbrella
  범위로 교체한다.
- #117 본문을 all-child-complete 상태와 최종 gate로 갱신한다.
- 이 계획을 lifecycle에 연결한다.

검증:

```bash
git status --short
git rev-parse HEAD
git rev-list --left-right --count origin/main...HEAD
gh issue view 117 --repo m16khb/agent-harness --json state,subIssues,body
```

### Task 2 — 활성 훅 fresh-host proof

- 현재 부모 프로세스를 사용하지 않는다.
- parent HEAD에서 `bin/agent-harness`를 다시 빌드한다.
- isolated `CODEX_HOME`에 repository Codex hook config를 byte-identical 복사하고
  `hooks/list` 및 fresh `codex exec --enable hooks`로 foreign sentinel mutation
  차단과 sentinel 미생성을 확인한다.
- fresh Claude를 repository `configs/claude/hooks.settings.json`,
  `--include-hook-events`, `--permission-mode bypassPermissions`로 실행해 native
  PreToolUse deny와 sentinel 미생성을 확인한다.
- `--disable hooks`, `--safe-mode`, `--bare`는 proof에 사용하지 않는다.

검증 결과는 `.agent-harness/turing/issue117-report.md`에 session ID, deny reason,
sentinel 부재와 함께 기록한다.

### Task 3 — 최종 로컬 gate

다음 명령은 전체 unit과 race를 동시에 돌리지 않고 순차 실행한다.

```bash
gofmt -l $(git ls-files '*.go')
go test ./internal/architecture -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
git diff --check origin/main...HEAD
```

### Task 4 — evidence·독립 리뷰·publication

- `.agent-harness/turing/issue117-report.md`에 child inventory, 활성 훅 proof, local
  gate, 변경 범위와 pending remote boundary를 기록한다.
- Shannon no-input guard를 적용해 production delta와 finalization docs delta를
  분리한다.
- 독립 reviewer가 exact final diff와 완료 주장 경계를 검토한다.
- AI-slop fingerprint와 implementation-review receipt를 final HEAD에 묶는다.
- Korean remote artifact gate와 label scorer 뒤 draft PR을 `main` 대상으로 만든다.

### Task 5 — CI, merge, close, cleanup

- final HEAD의 push와 pull_request CI 두 건이 모두 success인지 읽는다.
- generation 5 `execution complete`로 exact SHA와 run URL을 기록한다.
- PR을 ready로 전환하고 expected-head CAS를 걸어 merge한다.
- completion을 #117에 반영하고 issue를 completed로 닫는다.
- remote parent branch를 typed CAS로 삭제하고 cleanup finish로 worktree, local
  branch, lifecycle record를 제거한다.
- `gh issue list --state open`, `gh pr list --state open`, `issueops list`가 모두
  종료 상태인지 재검증한다.

## rollback

finalization 문서 커밋은 단독 revert할 수 있다. Umbrella merge 뒤 회귀가 발견되면
`main`의 merge commit을 revert하되 child branch나 persisted schema를 다시 쓰지
않는다. Temporary hook homes, fixtures, sentinels와 publication drafts는 exact path로
Trash에 이동해 복구 가능하게 정리한다.
