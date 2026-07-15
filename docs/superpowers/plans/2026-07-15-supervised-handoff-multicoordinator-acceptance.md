# Supervised Handoff Multi-Coordinator Acceptance Matrix

| 상황 | 기대 결과 | 금지된 외부 호출 | 증거 |
| --- | --- | --- | --- |
| source recipient 1개 | preview가 그 handle을 반환하고 confirm이 해당 handle을 seal | 다른 record recipient 사용 | `TestHandoffStartPreviewAutoSealsUniqueSourceRecipient` |
| source recipient 0개 또는 다수 | record는 변하지 않고 concrete handle을 요구 | task create, dispatch | `TestHandoffStartPreviewRejectsAmbiguousSourceRecipients` |
| active record handle 충돌 | 새 record는 시작 전 거부 | Orca 호출 | `TestHandoffStartRejectsRecipientSealedByAnotherActiveRecord` |
| worker baseline 1개 | baseline을 adopt하고 terminal 생성 없이 dispatch | second terminal create | `TestHandoffStartAdoptsExactlyOneCleanWorkerBaseline` |
| worker baseline 0개 | 정확히 한 terminal을 생성 | duplicate create | `TestHandoffStartCreatesTerminalTaskDispatchExactlyOnce` |
| worker baseline 다수·partial dispatch | recovery/fail-closed | terminal stop, task/dispatch auto-cancel | focused handoff recovery regressions |
| 서로 다른 record/worktree | 각 record의 coordinator/worker handle과 context가 분리 | cross-record handle reuse | recipient collision regression + runtime preview |
| 같은 record 경합 | durable checkpoint의 stable projection, duplicate terminal/task/dispatch 없음 | second external mutation | race test and operation-journal regressions |

검증 명령:

```bash
go test ./internal/core/issueops ./internal/core/lifecycle -count=1
go test -race ./internal/core/issueops ./internal/core/lifecycle -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
```
