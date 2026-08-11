---
name: cautions/lessons/2026-08-08-command-only-payload-cwd-fence-exemption.md
description: Dated lesson — command-only payload exempts the cwd fence only when the command fully describes its target.
---

# 2026-08-08 — command-only payload에서는 명령이 대상을 완전히 기술할 때만 cwd fence를 면제한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: IssueOps #331 (atomic commit gate), 후속으로 #329/#330의 owner mutation 경로와 동일 축
- Summary: Codex 0.146의 stable PreToolUse payload는 `exec_command`의 `workdir`를 전달하지 않는다. hook이 보는 cwd는 언제나 turn의 source checkout이므로, cwd 일치를 무조건 요구하면 canonical worktree에서 실행하는 정상 명령이 영구 차단된다.
- Context: `python3 <worktree>/skills/atomic-commit-push/scripts/git_preflight.py <worktree>`를 canonical worktree에서 실행해도 hook은 source cwd로 판정해 `unsafe_mutation`을 반환했다. 상대 경로와 절대 경로 두 형태 모두 막혔다.
- Resolution:
  - 관측할 수 없는 workdir를 추측하지 않는다. 대신 **명령이 대상을 완전히 기술하는지**로 판정한다 — script 절대 경로와 repo 인자 절대 경로가 같은 root를 지목할 때만 cwd 일치를 면제한다(`selfDescribingAtomicWorkflowInvocation`).
  - IssueOps owner mutation은 같은 원리를 provenance로 구현한다: trusted absolute executable + generation-bound provenance + exact actor + command `--cwd` (`exactIssueOpsOwnerHookCWD`, `exactExecutionSyncBaseHookCWD`).
  - 면제는 분류 단계에만 적용되고 holder·worktree·generation fence는 그대로 남는다. 다른 repo를 지목하면 그 lifecycle의 holder가 아니므로 차단된다.
  - 설치본 skill로 worktree를 겨누는 형태는 script root와 repo가 다르므로 이 경로에 들어오지 않는다. 그 조합은 여전히 실제 workdir를 제공하는 transport에서만 동작한다.
- Evidence:
  - `internal/adapter/lifecycle/lifecycle_execution_guard.go` (`selfDescribingAtomicWorkflowInvocation`)
  - `TestAtomicCommitWorkflowSurvivesCodexCommandOnlyPayload`
  - `TestAtomicCommitWorkflowCommandOnlyPayloadStaysFailClosed`
  - `TestAtomicCommitWorkflowCommandOnlyRejectsForeignHolder`
- Boundary: 임의 Python 실행, 다른 script, 다른 repo 인자, repo 인자 생략, 상대 script 경로, foreign holder는 계속 fail-closed다.

> Incident-time IssueOps command, field, and state references are historical evidence only, not execution directives. The current contract is `skills/issueops/references/execution.md` and `.agent-harness/OPERATIONS.md`.
