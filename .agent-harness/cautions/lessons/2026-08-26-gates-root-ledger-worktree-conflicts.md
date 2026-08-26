---
name: cautions/lessons/2026-08-26-gates-root-ledger-worktree-conflicts.md
description: Dated lesson — committing one root GATES.md per IssueOps worktree caused deterministic add/add or content conflicts; task ledgers now use .agent-harness/gates/*.md paths.
---

# 2026-08-26 — Root GATES.md caused conflicts across IssueOps worktrees

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: 사용자 보고와 로컬 git 재현 (2026-08-26)
- Summary: IssueOps의 task-gate 지침이 모든 cycle에 같은 root `GATES.md`를 만들고
  커밋하게 했다. 병렬 worktree 두 개가 서로 다른 acceptance ledger를 같은 경로에
  추가하거나 수정하므로, 두 번째 branch를 merge할 때 add/add 또는 content conflict가
  발생했다.
- Context:
  - `skills/issueops/SKILL.md`가 `agent-harness gates init --file GATES.md`와
    ledger commit을 함께 요구했다.
  - gate discovery는 root `GATES.md`와 `gates/*.md`를 호환 경로로 지원하므로,
    충돌은 파일 형식이나 readiness 판정이 아니라 cycle별 산출물에 공유 경로를 쓴
    naming policy 결함이었다.
- Resolution:
  - `gates init`의 파일 미지정 기본값을 required scope에서 유도한
    `.agent-harness/gates/<scope-slug>.md`로 변경했다.
  - IssueOps는 더 강한 stable key인
    `.agent-harness/gates/issue-<provider-issue-number>.md`를 명시하고,
    `gates abandon`에도 같은
    파일을 전달한다.
  - 기존 root `GATES.md`는 unlazy 호환 discovery 경로로 읽을 수 있지만 신규
    IssueOps cycle은 만들지 않는다.
- Evidence:
  - 임시 git repo에서 두 branch가 각각 root `GATES.md`를 추가한 뒤 순서대로
    merge하면 `AA GATES.md` conflict가 발생했다.
  - 같은 repo에서 두 branch가 `.agent-harness/gates/issue-11.md`와
    `.agent-harness/gates/issue-12.md`를 각각
    추가하면 두 merge가 모두 clean이었다.
  - `TestInitDefaultsToScopeNamespacedFile`과
    `TestGatesCLIInitDefaultsToScopeNamespacedFile`이 root 파일 미생성과 namespaced
    기본 경로를 고정한다.
- Rule: branch/cycle별 산출물은 고정 root 파일명을 공유하지 않는다. review 가능한
  파일 기반 증거를 유지해야 할 때는 durable task identity를 경로에 포함하고,
  compatibility discovery와 신규 write default를 분리한다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
