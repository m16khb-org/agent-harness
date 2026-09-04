# #251 verification report

## 계약

- Codex/Claude shell tool의 명시적 `tool_input.workdir`를 lifecycle CWD로 사용한다.
- workdir이 없거나 빈 값·비문자열이면 기존 top-level/nested cwd를 유지한다.
- 일반 `tool_input.cwd`는 신뢰하지 않고 holder identity fence를 보존한다.

## TDD 증거

- RED: `TestCWDFromHookInputPrefersExplicitToolWorkdir`가 `/source`를 반환해 실패했다.
- GREEN: shell tool에서만 `EffectiveCWDFromHookInput`이 non-empty string `tool_input.workdir`를 우선하도록 한 뒤 focused test가 통과했다.
- 실제 bootstrap: 동일 parent holder의 `issueops child start`가 임시 활성화 전 `write_lease_required`로 거부됐고, 수정 바이너리 활성화 후 같은 canonical `workdir`에서 성공했다.

## 검증

- `go test ./cmd/issueops/hookcli/hookinput ./cmd/issueops/hookcli ./internal/core/lifecycle -count=1`
- `go test ./cmd/issueops/contractgolden -run Golden -count=1`
- `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/issueops ./cmd/issueops`
- `git diff --check`

모든 명령이 성공했다.
