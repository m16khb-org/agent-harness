---
name: cautions/lessons/2026-08-04-codex-command-only-hook-payload-workdir.md
description: Dated lesson — Codex command-only hook payload omits workdir; do not mistake turn cwd for exec cwd.
---

# 2026-08-04 — Codex command-only hook payload에서 선언한 workdir를 실제 cwd로 오인하지 말 것

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps #248/#329/#330 live dogfood
- Summary: Codex 0.146의 stable `ExecCommandHandler::pre_tool_use_payload`는 `tool_input`에 `command`만 넣고 `workdir`를 누락한다. top-level `cwd`도 exec 요청값이 아니라 turn cwd라서, source checkout에서 canonical sibling worktree를 지정한 정상 generated command가 hook에서 영구 차단됐다.
- Resolution: hook은 current generation과 canonical executable provenance를 통과한 absolute IssueOps command에 한해 command의 exact `--cwd`를 canonical root와 대조한다. 일반 PATH/bare 명령과 provenance 없는 absolute 명령에는 이 fallback을 열지 않는다. CLI root는 provenance를 검증한 뒤 owner mutation의 실제 `os.Getwd()`와 `--cwd`를 core mutation 전에 canonical 비교한다. delegation 명령의 durable identity는 `--id`가 아니라 `--parent`로 읽는다.
- Evidence:
  - Codex tag `rust-v0.146.0`의 `codex-rs/core/src/tools/handlers/unified_exec/exec_command.rs`
  - `internal/core/lifecycle/lifecycle_execution_matrix_test.go`
  - `internal/core/lifecycle/lifecycle_owner_mutation_test.go`
  - `cmd/harness/issueopscli/generated_command_provenance_test.go`
- Boundary: hook에서 전달되지 않은 workdir를 복원했다고 가장하지 않는다. 생성 provenance, native holder identity, canonical root, actual process cwd 중 하나라도 어긋나면 mutation을 fail-closed한다.

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
