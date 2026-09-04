---
name: cautions/lessons/2026-08-27-cleanup-stops-worktree-processes.md
description: Dated lesson — cleanup finish stopped on one Orca terminal shell occupying the worktree and asked the user to close it; cleanup finish/abandon now stop worktree processes and Orca terminals themselves with requester refusal gates (#477).
---

# 2026-08-27 — cleanup이 워크트리 Orca 터미널 하나에 막혀 사용자에게 종료를 되돌려 보냈다

Family index: [CAUTIONS.md](../../CAUTIONS.md).


- Kind: `caution`
- Source: IssueOps #477 (io-2904978dedb4), Claude Code session 2026-08-27
- Summary: cleanup finish가 워크트리를 점유한 Orca 터미널 셸 하나 때문에 workspace_processes_quiescent로 멈춰 사용자에게 닫아 달라고 요구했다. 정리를 요청한 사용자에게 워크트리에 매인 터미널과 프로세스는 정리 대상이다.
- Context: 2026-08-27 대상 repo 사이클 io-22a5f93d7fc2 정리: MR 머지·완료 기록·이슈 close까지 끝났는데 cleanup finish --preview가 workspace_processes_quiescent 하나로 막혔다. 점유자는 Orca.app → login(orca-tcc-login) → zsh(cwd=워크트리) 한 개였고 Orca 런타임은 ready였으며 orca terminal stop --worktree가 존재했지만 하네스는 시도하지 않았다. 사용자: "cleanup을 요청했으면 터미널도 다 닫고 cleanup해야하는거아냐?" 설계 제안 뒤 사용자가 점유 프로세스 전부 종료·abandon 포함·Orca 런타임 불가 시 signal 폴백을 승인했다. 두 차례 독립 design-review 검토가 요청자 자기 종료(terminal stop --worktree는 터미널 선택자가 없고 cwd를 옮긴 터미널도 죽인다, 무선택자 terminal show는 UI-active 터미널을 돌려준다), 대화형 zsh의 SIGTERM 무시(SIGHUP에 종료), 점유자+자손 종료가 공유 서버의 무관한 세션을 죽이는 문제, 새 실패 단계의 codec 등록, selector_not_found 처리, cleanup status 투영 파급을 지적했고 모두 실측으로 확인했다.
- Resolution: cleanup finish/abandon(#477): preview가 점유 프로세스 receipt·자손/부수 피해 수·Orca 터미널 handle을 싣고 fingerprint에 결속하며 점유 자체는 막지 않는다. apply ①′ workspace_processes_stop은 봉인된 handle마다 `orca terminal close --terminal`을 호출해 same-handle·`ptyKilled=true` receipt를 확인한 뒤 HUP+TERM → 5초 폴링 → KILL → 최종 점유·터미널 재관측 순서로 닫고, 그 뒤에만 orca 회수·git 제거로 넘어간다. bulk `terminal stop --worktree`는 fingerprint에 없던 동시 생성 터미널까지 닫을 수 있어 사용하지 않는다. 점유나 터미널이 남으면 레코드를 보존한 채 멈춘다. 요청자 보호는 거부(requester_occupies_worktree, ORCA_PANE_KEY/ORCA_TERMINAL_HANDLE과 terminal list join 기반 requester_terminal_outside_worktree/unresolved, worktree_is_source_checkout). 신호는 apply 시점 점유자에게만, 자손은 stale 허용 근거로만. Orca 바인딩 사이클은 reachable runtime·ready runtime state·ready graph를 요구하고, abandon ⑨는 TaskLive를 거부하며 TerminalLive는 ①′가 닿을 수 있을 때만 통과한다. malformed/중복 requester·terminal inventory, exact close receipt 실패, 최종 inventory 잔존은 모두 fail-closed다. 새 실패 단계는 record_validation과 abandon matcher에 등록하고 cleanup status는 종료 예정 목록을 warning으로 투영한다. abandon receipt는 #433 비대칭 잔여(worktree-only/branch-only)에서도 재-preview 가능하다.
- Evidence:
  - orca terminal stop --worktree path:<wt> --json → {stopped:1}, zsh·sleep 약 2초 내 종료; cd <repo> && sleep으로 cwd를 옮긴 터미널도 종료(자기 종료 위험 실재)
  - orca terminal close --terminal <handle> --json → `result.close.handle=<handle>`, `ptyKilled=true`; bulk stop 대신 fingerprinted exact close에 사용
  - 대화형 zsh: SIGTERM 뒤 생존, SIGHUP 뒤 종료(2026-08-27 실측)
  - orca status --json .result.app.pid=903 .result.runtime.state=ready; orca terminal list --worktree path:<미등록> → ok:false error.code=selector_not_found
  - ORCA_PANE_KEY=tabId:leafId · ORCA_TERMINAL_HANDLE이 orca terminal list --json 행과 join됨(이 세션 실측)
  - internal/adapter/issueops/cleanup_workspace_processes.go, cleanup_workspace_gates.go, issueops_cleanup_finish.go, issueops_cleanup_abandon.go; internal/adapter/orca/client.go(ListAllTerminals, ListWorktreeTerminalsByPath, StopWorktreeTerminals); internal/port/cleanup_workspace.go, orca.go(CleanupOrcaTerminals, AppPID); internal/contract/issueops(record_validation, cleanup_types)
  - go test ./internal/adapter/issueops ./internal/adapter/orca ./internal/contract/issueops ./cmd/issueops/issueopscli/... -count=1 (RED→GREEN, gate ledger G1~G7)
- Rule: cleanup 요청은 워크트리에 매인 세션까지 정리하라는 뜻이며, 요청자 식별은 env join으로만 하고 무선택자 terminal show를 쓰지 않는다. Evergreen 규칙: [issueops-lifecycle.md §32](../issueops-lifecycle.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
