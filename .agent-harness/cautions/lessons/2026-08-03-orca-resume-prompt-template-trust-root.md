---
name: cautions/lessons/2026-08-03-orca-resume-prompt-template-trust-root.md
description: Dated lesson — Orca resume must not re-render expected prompts from the current binary template.
---

# 2026-08-03 — Orca resume은 현재 prompt template을 trust root로 쓰면 안 된다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps #248/#254 Orca dogfood
- Summary: terminal preparation intent를 삭제한 뒤 resume이 현재 바이너리의 owner prompt template으로 expected prompt를 다시 렌더링하면, 정상적인 template 변경만으로 이미 봉인된 실행이 영구 중단된다.
- Resolution: prepare와 Orca reseed는 identity version 1과 issue-body, context-packet, owner-prompt SHA-256을 generation-bound Orca binding에 함께 저장한다. Resume은 artifact bytes를 이 durable identity와만 비교하고 prompt를 다시 렌더링하지 않는다. Version marker와 세 digest가 모두 없는 기존 v1 binding만 status가 preview → generation-CAS reseed → resume 복구 체인으로 보낸다. Versioned all-empty는 새 persistence 결함이므로 invariant violation이며, unversioned-complete·일부 digest·future version·worktree 파일을 새 trust root로 채택하는 fallback은 fail-closed한다.
- Evidence:
  - `internal/core/issueops/execution_resume_identity_test.go`
  - `internal/adapter/outbound/issueopspreparation/repository_orca_test.go`
  - `internal/application/issueopslease/reseed_test.go`

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
