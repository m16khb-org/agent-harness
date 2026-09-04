---
name: cautions/issueops-execution.md
description: Cautions for IssueOps execution liveness, v1 fence identity, lease authority, operational-health diagnosis, and exact-reader immutability.
---

# IssueOps execution-liveness and lease-authority cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for the worktree
guard/execution-liveness v1 fence, operational-health diagnosis versus cleanup
authority, lease-to-lifecycle-phase promotion, atomic-skill Python gates, and
exact-reader immutability. Orca supervised-handoff operations live in
[issueops-orchestration.md](issueops-orchestration.md); branches, worktree
guards, and readiness gates live in
[issueops-lifecycle.md](issueops-lifecycle.md).

## 21. Worktree guard and execution liveness must use the same v1 fence

A guard that checks only branch or path can deadlock an absent holder or admit a
stale one. All readers must agree on the exact lifecycle ID, generation, native
process receipt, canonical worktree, and mode-specific resource identity.

주의:
- A block message must name one command that the current state actually allows. Start with `issueops execution status`; use its rendered claim, reconciliation, or replacement command rather than inventing an override.
- A missing directory, old timestamp, stable diff, or absent terminal row is not lease-release evidence. Replacement requires expected generation plus preview inventory and quiescence fingerprints.
- Hooks may read bounded state and reject a mismatched mutation, but must not call Git, provider, or Orca mutators, revoke a lease, or delete a worktree.
- One active execution exists per record, not per repository. Exact-ID selection must not capture an unrelated parallel cycle or make the source main worktree read-only.
- Completion is generation-fenced and requires phase `pr` plus verified remote evidence. Never force a non-PR cycle to `done` as a recovery shortcut.

## 23.1 Operational health is diagnosis, not cleanup authority

Cross-system residue must not acquire a second truth source in stale scan, status, or the stability script.

주의:
- `issueops doctor` is the sole public operational-health gate. `--preserve-cycle` and `--preserve-terminal` are exact, invocation-only inputs; never persist them as exemptions.
- A generic binding proves neither authority nor liveness. An active generation without a complete native process receipt, canonical worktree, and mode-specific identity is unhealthy even when other resource IDs match.
- Time is diagnostic only. Age never interrupts a holder, deletes a resource, or authorizes replacement; expected generation, exact inventory, and quiescence proof are required.
- Orca absence is optional only when no durable cycle claims Orca resources. Never turn a missing or incomplete Orca inventory into an empty healthy list.
- One-time global cleanup evidence lives in an external `0700` recovery bundle. Git/SQLite copies can be restore-tested, but Orca snapshot is archival-only: the public CLI has no conditional reset/import/restore and a last-moment external actor race remains. Stop on pre-reset digest drift; after reset, continue idempotently from the append-only journal instead of guessing rollback.
- A destructive IssueOps replacement must not rely on a runner-side observation followed by an unfenced write: record and process state can change in that gap. Use expected generation plus inventory/quiescence fingerprint CAS and journal the locked before/after proof.
- Replacement의 최초 `--preview`는 현재 generation을 찾는 읽기이므로 세대를 생략할 수 있지만, 이후 revoke/finalize/reseed는 preview가 돌려준 exact generation CAS를 요구한다. Orca finalize/reseed는 새 token을 만든 뒤 owner packet/prompt 재봉인까지 성공해야 durable claimable 상태를 기록한다. 실패하면 이전 durable 상태를 보존하고 아직 권한 없는 target generation의 token/packet/prompt residue를 정리하며, 다음 재시도도 같은 exact harness-owned 경로를 먼저 회수한다. GitLab MCP snapshot은 이 재봉인이 실제로 읽는 `replace --finalize|--reseed`에만 전달한다.
- A sealed cleanup must also seal its authorities: invoke a bundle-private clean-HEAD executor by hash/VCS revision, override live state/root/daemon/worker environment paths, require singleton equal fetch/push authorities, and push to the sealed explicit URL. Fetch/prune readiness, mutation, and readback must share that URL plus a heads-only refspec, `--no-tags`, and `--no-write-fetch-head`; never reopen mutable `origin` refspec/tag authority. Ignored `bin/issueops`, inherited environment, and mutable remote names are not execution evidence.

## execution owner에게 lease 명령만 주고 lifecycle phase 전이를 추론시키지 말 것

Orca owner가 active lease를 정상 claim하고 구현·대상 검증까지 마쳤지만 lifecycle은 `problem`에 남아 있었다. 기존 sealed prompt가 `link-plan`, `phase --to implement`, `ai-slop-clean record`, `phase --to ai-slop-clean`, `phase --to pr`의 순서와 exact command를 제공하지 않았기 때문이다. cleanup evidence만 기록해도 phase와 fingerprint는 자동으로 전이되지 않으므로 implementation review가 `implement phase` 이전이라고 거부했다.

- sealed owner packet은 plan 연결부터 PR phase까지 필요한 lifecycle mutation을 exact command로 렌더하고 command catalog로 검증한다.
- staged plan과 기존 `plan_path`가 모두 없으면 임의 계획이나 phase jump로 우회하지 않고 blocker를 보고한다. `link-plan`은 같은 canonical path만 멱등 허용하며 다른 path로의 교체는 fail-closed한다.
- 구현 수정은 `phase=implement` readback 뒤에 시작하고, cleanup evidence 기록 뒤 `ai-slop-clean` 전이로 fingerprint를 봉인한다.
- implementation review는 실제 `pass|revise|stop` verdict를 기록한다. `revise`는 수정·재검증·fresh 리뷰를 반복하고 `stop`은 publication을 중단한다. 리뷰 뒤 diff를 바꾸면 cleanup/review fingerprint를 다시 기록한다. clean/synced push 뒤 `phase=pr`을 통과한 다음에만 governed PR/MR 생성 명령을 실행한다.
- 최신 사용자 지시가 전체 테스트를 제한하면 sealed issue의 오래된 full 명령을 강행하지 않고 targeted 검증과 생략 근거를 verification report에 남긴다.

## active lease에서 atomic 스킬의 Python gate를 일반 관찰로 열지 말 것

`atomic-commit-push`의 필수 `git_preflight.py`가 읽기 전용이어도 Python 스크립트 실행은 정적 shell reader 목록에 없어서 `unsafe_mutation`으로 차단된 적이 있다. 반대로 파일 이름만 보고 observation으로 승격하면 저장소가 제공한 Python 코드가 non-holder에게도 열릴 수 있다.

- `git_preflight.py`와 `api_doc_gate.py`의 정확한 단일 `python3` 호출만 current holder workflow로 인정한다.
- 스크립트는 저장소 상대 경로와 사용자 홈의 설치·심볼릭 링크 경로를 모두 쓸 수 있으므로 사용자별 절대 경로를 하드코딩하지 않는다. 절대 경로는 명시적 expected worktree/source checkout, `ISSUEOPS_ROOT`, `CODEX_HOME`, 사용자 홈의 Codex·Claude skill root처럼 설치기가 관리하는 base와 정확히 일치할 때만 허용한다. generic repo/cwd와 단순 `/skills/...` suffix 비교는 신뢰 근거로 쓰지 않는다.
- 상대 `skills/...` 스크립트는 active lifecycle의 canonical worktree root에서만 실행한다. 하위 디렉터리에서는 같은 상대 경로가 `<subdir>/skills/...`를 가리키므로 holder라도 허용하지 않는다.
- 선택적 repo 인자는 실제 shell 작업 디렉터리와 같은 canonical 경로만 허용한다. Codex 0.146 stable hook은 `exec_command.workdir`를 `tool_input`에 직렬화하지 않고 turn-level `cwd`만 보내므로, generated absolute IssueOps 명령은 CLI가 `os.Getwd()`와 `--cwd`를 mutation 직전에 대조한다. Claude Bash는 top-level `cwd`를 기준으로 삼고 해석이 모호한 상대 `workdir`는 거부한다.
- 비-shell tool은 같은 command 문자열을 실어도 이 경로로 분류하지 않는다. 공백이 포함된 argv, 추가 인자, 다른 인터프리터, 다른 스크립트, 외부 repo 대상은 계속 fail-closed한다.
- 이 경로는 일반 read-only observation이 아니다. 기존 native holder identity와 canonical worktree containment를 모두 통과한 뒤에만 실행한다.

## exact reader를 열기 전에 실제 구현의 무변이성을 확인할 것

Shannon 측정 중 `rg -c`와 `issueops state read --key ...`가 active lease에서 차단되었다. `rg --count`는 허용하면서 같은 read-only short flag `-c`를 빠뜨린 명세 누락이 있었고, 기존 `StateRead`는 이름과 달리 store가 없으면 SQLite 디렉터리를 생성했으므로 곧바로 observation으로 승격할 수 없었다.

- CLI 이름만 보고 read-only로 분류하지 않는다. 파일·DB·네트워크 구현이 누락 상태에서도 데이터를 만들거나 복구하지 않는지 먼저 테스트한다.
- state의 단일 row 조회는 `sqlstore.GetExisting`처럼 기존 store만 여는 경로를 사용하고, 없는 store에서 파일·디렉터리를 만들지 않는 회귀 테스트를 둔다.
- 외부 reader의 long/short 동의어는 실제 사용하는 형태를 모두 characterization corpus에 넣되, 실행기·전처리·출력 파일처럼 mutation으로 확장되는 flag는 계속 거부한다.
- 새 exact reader는 command parser, active lifecycle decision, 실제 hook CLI full payload를 함께 검증한다. parser 단위 테스트만 통과한 상태로 설치 바이너리를 갱신하지 않는다.

## Git porcelain의 선행 공백을 일반 문자열처럼 자르지 말 것

`git status --porcelain=v1`의 첫 행이 unstaged 변경이면 상태 코드는 선행 공백으로 시작한다. 전체 stdout에 `TrimSpace`를 적용하면 첫 경로의 첫 글자까지 상태 필드로 오인해, AI-slop-clean 지문이 같은 내용을 커밋한 뒤 달라지고 plan-only 변경도 구현 변경으로 오탐한다.

- porcelain처럼 공백이 스키마인 출력은 `GitCmdRaw`로 읽고 행 단위 파서에 원문을 전달한다.
- 사람이 읽는 단일 값에 쓰는 `GitCmd`/`GitOut`의 trim 계약을 structured Git 출력에 재사용하지 않는다.
- 회귀 테스트는 첫 행이 ` M <path>`인 tracked 변경을 포함하고, dirty 상태와 동일 내용 commit의 지문 일치 및 tracked plan-only 변경 제외를 함께 검증한다.
- 잘못 봉인된 기존 지문은 첫 파일 내용을 해시에 포함하지 않았으므로 호환 계산으로 통과시키지 않는다. 수정된 바이너리에서 committed diff를 다시 독립 검토하고 새 cleanup/review 지문을 기록한다.

## dirty main에서 원격 변경과 겹치는 파일을 바로 pull하지 말 것

로컬 `main`의 미커밋 변경과 `origin/main`의 fast-forward 변경이 같은 파일을 수정하면 `git pull --ff-only`는 overwrite 방지를 위해 중단한다. 이 실패를 branch divergence나 pull 설정 문제로 오인하지 않는다.

- 먼저 `git rev-list --left-right --count HEAD...origin/main`, 로컬 `git diff --name-only`, `git diff --name-only HEAD..origin/main`을 대조해 실제 겹침을 증명한다.
- tracked와 untracked 변경을 이름 있는 `git stash push --include-untracked`로 보존하고 stash SHA를 기록한 뒤 fast-forward한다.
- `git stash apply`를 사용해 복구 지점을 유지한 채 변경을 재적용하고, conflict marker·diff·focused/full tests와 두 번째 `git pull --ff-only`를 확인한 뒤에만 stash를 제거한다.
- dirty `main`을 맞추기 위해 `reset --hard`, `clean`, 사용자 변경 폐기, 임시 worktree branch 삭제를 사용하지 않는다.
