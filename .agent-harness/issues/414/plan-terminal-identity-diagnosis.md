# #319 Orca 순서 계약 검증 — Plan (2026-08-09)

## 현재 이슈와 lifecycle

- 이슈: https://github.com/m16khb/agent-harness/issues/414
- lifecycle: `io-7d96075bb78f`
- branch: `414-terminal-identity-diagnosis`
- base: `main`

## 목표

`skills/issueops/SKILL.md` line 18이 명시한 #176 ordering을 실제로 밟아 통과 여부를 실측한다.

`branch prepare`(base SHA only) → `artifact stage --name plan` → `execution prepare --mode orca`
→ GraphQL `createLinkedBranch`(oid=sealed base SHA) → `branch prepare --link-verified`

## Bounded scope

- 순서 실측만 한다. 코드 변경은 관측 결과가 결함을 가리킬 때만.
- 이전 시도(PR #410)는 pre-check를 열어 orphan을 만들었고 PR #413으로 되돌렸다.

## Acceptance criteria

- AC-01 문서 순서대로 `execution prepare --mode orca`가 통과한다.
- AC-02 Orca가 suffix 없는 정확한 branch로 worktree를 만든다.
- AC-03 그 뒤 `createLinkedBranch`가 같은 이름에 붙는다.
- AC-04 `branch prepare --link-verified` 재기록이 통과한다.
- AC-05 실패 시 orphan worktree/branch/intent를 남기지 않는다.

## 검증

```bash
agent-harness issueops execution status --id io-7d96075bb78f --json
git worktree list
orca worktree list --json
```

## Cleanup boundary

이 검증이 만든 Orca worktree, 로컬/원격 브랜치, lifecycle record는 typed 경로로 회수한다.
