## 요약

- 기존 `~/.local/bin/issueops` regular file은 기본적으로 계속 거부하고, 명시적인 `--adopt-command-file` 승인과 정적 Go build identity 검증이 있을 때만 adoption합니다.
- install wrapper의 managed-path dry-run preflight를 native activation Begin보다 먼저 수행하고, 실제 staged/canonical 실행 파일을 candidate로 검증합니다.
- mode `0600` backup과 atomic exchange/displaced-object 검증으로 apply·rollback 경합에서 승인되지 않은 동시 교체 파일을 덮어쓰지 않습니다.
- Begin/Seal/Abort를 exact transition ID로 fence하고, Seal 전 실패는 command rollback과 단일-owner Abort로 정리합니다.
- child-host smoke의 adoption 승인을 literal confirmation에 한정하고 source restore 계약을 유지합니다.

## 검증

- `go test ./internal/adapter/installutil ./internal/application/nativeactivation ./internal/adapter/outbound/nativeactivation ./cmd/issueops/installcli -count=1`
- `go test ./internal/adapter/hostprobe ./internal/adapter -run 'ChildHostSmoke|InstallNative|NativeActivation' -count=1`
- `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run 'Golden|ResponseContracts' -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/issueops ./cmd/issueops`
- `git diff --check`

## 위험 및 복구

- adoption은 current-user executable single-link regular file과 exact managed build identity에만 허용됩니다.
- Seal 전 오류는 private backup에서 bytes/mode를 복원하고 exact pending transition만 Abort합니다.
- Seal 이후 backup cleanup 오류는 committed activation을 되돌리지 않고 recovery path를 receipt에 유지합니다.

Closes #261
