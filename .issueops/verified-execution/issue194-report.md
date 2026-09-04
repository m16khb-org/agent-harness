# Issue #194 Turing 검증 보고서

- 이슈: https://github.com/m16khb-org/issueops/issues/194
- PR: https://github.com/m16khb-org/issueops/pull/214
- 대상 브랜치: `117-hexagonal-architecture-migration`
- 구현 커밋: `3ccd0e5e4bc27b6f297bde57e84b749202be1293`
- 부모 동기화 후 검증 헤드: `813f71df3d7d25bbe8a77a0bb3eb82c98f02ba44`

## 결과

Orca의 `worktree_create`, `owner_launch`, `dispatch` execution reconcile을
contract/domain/application/inbound/outbound vertical로 이전했다. 공개 CLI/MCP 및
persisted schema는 바꾸지 않았고, `remote_pr_create`, preview, no-pending,
unsupported kind는 기존 경로를 유지한다.

| acceptance | 결과 | 핵심 증거 |
|---|---|---|
| AC-194-01 | PASS | legacy/new result, error, inspection, migration, raw record와 external-intent bytes 차등 테스트 및 contract golden 통과 |
| AC-194-02 | PASS | attempts 0/1/2, authoritative zero, multiple/unknown 후보와 여섯 payload stage의 one-stage call-count 테스트 통과 |
| AC-194-03 | PASS | 세 Orca kind의 injected handler routing, nil handler fail-closed, request-scoped reader, legacy production caller-zero 검증 통과 |
| AC-194-04 | PASS | focused unit/race, dependency fitness, scoped vet, golden, build, 원격 full test/full race 통과 |

공개 contract hash는 동기화 전후 모두
`bef9b8eeb380337c6b4a2e5431e1c2bc08c4a5ccceee63260dff66d934390474`다.

## TDD 및 adversarial 보강

기본 vertical 구현 뒤 독립 리뷰가 두 경계를 찾아냈다.

1. typed snapshot과 raw snapshot 사이 drift가 나면 stale decision을 적용할 수 있었다.
2. production composition이 주입된 `deps.Now`를 handler에 전달하지 않았다.

RED 테스트를 먼저 추가한 뒤 typed/raw drift를 fail-closed하고, migration 뒤
canonicalization 실패에서도 partial migration disclosure를 보존하며, 주입된 clock을
failure receipt까지 전달하도록 최소 수정했다. 재리뷰는 blocker, important, minor
finding 없이 PASS했다. 부모 동기화 뒤 동일 reviewer가 검증 헤드 `813f71df`의
42-file implementation diff를 다시 검토해 동작과 범위가 유지됐음을 확인했다.

## 로컬 최종 gate

2026-08-01 KST, canonical IssueOps worktree에서 실행했다. 아래 명령은 모두 exit 0이다.

```text
go test ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease -run Reconcile -count=1
go test ./internal/core/issueops -run 'Reconcile|OrcaIntent|IssueSnapshot' -count=1
go test ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Reconcile|ExecutionHandler|IssueSnapshot|ResponseContractsGolden' -count=1
go test ./internal/architecture -run Dependency -count=1
go test -race ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp -run 'Reconcile|ExecutionHandler|IssueSnapshot' -count=1
go vet ./internal/contract/issueopslease ./internal/domain/issueopslease ./internal/application/issueopslease ./internal/adapter/inbound/issueopslease ./internal/adapter/outbound/issueopslease ./internal/core/issueops ./cmd/issueops/issueopscli ./cmd/issueops/issueopscli/executioncmd ./cmd/issueops/mcpcli ./cmd/issueops/issueopsapp
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops contract check --json
git diff --check
```

핵심 출력은 각 대상 패키지 `ok`, race 대상 패키지 `ok`, vet 무출력, 두 golden
테스트 `ok`, build exit 0, contract check `ok: true`, diff check 무출력이다.

부모 동기화가 daemon process identity를 포함하므로 다음 회귀 경계도 별도로
확인했다.

```text
TZ=UTC go test ./cmd/issueops/daemoncli ./cmd/issueops/daemoncli/daemonpaths -count=1
TZ=Asia/Seoul go test ./cmd/issueops/daemoncli ./cmd/issueops/daemoncli/daemonpaths -count=1
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o /dev/null ./cmd/issueops/daemoncli/daemonpaths
```

세 명령 모두 exit 0이다.

## 부모 브랜치 동기화와 원격 CI

첫 #214 CI 실패는 #194 vertical이 아니라, 부모 브랜치가 최신 `main`의 Linux
RFC3339Nano process start identity를 포함하지 않아 발생한 두 daemon identity
회귀 테스트였다. 부모 동기화 PR #215에서 `main`의 start identity와 부모의
`ExecutablePathStable: true`를 함께 보존했고, 두 CI 실행을 통과한 뒤 부모에
merge했다. 검증 헤드 `813f71df`에서 #214는 부모 대비 정확히 42개의 #194
implementation 파일만 남았고, 현재 PR은 이 42파일과 evidence report 1파일로
구성된다.

부모 동기화 후 #214 원격 검증:

- push run: https://github.com/m16khb-org/issueops/actions/runs/30697449803 — PASS, 7m19s
- pull_request run: https://github.com/m16khb-org/issueops/actions/runs/30697450996 — PASS, 7m08s

두 run 모두 format, vet, skill frontmatter, build, `go test ./... -count=1`,
`go test -race ./... -count=1`, deterministic self-verify를 통과했다. 프로젝트 계약에
따라 로컬 full test와 local full race는 실행하지 않고 이 원격 결과를 사용했다.

## 범위 및 정리

- `golangci-lint` duplicate 검사는 부모에서 이미 존재하던 hook test 쌍만 보고했고
  #194 신규 구현 중복은 없었다.
- OpenWiki 자동 update, force-push, history rewrite, child worktree/branch 삭제를 하지
  않았다.
- child cleanup은 사용자 별도 선택 전까지 보류한다.
