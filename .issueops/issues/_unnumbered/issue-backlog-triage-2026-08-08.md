# Issue Backlog Triage & Residual Delivery — Spec & Plan (2026-08-08)

## 1. Problem

`#228` 헥사고날 umbrella가 8/8 종료된 뒤 열린 이슈 45건이 남았다. 이 중 어느 것이
실제 미해결 작업이고 어느 것이 이미 끝났거나 중복·무효(slop)인지 구분되지 않아
백로그 자체가 신뢰할 수 없는 상태다.

선행 세션(`repo-artifact-reconciliation-2026-08-07.md`) 종료 후 실측한 저장소 상태:

- 열린 PR 0건, 워크트리 1개(main) — PR/워크트리 축은 깨끗하다.
- **원격 브랜치 42개 중 37개가 main 대비 미머지 패치 0** — 8/8 wave 머지로 다시 쌓였다.
- 로컬 브랜치 4개 잔여, IssueOps lifecycle 21건 중 `cleanup_candidate` 3건.
- `lease_status=active` 2건(`io-c26802f00c2b`/228, `io-2ffbd9a6739c`/293) 고착.
- 6건(#233 #234 #238 #342 #293 #250)은 이미 완료됐는데 닫히지 않은 상태였다 — 본 사이클 착수 시 종료 완료.

## 2. 근본 blocker

코드 실측 결과 남은 이슈의 절반 이상이 **하나의 transport 결함**에서 파생됐다.

`cmd/issueops/hookcli/hookinput/hook_input.go:43` `EffectiveCWDFromHookInput`은 shell tool일 때
`tool_input.workdir`가 전달된다고 가정하고, 없으면 top-level `cwd`(= source checkout)로 폴백한다.
그러나 Codex 0.146.0의 `ExecCommandHandler::pre_tool_use_payload`는 stable hook input을
`{"command": args.cmd}`로만 만들며 `workdir`를 넣지 않는다. 따라서 request CWD로 canonical
worktree를 판정하는 **모든 owner mutation**이 coordinator 세션에서 도달 불가능해진다.

이 결함이 살아 있는 한 나머지 hook 분류 이슈들의 dogfood 검증 자체가 불가능하므로
**T1이 최우선 선행 작업**이다.

## 3. Non-goals

- upstream `openai/codex` 저장소 수정 (#278) — issueops 범위 밖. 추적만 유지한다.
- IssueOps 가드의 일반 shell admission 확대. 모든 허용은 좁은 exact grammar로만 한다.
- Orca relay 자체 수정. issueops 쪽 계약/복구 경로만 다룬다.
- OpenWiki 자동 갱신.
- 원격 `main` force-push, 브랜치 보호 우회.

## 4. Constraints

- 잔여 worktree/branch 제거는 typed 경로(`issueops cleanup finish|abandon`)만 사용한다.
- 이슈 종료는 근거 코멘트(코드 위치 또는 머지 PR 번호)를 동반한다. 근거 없는 종료 금지.
- lease 강제 해제는 프로세스 부재 증거를 관측한 뒤에만 수행한다.
- 모든 구현은 RED → GREEN. 기존 동작을 건드리면 characterization test로 먼저 고정한다.
- hook 허용 확장은 deny matrix를 함께 추가한다 — 허용만 추가하고 거부 케이스를 안 쓰면 미완이다.

## 5. Triage 결과 (45건 → 6개 실작업 + 종료 대상)

### 5.1 이미 해결됨 — 근거와 함께 종료

| 이슈 | 근거 |
|---|---|
| #284 orca worker_done 차단 | `lifecycle_execution_guard.go:662-670`에 `worker_done` 분류 구현됨 (task-id/dispatch-id/outcome/capability route 검증 포함) |
| #302 cleanup finish stale fingerprint | `issueops_cleanup_finish.go:110-112`가 파괴 단계 진입 **전** CAS 검증 후 반환 |
| #322 remote completion actor help 불일치 | `issueops_catalog.go:79-80` 상위 usage와 `remote.go:253-258` 실제 flagset이 모두 `[--provider]`만 요구, RECORD_ACTOR_FLAGS 제거됨 |

### 5.2 slop — 사유와 함께 종료

| 이슈 | slop 사유 |
|---|---|
| #226 | #221 시절 dogfood 잔여. 5개 관측 중 collaboration actor 축은 #276, hook target 축은 #330, cleanup dead-end는 #293(종료)로 각각 승계됨. 원 이슈는 재현 컨텍스트가 소멸 |
| #248 | AC-01/02/04/05/07은 PR #392로 머지 완료. 남은 AC-06은 "이 child 자체를 Orca로 dogfood"라는 self-referential 조건인데 해당 작업이 direct로 이미 완료되어 소급 불가 |
| #260 | 재현 lifecycle `io-268bd6ac6e7a` generation 3과 Orca Run `run_a7765e771192`가 모두 소멸. terminal handle 관찰은 #319 순서 계약으로 흡수 |
| #275 | 재현 대상 `io-19f0c798e299`·#274 worktree가 정리 완료. plan/worktree relocation 요구는 #319 state machine에 흡수 |
| #276 | 재현 대상 두 PID(34169/83262) 모두 소멸. actor identity 축은 #330 provenance 검증으로 승계 |
| #279 | 재현 lifecycle `io-2f357ae7cf7a`가 정리 완료. released lease의 명령 차단 축은 #332가 대표 |
| #280 | 동일 `io-2f357ae7cf7a` generation 2 상태 의존. run `run_8197c82a3887`·task `task_eef6a8b44cfb` 모두 소멸 |
| #278 | `[upstream]` — openai/codex `command_runner.rs` 수정 요구. 이 저장소에서 해결 불가. 진단 표면 개선분은 #268이 보유 |
| #354 | CI 1회 실패 후 동일 커밋 재실행 통과. 재현 조건 미특정이며 로컬 3회·`-count` 반복 통과. 재발 시 재개설 |

### 5.3 실작업 — 6개로 압축

| ID | 대표 이슈 | 흡수 | 내용 |
|---|---|---|---|
| **T1** | #330 | #329 #331 #334 | Codex command-only hook payload에서 owner mutation 도달성 복구. hook은 trusted absolute executable + generation-bound provenance + exact actor + command `--cwd`가 모두 일치할 때만 transport-blind fallback 허용, CLI는 `os.Getwd()`·`--cwd`·durable root 3자 일치를 mutation 전 강제 |
| **T2** | #266 | #272 #299 #301 #321 | active lease read-only reader grammar 확장: `bash -n <repo files>`, `cat`/`sed -n` repo-local 문서, `self-verify` 표준 플래그, bounded `ps`/`pgrep`, quoted 정규식 `rg`. 각각 deny matrix 동반 |
| **T3** | #292 | #267 #309 #312 #320 | exact IssueOps command 분류 보강: executable을 binary identity로 canonicalize(PATH/relative/absolute 동일 분류), `link-related`·`feedback add` grammar 등록, `child start` payload/control flag 분리 파싱 |
| **T4** | #314 | #263 #300 #306 #310 | provider 계약: GitHub create issue/PR result의 `number` 투영, connector base/head 직렬화 보존, remote 단일 판별 시 provider 자동 추론, `create-issue` usage `--provider` 노출, `createLinkedBranch` ref:null partial-success 검출·복구 |
| **T5** | #291 | #283 #323 | cleanup 잔여 계약: absent local ref를 idempotent success로 정규화, superseded merged artifact 경로, 후속 병합으로 전진한 remote branch의 증명 기반 삭제 |
| **T6** | #319 | #325 #328 #332 | Orca 실행 계약: branch/worktree/remote-link 단일 state machine, sealed task identity 기반 completion 수렴(consumer_fenced), generation skew 진단, released sync-base conflict의 bounded writer 권한 |
| **T7** | #268 | — | Codex hook 실패 진단에 event·handler identity·termination reason 기록 (upstream 대기 없이 가능한 범위) |

### 5.4 저장소 잔여물 (이슈 아님)

원격 브랜치 37개 + `228-clean-break-hexagonal-architecture`, 로컬 브랜치 4개,
`cleanup_candidate` lifecycle 3건, active lease 2건, 미커밋 verified-execution 상태.

## 6. Goals & Success Criteria

| Goal | 목표 | 완료 판정 |
|---|---|---|
| G1 | Triage 확정 및 종료 실행 | 5.1·5.2 대상 12건이 근거 코멘트와 함께 CLOSED, 실작업 이슈에 압축 관계 코멘트 부착 |
| G2 | T1 hook transport 복구 | command-only payload fixture에서 owner mutation이 exact 조건에서만 allow, 위조 cwd는 CLI에서 무변경 거부 |
| G3 | T2 read-only reader 확장 | 5개 reader form이 allow, 각 deny matrix가 fail-closed |
| G4 | T3 command 분류 보강 | 3 executable form 동일 분류 + link-related/feedback add allow + long payload holder 판정 불변 |
| G5 | T4 provider 계약 | number 비공백, base/head 보존, provider 자동 추론, usage 노출, ref:null typed 오류 |
| G6 | T5 cleanup 계약 | absent ref 멱등 수렴, superseded/advanced remote 증명 기반 삭제 |
| G7 | T6 Orca 계약 | 순서 state machine 단일화, sealed identity completion, skew 진단, bounded writer |
| G8 | T7 hook 진단 | 종료 사유가 event/handler identity와 함께 기록 |
| G9 | 저장소 잔여물 정리 | 머지 완료 원격/로컬 브랜치 0, cleanup_candidate 0, `git worktree list`가 main만 보고 |

## 7. Verification mode

**Proportionate.** 전 criterion이 CLI/hook-shaped이므로 auxiliary surface(명령 stdout,
hook JSON 판정, go test `-v` 결과)를 증거로 채택한다. 단:

- hook 허용 확장(G3/G4)은 **allow와 deny를 쌍으로** 캡처한다. allow만 있는 증거는 미완으로 본다.
- 되돌리기 어려운 mutation(G9 브랜치 삭제)은 before/after를 쌍으로 캡처하고 삭제 전
  미머지 패치 0 재확인을 gate로 둔다.
- G2는 실제 Codex payload 형태(`tool_input={"command":...}`, top-level cwd = source checkout)를
  fixture로 재현해야 한다. synthetic `workdir` 주입 fixture는 증거로 인정하지 않는다 (#330 명시 요구).

## 8. Cleanup boundary

QA용 런타임 상태를 새로 띄우지 않는다(서버·tmux·컨테이너 없음). Go TempDir fixture는
자체 정리된다. 따라서 대부분의 cleanup receipt는 "none spawned"이며, G9의 저장소 잔여물
제거는 QA 정리가 아니라 목표 산출물 자체다.
