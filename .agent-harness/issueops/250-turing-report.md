# #250 Turing report

## Goal

Orca claimable recovery에서 `execution status`가 반환한 actor-free, generation-bound, confirmed `execution resume` 명령을 그대로 실행 가능하게 만든다.

## Acceptance evidence

- AC-01/02/03: CLI actor resolution tests가 actor-free native observation, complete explicit receipt, partial receipt rejection을 검증한다.
- AC-04/05: lifecycle guard tests가 core status command builder의 exact actor-free 출력과 explicit 출력은 허용하고 generation/confirm/substitution/partial actor near-miss는 차단한다.
- AC-06: embedded owner prompt와 Karpathy prompt parity가 sealed claim과 recovery resume을 구분한다.
- AC-07: execution usage, canonical catalog, usage golden, operations와 IssueOps skill reference가 optional actor 계약을 공유한다.
- AC-08: #250 병합 후 #248 Orca lifecycle에서 status의 exact next command를 재실행하고 새 owner가 lease를 claim한 증거를 기록한다.

## Verification

Focused RED는 누락된 resolver, actor-free hook 거부, owner prompt 문구 부재로 실패했다. 최소 구현 후 다음 검증이 통과했다.

- `go test ./cmd/harness/issueopscli/executioncmd ./internal/core/lifecycle ./internal/core/issueops -count=1`
- `go test ./cmd/harness/contractgolden -run Golden -count=1`
- `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/agent-harness ./cmd/harness`
- `git diff --check`

AC-08 live Orca 증거는 이 fix를 parent에 병합·활성화한 직후 #248 generation 1을 exact status→resume으로 재디스패치해 기록한다.
