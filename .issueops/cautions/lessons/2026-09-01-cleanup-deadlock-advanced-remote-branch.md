---
name: cautions/lessons/2026-09-01-cleanup-deadlock-advanced-remote-branch.md
description: Dated lesson — a merged cycle whose remote branch advanced past the merged head could neither delete nor keep that branch, so no typed path finished it; cleanup finish now takes --keep-remote-branch.
---

# 2026-09-01 — 전진한 원격 브랜치가 cleanup을 교착시켰다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: 사내 GitLab 이슈 #2828 사이클 정리. MR은 `8636a3bf07`로 머지됐고, 그
  **뒤에** 문서 커밋 2개(`4387e487ac`, `d6357086ec`)가 같은 브랜치로 push됐다.
- Summary: 원격 브랜치를 지우는 문과 남기는 문이 동시에 잠겼다.
  `cleanup remote-branch`는 게이트 ⑩ `remote_tip_equals_merged_head`에서 막혔고
  통과 경로 셋이 모두 불성립했다(OID CAS: 원격 tip `d6357086ec` ≠ 머지 head
  `8636a3bf07`; ancestry: `origin/2803` `607e9e03d7`의 조상 아님; `--superseded-by`:
  후속 merged MR 없음). 남기고 끝내려 하면 `cleanup finish`가
  `remote_branch_absent`로 막았다. 하네스 안에서 사이클을 끝낼 경로가 하나도 없었다.
- Context: 두 게이트는 각각 옳다. ⑩은 머지 이후 push된 커밋의 유실을 막고,
  `remote_branch_absent`(design-review H8)는 finish가 레코드를 지운 뒤 typed 삭제 경로가
  그 브랜치에 닿지 못하는 상태를 막는다. 교착은 둘의 교집합에서만 생기고, 어느 한
  게이트를 읽어서는 보이지 않는다. `cleanup abandon`도 출구가 아니다
  (`RemoteBranchDeletion: "not_planned"` — 원격을 건드리지 않는다).
- Resolution: `cleanup finish --keep-remote-branch`가 남기는 문을 연다. H8의 대가는
  면제하는 대신 기록으로 갚는다: 결과의 `kept_remote_branch`(branch, remote_oid,
  state)와 이슈 감사 라인의 `remote_branch_kept=<branch>@<oid>`가 레코드 삭제 뒤 그
  브랜치를 찾을 유일한 흔적이다. 원격을 읽지 못하는 경우도 같은 문을 쓰되
  `state: "unreadable"`로 구분해 적는다. 지우는 문(게이트 ⑩)은 그대로 두었다.
  (`issueops_cleanup_finish.go`, `TestCleanupFinishKeepsSurvivingRemoteBranchWhenRequested`,
  `TestCleanupFinishKeepsRemoteBranchWhenRemoteIsUnreadable`,
  `TestCleanupFinishReportsNothingKeptWhenRemoteBranchAlreadyAbsent`)
- Evidence: 사고 당시 `git push origin --delete`로 원격 브랜치를 직접 지운 뒤에야
  finish가 통과했다. 폐기된 커밋 2개는 소스 저장소의 `archive/2828-discarded-docs`
  태그(`d6357086ec6a348282db20403ac5b43568704f96`)로 보존했다.
- Rule: 원격 브랜치가 남아 cleanup이 막히면 손으로 지우지 않는다. 출구는 둘이고
  선택은 사용자 몫이다. 지울 것이면 `cleanup remote-branch`(별도 결정, 별도 흐름),
  남길 것이면 `cleanup finish --keep-remote-branch`다. 게이트 하나가 막았다고 해서
  다른 게이트도 함께 막혔는지는 따로 확인해야 한다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
