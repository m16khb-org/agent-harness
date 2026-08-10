## 요약

- IssueOps completion이 Orca task를 선제적으로 종료하지 않도록 제거했습니다.
- worker owner의 `worker_done` 신호가 task의 유일한 terminal transition을 계속 담당합니다.

## 변경 사항

- completion service의 settler port, outbound adapter, Orca wiring을 제거했습니다.
- 기존 JSON 소비자를 위해 settlement 결과 필드는 optional 형태로 유지하되, 새 completion에서는 채우지 않습니다.
- direct completion과 동일 재시도에서 Orca settle 호출이 없음을 검증하는 RED/GREEN 테스트를 추가했습니다.

## 검증

- `go test ./internal/application/issueopscompletion -count=1`
- `go test ./cmd/harness/harnessapp ./cmd/harness/contractgolden -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/agent-harness ./cmd/harness`

## 범위

- worker dispatch capability, late-message 처리, completion의 durable done/released semantics는 변경하지 않았습니다.
