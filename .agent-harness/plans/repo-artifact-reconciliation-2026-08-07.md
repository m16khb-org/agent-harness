# Repo Artifact Reconciliation — Spec & Plan (2026-08-07)

## 1. Problem

`#228 헥사고날 아키텍처 clean-break` umbrella 작업이 8/4 04:24 (PR #335) 이후 3일간 정지했다.
정지 시점의 저장소는 다음 상태다.

- `228-clean-break-hexagonal-architecture` 통합 브랜치가 main 대비 **74 ahead / 2 behind**.
  하위 PR 20여 건이 main이 아니라 이 브랜치로 머지되어 있어, 228이 막히면 전 작업이 막힌다.
- **고아 lease 2건** — `io-c26802f00c2b`(228), `io-2ffbd9a6739c`(293)이 3일 전 시각으로
  `lease_status=active`, holder=codex.
- **base 삭제로 유실된 PR 2건** — `#264`(이슈 #262), `#273`(이슈 #270)이 base였던
  `258-orca-owner-sealed-claim` 삭제와 함께 동일 시각 자동 CLOSED. 코드는 미머지 상태로 생존.
- **열린 draft PR 2건이 CONFLICTING** — `#265`(248-dogfood), `#316`(293-cleanup).
- **워크트리 15개 / 로컬 브랜치 16개**. 이 중 7개는 228에 완전 흡수(미머지 패치 0)되어 잔여물이다.
- **열린 이슈 50건 중 43건이 브랜치 없음.** 근본 원인별로 뚜렷한 클러스터를 이룬다.

## 2. Non-goals

- `#228` 본체 wave(#229/#233/#234/#235/#236/#238)의 신규 구현.
- `#226` revoking cleanup 결함의 우회 또는 동시 수정 (#228 intent의 non-goal을 승계).
- OpenWiki 자동 갱신.
- 원격 `main`으로의 강제 push, force-push, 브랜치 보호 우회.

## 3. Constraints

- 잔여 worktree/branch 제거는 typed 경로(`issueops cleanup finish|abandon`)만 사용한다.
  raw `git worktree remove` / `git branch -D` 우회 금지 (#293 설계 제약 승계).
- 미커밋 작업은 삭제 전 반드시 커밋으로 보존한다.
- 되돌리기 어려운 원격 mutation(브랜치 삭제, 머지)은 머지 증거를 확인한 뒤에만 수행한다.
- lease 강제 해제는 프로세스 부재 증거를 관측한 뒤에만 수행한다.

## 4. Decisions taken before execution

| # | 결정 | 근거 |
|---|---|---|
| D1 | `274-...-orca-v2`의 미커밋 486줄을 채택본으로 확정 | v2는 PR #277이 어댑터에 넣은 `validateReseedRuntimeRollover`를 제거하고 동일 판정을 도메인 `ValidateHolderlessRuntimeRollover`로 승격한 상위 호환. 빌드 통과 + 관련 24개 패키지 테스트 전부 GREEN |
| D2 | PR #277 / `274-orca-holderless-resume-reseed`를 superseded 처리 | D1의 직접 귀결 |
| D3 | 통합 방향은 `main → 228` (역방향 아님) | 하위 PR 20여 건이 228 기준. 228→main은 wave 완료 이후의 별개 결정 |
| D4 | cleanup은 전 단계 완료 후 마지막에 일괄 수행 | 삭제 대상이 앞 단계의 리베이스 base로 쓰일 수 있다 |

## 5. Goals & Success Criteria

| Goal | 목표 | 완료 판정 |
|---|---|---|
| G1 | 274 v2 채택 확정, #277 supersede | v2 커밋 존재 + 테스트 GREEN + #277 CLOSED with 사유 |
| G2 | 고아 lease 2건 해소 | 두 lease가 `active`가 아님을 `execution status`로 관측 |
| G3 | 228 ↔ main 동기화 | `git rev-list --count origin/main ^228` == 0 |
| G4 | 유실 PR 재생성 (#262, #270, #274) | base=228인 열린 PR 3건 존재 |
| G5 | 열린 PR 정리 (#265, #316) | 두 PR이 mergeable 또는 명시적 사유로 종료 |
| G6 | 이슈 클러스터 통합 | 중복 이슈가 대표 이슈로 링크되고 닫힘 |
| G7 | 최종 cleanup | 잔여 워크트리/브랜치가 typed 경로로 제거되고 `worktree list`가 이를 증명 |

## 6. Verification mode

Proportionate. 전 criterion이 CLI-shaped이므로 auxiliary surface(명령 stdout, git/gh 관측)를
증거로 채택한다. 단 되돌리기 어려운 mutation(G3 머지, G7 삭제)은 **before/after 관측을 쌍으로**
캡처하고, G7은 삭제 전 미머지 패치 0 재확인을 gate로 둔다.

## 7. Cleanup boundary

QA용 런타임 상태를 새로 띄우지 않는다(서버·tmux·컨테이너 없음). 따라서 cleanup receipt는
"none spawned"이며, G7의 저장소 잔여물 제거는 QA 정리가 아니라 **목표 산출물 자체**다.
