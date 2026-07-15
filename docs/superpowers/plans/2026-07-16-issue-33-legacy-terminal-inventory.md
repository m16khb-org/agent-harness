# Issue #33: legacy terminal inventory로 병렬 dispatch 복구

## 목표

기존 Orca terminal이 남아 있는 정확한 worker worktree에서 `lease_attestation`이
그 terminal을 inventory 누락으로 오판하지 않게 한다. #24와 #25의 coordinator는
서로의 terminal을 건드리지 않고 실제 worker task와 dispatch를 생성해야 한다.

## 범위와 불변식

- exact worker worktree와 일치하는 terminal만 candidate다.
- task 또는 dispatch가 이미 존재하면 candidate를 새 worker로 채택하지 않는다.
- candidate가 복수이거나 foreign worktree이면 recovery-required를 유지한다.
- raw Orca task/dispatch 호출과 source-checkout 권한 완화는 범위 밖이다.

## 실행 순서

1. 현재 `lease_attestation`의 terminal/task/dispatch inventory 순서를 focused test로 재현한다.
2. singleton legacy terminal을 pre-dispatch worker bootstrap으로 정규화하고 보안 negative case를 보존한다.
3. focused IssueOps handoff tests와 race-safe test를 실행한다.
4. 새 binary/daemon으로 #24와 #25 recovery retry를 병렬 실행해 worker terminal, task, dispatch를 실측한다.

## 검증

`go test ./internal/core/issueops -run 'Legacy|SoleWriter|HandoffStart' -count=1`와
`go test ./internal/core/issueops -count=1`를 실행한다. 이후 #24와 #25의 durable
handoff record에서 서로 다른 worker terminal, task ID, dispatch ID를 확인한다.
