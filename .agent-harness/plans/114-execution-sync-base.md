# 이슈 #114 — execution sync-base: completion 이후 typed 충돌 해소 표면

이슈: https://github.com/m16khb/agent-harness/issues/114

## 문제

sealed git topology 가드가 lifecycle 전 기간 canonical worktree의 merge/rebase를 형태 차단해, execution complete 이후 PR 머지 충돌을 하네스 안에서 해소할 경로가 없다. 실증(2026-07-25): PR #110 CONFLICTING — released·재claim 상태 모두에서 merge/rebase/fetch 거부, 별도 clone 우회로 해소(durable 기록 부재, #99 등록).

## 설계 v2 (brooks revise 전건 반영)

`issueops execution sync-base --id ID ACTOR_FLAGS` 신설. **가드 정책은 그대로 두되 typed 명령 가산 등록이 필요하다**(brooks F1 실측 정정 — "가드 무변경" 원문구는 거짓): lifecycle guard의 typed control plane switch 목록과 commandparse spec에 `execution sync-base`를 등록해야 워크트리 cwd에서 통과한다. typed 등록은 훅의 mutation 가드 블록 전체를 스킵시키므로(**brooks F14**) lease·권위 검사는 100% core 책임이며, RED에 훅 레벨 케이스(worktree cwd × 4모드 × 플래그 조합)를 포함한다.

### 4모드와 권위 (brooks F8·F17 결정: 변형 모드는 활성 holder 필수로 통일)

- `--preview`: released에서도 허용(관측 전용 — merge-tree는 ODB에 객체를 기록하나 워크트리 무오염, "무mutation" 표현은 "워크트리 무오염"으로 정정, brooks F12). base fetch 후 병합 필요성·예상 충돌 파일(`git merge-tree --write-tree --name-only -z`, exit 0/1/기타 구분, git 2.38 미만 fail-closed)·HEAD/브랜치/원격 브랜치 존재/merge-in-progress 전부 노출(비-holder 세션 진단 채널) + fingerprint 발급.
- `--apply --confirm --fingerprint SHA256`: **활성 lease holder 필수**(sameNativeActor 완전 일치 — 게이트 절반 축소, 무잠금 변형 소멸). base를 작업 브랜치로 merge. 무충돌 → merge commit + push 완결. 충돌 → 파일 나열 후 merge-in-progress 정지(해소 편집은 같은 holder).
- `--finalize`: 활성 holder 필수. conflict marker 잔존 거부 후 merge commit + push.
- `--abort`: 활성 holder 필수. `git merge --abort`로 명시 철회.

released 사이클은 reseed→claim 선행이 계약(109 사이클 실증 경로). 트레이드오프 기록: released fast 경로 포기 — 게이트 표 단순화·동시 변형 위험 제거가 재claim 1회 비용보다 크다.

### 게이트 (fail-closed, missing 나열)

1. `completion_present`: Completion == nil 거부(AC-02).
2. `remote_artifact_present`: RemoteArtifact == nil 거부.
3. `remote_branch_present`: `git ls-remote --heads origin <branch>` 부재 거부 — 머지·삭제된 브랜치 부활 방지(brooks F7).
4. `pending_intent_absent`: Execution.Pending != nil 거부 → reconcile 안내(brooks F13, replace 선례 준용).
5. `worktree_present` + `cwd_canonical`: 호출 cwd == canonical root 필수(brooks F2 — 소스 루트 호출의 훅 사각지대 봉쇄; complete/claim 선례 준용).
6. `head_on_recorded_branch`: `git branch --show-current` == 기록 브랜치, detached 거부(brooks F3 — 무증상 push 실패 방지).
7. `merge_state_clean`(preview·apply 진입): MERGE_HEAD/CHERRY_PICK_HEAD/REBASE_HEAD/rebase-merge 디렉토리 검출 시 apply 거부(finalize 또는 abort 안내, brooks F11).
8. `worktree_clean`(apply 진입): tracked 변경만 차단, untracked는 결과에 경고 나열(brooks F10 — 상시 거부 방지).
9. `lease_holder`(apply/finalize/abort): 활성 lease + sameNativeActor 완전 일치(ACTOR_FLAGS 전체 + session process receipt — brooks F4; 훅은 --id만 보므로 core 강제).
10. fingerprint TOCTOU: inventory(id, repo, branch, base branch, base tip OID(fetch 후), work tip OID, in-progress 여부, 원격 브랜치 존재) sha256, apply 직전 재계산.

### 절차 계약

- **fetch 선행**(preview·apply): `git fetch origin <base>` — stale base 머지 방지(brooks F6, pr-readiness strict 선례).
- **push 계약**(brooks F5 — issueops 프로덕션 첫 push): env `GIT_TERMINAL_PROMPT=0` + `GIT_SSH_COMMAND=ssh -oBatchMode=yes`, context timeout, 실패는 typed 오류. non-fast-forward 거부 시 로컬 merge commit 잔존 상태는 preview가 "push 재시도 필요(ahead)"로 보고하고 apply 재실행이 merge 생략 후 push만 수행하는 멱등 수렴.
- **durable 기록**(brooks F9): apply/finalize 성공 시 record에 sync-base 이벤트 append(base OID, merge commit OID, 충돌 파일 수, actor, 시각). `Completion.FinalHead`는 **불변 유지**(완결 시점 증거 보존) — PR head는 provider가 관측하며 이벤트가 merge OID를 담당. 정책 명문화.
- **MCP 표면**(brooks F15): CLI 전용 — ExecuteExecution 밖, MCP action 미추가(카탈로그·mcp golden 무변경).

### CLI/parity 연쇄 (brooks F16 체크리스트)

adapter usage(96-102 부근)·issueOpsUsageText(44-50 부근)·executioncmd Usage const+Run switch·usage golden 재생성·commandparse spec·guard typed 목록·문서(OPERATIONS 정리 순서, skills execution 참조는 별도 저장소라 비범위 기록). dispatch registry 무영향(top-level execution 기존재), contract golden 선택(미채택 — complete도 없음).

## TDD 순서

1. RED: 게이트 10종 거부 전수 + 훅 레벨 케이스(worktree cwd × 4모드: typed 등록 전 거부 → 등록 후 통과) + 무충돌 fast 경로(fetch→merge→push 시퀀스) + 충돌 정지 + finalize(marker 잔존 거부) + abort + push 실패 멱등 + fingerprint TOCTOU + durable 이벤트 기록.
2. GREEN: core(`internal/core/issueops/execution_sync_base.go`) → guard/commandparse 등록 → CLI → usage/golden.
3. 회귀: issueops·commandparse·lifecycle·executioncmd·harnessapp·전체 green.

## AC 매핑

- AC-01: typed 명령만으로 fetch→merge→push 완결(무충돌 fast 경로 테스트, 실환경 실증은 다음 충돌 발생 시).
- AC-02: completion 부재·비holder·claimable/revoking·detached·pending 거부 테스트.
- AC-03: merge ancestry 보존(rebase 부재가 코드 구조로 보장) + 전체 green.

## 비범위

- rebase 지원, 충돌 자동 해소, SealedGitTopologyMutation 완화, MCP action, 원격 브랜치 삭제, skills 저장소 문서.

## 역할 분담

- 계획·brooks 리뷰: Fable 5(메인). 구현안: Opus 5 서브에이전트(holder 적용).
