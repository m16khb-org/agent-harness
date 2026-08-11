---
name: cautions/lessons/2026-08-04-generated-issueops-command-path-token.md
description: Dated lesson — do not trust a generated IssueOps command executable from its PATH token alone.
---

# 2026-08-04 — 생성된 IssueOps command의 PATH token만으로 실행 바이너리를 신뢰하지 말 것

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps #303 stale installed-binary dogfood
- Summary: core가 만든 `next_command`를 CLI 출력 단계에서만 꾸미거나 executable 관측 실패를 빈 evidence로 대체하면 MCP 경로와 cleanup command가 오래된 PATH 바이너리를 그대로 실행할 수 있다.
- Resolution: contract는 canonical executable path·SHA-256·lease generation의 DTO와 pure bind/validate만 소유한다. Port는 contract를 import하지 않는 순수 observation receipt를 반환하고 application binder가 이를 contract evidence로 변환하며, 실제 observer는 `harnessapp` composition root만 생성한다. CLI와 MCP composition은 생성 command의 첫 token을 관측한 canonical executable literal로 바꾸고 같은 envelope를 결합한다. Hook은 self-declared envelope만 믿지 않고 absolute token을 durable worktree/source의 canonical `bin/agent-harness`로 제한하며 wrapper·substitution·outside-root target을 차단한다. IssueOps root는 subcommand dispatch 전에 현재 executable과 durable generation을 다시 검증한다. 관측 실패, incomplete envelope, binary mismatch, generation drift는 command fallback 없이 structured error로 끝난다. 일반 수동 command의 PATH UX는 유지하고 pre-v1 바이너리는 reserved flag를 알 수 없는 flag로 거부한다.
- Evidence:
  - `internal/contract/issueops/generated_command_provenance.go`
  - `internal/adapter/outbound/issueopsprovenance/observer.go`
  - `cmd/harness/issueopscli/generated_command_provenance_test.go`
  - `cmd/harness/mcpcli/generated_command_provenance_test.go`
  - `cmd/harness/issueopscli/feedbackcleanup/generated_command_provenance_test.go`

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
