---
name: cautions/lessons/2026-07-31-orca-task-mutation-explicit-run-coordinator-consumer.md
description: Dated lesson — Orca task mutation must seal the explicit Run and coordinator consumer together.
---

# 2026-07-31 — Orca task mutation은 explicit Run과 coordinator consumer를 함께 봉인해야 한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: Orca 1.4.162 IssueOps #194 dogfood
- Summary: `task-list`를 전역 호출하거나 현재 terminal RPC에 `--from`을 강제로 붙이면 `legacy_read_only` 또는 `consumer_fenced`로 막힌다.
- Context: Orca 1.4.162는 task create/update/dispatch를 Run 단위로 격리하고, mutation을 실행하는 coordinator terminal이 그 Run의 current consumer인지 확인한다. CLI가 설치되어 있고 worktree/terminal 기능이 정상이어도 전역 task readiness probe 때문에 `mode=auto`가 direct로 내려갈 수 있었다.
- Resolution:
  - readiness는 `ORCA_TERMINAL_HANDLE` 형식을 먼저 검증한 뒤 `run-current --json`과 완전한 `run-list`로 확인하고 mutation을 실행하지 않는다.
  - prepare/resume는 `run_create`와 `run_bind`를 별도 journal stage로 기록한다.
  - 현재 coordinator가 실행하는 Run/task mutation은 `--from`을 생략해 Orca가 호출 프로세스의 terminal authority를 인증하게 한다. `--from`은 다른 terminal을 명시적으로 대리하는 복구·조회 경로에만 사용한다.
  - injected worker 제어 명령은 예전 `--to` 주소와 현재 `--dispatch-capability` 주소를 섞지 않는다. capability 경로는 exact `--from`이 필요하고 `worker_done`은 `--outcome succeeded|failed`를 반드시 포함한다. hook은 이 표면만 열거하며 capability 원문을 기록하지 않는다.
  - `branch_prepare.link_verified=false`인 GitHub owner packet에는 `gh issue develop --list <issue> --repo <owner/repo>`를 exact reader로 함께 봉인한다. owner가 임의 GraphQL을 만들게 하지 않고, 연결 readback 뒤에만 branch prepare recorder를 실행한다.
  - 환경의 exact concrete terminal handle은 coordinator 실행 여부를 fail-closed로 확인하는 gate이며, focus/cwd를 권한으로 추론하지 않는다.
  - task 인벤토리는 모든 explicit Run의 `task-list --run`을 합치며 Run ID와 runtime ID를 함께 검증한다.
  - Run 도입 전 binding은 같은 task ID가 정확히 한 explicit Run에 있을 때만 읽기·완료 처리를 복구한다.
- Evidence:
  - internal/adapter/orca/client.go
  - internal/adapter/orca/execution.go
  - internal/core/issueops/execution_orca_intent.go
  - TestExecutionOrcaRunBindCanConvergeAfterUnknownOutcome
  - TestClientTaskInventoryKeepsSameTaskIDDistinctAcrossRuns
  - TestExecutionAdmitsExactOrcaOwnerControlPlaneCommands
  - TestRunHookPreToolUseAllowsCurrentOrcaOwnerControlCommands

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.issueops/OPERATIONS.md`.
