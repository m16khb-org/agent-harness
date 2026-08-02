# IssueOps 병렬 격리 worktree 도그푸드 보고서

## 실행 목표

GitHub 이슈 생성부터 provider-native child 분할, 서로 다른 direct execution·canonical worktree·Codex agent의 병렬 구현, parent review·통합, PR merge, remote/local cleanup까지 한 lifecycle로 실측한다. regression plane과 lifecycle plane은 별도 판정한다.

## 기준 상태

- Source checkout: `/Users/m16khb/Workspace/agent-harness`
- Source branch/HEAD: `main` / `a8303efad9e093dcd6e43b0ab2a1a9622ebade9b`
- Baseline dirty paths: 0
- Baseline open GitHub issues/PRs: 0 / 0
- Baseline IssueOps records: 0
- Baseline worktrees: source `main` 하나
- GitHub authenticated user: `m16khb`
- Doctor baseline: binary current; `operational_inventory_unknown: orca_gates_failed` 한 건으로 `healthy=false`. 이번 direct-mode 범위 이전부터 존재한 Orca inventory 관찰이며 regression 판정과 분리한다.

## 원격 artifact

- Parent issue: [#221](https://github.com/m16khb/agent-harness/issues/221)
- Parent labels: `enhancement`, `documentation`
- Parent assignee: `m16khb`
- Related score threshold: `0.70`
- Selected related issues: `#65=0.95`, `#47=0.86`, `#129=0.80`
- Rejected related issue: `#59=0.18`
- Child A issue: [#222](https://github.com/m16khb/agent-harness/issues/222), provider hierarchy·labels·assignee·body readback 완료
- Child B issue: [#223](https://github.com/m16khb/agent-harness/issues/223), provider hierarchy·labels·assignee·body readback 완료
- Child A PR: [#224](https://github.com/m16khb/agent-harness/pull/224), `ae066829ea8a4e09abee8b4fe0e937fc11aac407`로 parent branch merge 완료
- Child B PR: [#225](https://github.com/m16khb/agent-harness/pull/225), draft·base/head·labels·assignee readback 완료

## Parent lifecycle

- Lifecycle ID: `io-1a6a8e362e51`
- Branch: `221-issueops-parallel-worktree-dogfood`
- Source sealed base: `a8303efad9e093dcd6e43b0ab2a1a9622ebade9b`
- Mode/generation: `direct` / `2` (`generation 1`은 hook binary bootstrap을 위해 정상 release 후 reseed)
- Canonical worktree: `/Users/m16khb/Workspace/agent-harness.worktrees/221-issueops-parallel-worktree-dogfood`
- Holder host/session/process: `codex` / `019fc065-25e9-7613-9de3-86c8b61b502c` / PID `56675` start `2026-08-02T02:54:41Z`
- Plan commit HEAD: `2bac6a590abc1234bc7720752329143415b1271e`
- Child sealed base: `b4ee1dd881abaa50335af0358e71117961ff7513`
- Lifecycle fixes: `49a21468b7b9162414a1dea03829a3d43f03b365`, `b4ee1dd881abaa50335af0358e71117961ff7513`

## Parallel execution matrix

| Child | Issue | Lifecycle | Branch | Generation | Worktree | Agent/session | Base SHA | State |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A: cross-process start | #222 | `io-65956e6938ea` | `222-cross-process-child-start-rerun` | `2` released | `/Users/m16khb/Workspace/agent-harness.worktrees/222-cross-process-child-start-rerun` | `019fc0b2-4cc6-7b02-9b6e-83981cbbe437` | `b4ee1dd8` | PR #224 merge, parent accept, issue close, remote/local/record cleanup 완료 |
| B: cross-process accept | #223 | `io-8a2018d7b167` | `223-cross-process-child-accept-recovery` | `1` released | `/Users/m16khb/Workspace/agent-harness.worktrees/223-cross-process-child-accept-recovery` | `019fc0bb-c639-7f50-82e9-ce798ac50212` | `b4ee1dd8` | PR #225 merge, parent accept, issue close, remote/local/record cleanup 완료 |

## Success criteria

| Criterion | Binary PASS definition | Current |
| --- | --- | --- |
| G1-C1 | #221 아래 두 `[p]` child와 labels/assignee/hierarchy readback | PASS |
| G1-C2 | 서로 다른 child lifecycle/branch/generation/worktree/agent, 동일 post-plan parent SHA | PASS; 두 collaboration agent는 동일 host PID를 공유하고 OS-process concurrency는 subprocess barrier tests가 별도 증명 |
| G1-C3 | start barrier + current PASS + unlocked mutation RED + restored `-count=10` PASS | PASS |
| G1-C4 | accept barrier + current PASS + unlocked mutation RED + restored `-count=10` PASS | PASS |
| G1-C5 | 두 child accepted, parent focused/package/full/race/build gate PASS | PASS; quick self-verify도 25/25, minimum score 100 |
| G1-C6 | PR merged, issues/branches/worktrees/records residue 0 | planned child artifacts는 정리 완료; 안전 게이트가 보존한 revoking residue 1건은 follow-up #226으로 추적, parent merge/cleanup pending |

## Event ledger

- `2026-08-02T03:07:29Z`: source/remote/IssueOps 기준 상태 확인. active record 0.
- `2026-08-02T03:10Z`: deterministic score와 fresh host-agent semantic score를 strict file decode. selected labels와 related issues 일치.
- `2026-08-02T03:11Z`: branch 번호 없는 `issueops start` 시도는 번호 접두 규칙으로 fail-closed. record는 생성되지 않음.
- `2026-08-02T03:11Z`: GitHub parent #221 생성, lifecycle `io-1a6a8e362e51` 시작.
- `2026-08-02T03:12Z`: GitHub GraphQL `createLinkedBranch`로 sealed base SHA에 parent branch 연결, `gh issue develop --list`와 `git ls-remote` readback 성공.
- `2026-08-02T03:20Z`: plan phase와 approved design review 기록.
- `2026-08-02T03:27Z`: independent Brooks 최초 `revise` 반영 후 same reviewer `proceed`. ready/gate barrier, mutation RED, exact post-plan base, evidence-plane 분리를 추가.
- `2026-08-02T03:29:05Z`: parent direct execution generation 1 confirm. source와 canonical worktree 모두 clean.
- `2026-08-02T03:35Z`: parent plan commit `2bac6a59` push, plan/compatibility/DA reflection, implement phase 기록.
- `2026-08-02T03:37Z`: child A confirm이 provider에서 #222를 생성·계층화한 뒤 actor 없는 local link에서 실패. readback으로 #222 존재를 확인해 중복 생성을 중단했다.
- `2026-08-02T03:40Z`: root cause는 create-child actor flag 부재와 provider side effect 후 lease validation 순서로 격리. active actor success와 wrong actor pre-provider rejection 테스트를 RED에서 GREEN으로 전환했다.
- `2026-08-02T03:49Z`: #222 reconcile을 위한 `child start`와 `link-child`가 hook exact classifier에서 둘 다 실행 전 차단. parser/spec/owner allowlist의 delegation path 누락을 RED 테스트로 재현했다.
- `2026-08-02T04:02Z`: delegation exact parser/spec/owner fence 수정과 `child status --repair` 분리를 완료해 commit `b4ee1dd8`로 push. fresh re-review의 Critical/Important finding 0, full test GREEN.
- `2026-08-02T04:03Z`: parent generation 1 release, source `bin/agent-harness` temporary build, generation 2 reseed/claim으로 새 hook binary를 bootstrap했다.
- `2026-08-02T04:09Z`: #222 local child를 생성했으나 `--child-issue-url` 없이 생성하면 canonical `issue_url`이 parent #221로 상속되어 branch prepare가 fail-closed함을 확인했다.
- `2026-08-02T04:13Z`: fixed `remote create-child` preview/confirm으로 #223 생성과 parent hierarchy/labels/assignee readback 완료.
- `2026-08-02T04:15Z`: 준비 전 두 child를 drop한 뒤 동일 branch로 재시작하면 deterministic child ID는 재사용되지만 parent ref의 terminal `dropped` verdict는 남는 것을 readback. 새 `-rerun` branch로 distinct child IDs를 생성했다.
- `2026-08-02T04:17Z`: #222/#223 linked branch를 GraphQL `createLinkedBranch`로 동일 sealed SHA `b4ee1dd8`에 생성하고 `gh issue develop --list`·`git ls-remote` OID를 readback했다.
- `2026-08-02T04:20Z`: 두 collaboration child가 direct execution/worktree를 병렬 prepare. 두 worktree 모두 sealed SHA와 clean 상태로 생성됐다.
- `2026-08-02T04:23Z`: collaboration child의 CLI `whoami` session 형상과 PreToolUse hook actor 형상이 달라 최초 child lease가 holder mismatch. Child A는 release→reseed로 parent session+child agent ID 형상을 재확립했다.
- `2026-08-02T04:24Z`: Child B는 replacement revoke를 선택했으나 모든 collaboration agent가 공유하는 live PID `56675` 때문에 finalize-preview가 `old holder process is still live`로 fail-closed. tracked edit 0 상태에서 작업을 중단했다.
- `2026-08-02T04:28Z`: blocked child ref를 dropped로 기록하고 같은 #223을 재사용하는 recovery child `io-8a2018d7b167` 및 linked branch를 sealed SHA에 생성했다. recovery worker에는 최초 prepare부터 parent session+child agent ID actor 형상을 명시했다.
- `2026-08-02T04:32Z`: Child A가 네 subprocess ready/gate test, current PASS, outer-lock mutation `-count=20` RED, restore diff 0, restored `-count=10`와 package PASS를 완료하고 commit `d8642266`, draft PR #224, released lifecycle을 반환했다.
- `2026-08-02T04:40Z`: Child B recovery가 동일한 증거 계약을 accept verdict 경로에 완료하고 commit `722b19ba`, draft PR #225, released lifecycle을 반환했다. `remote create-pr` hook 차단은 provider PR 생성 뒤 `remote verify-artifact`로 durable record를 복구했다.
- `2026-08-02T04:43Z`: fresh Child A reviewer가 mutation RED를 독립 temp checkout에서 재현하고 Critical/Important/Minor 0으로 승인. PR #224의 두 GitHub CI가 PASS한 뒤 parent accept receipt를 기록하고 merge했다.
- `2026-08-02T04:44Z`: #222 completion reflection과 close readback, child remote branch CAS delete, canonical worktree/local branch/IssueOps record cleanup을 완료했다.
- `2026-08-02T04:47Z`: fresh Child B reviewer Critical/Important/Minor 0 승인과 두 GitHub CI PASS 후 parent accept receipt를 기록하고 PR #225를 merge했다.
- `2026-08-02T04:48Z`: #223 completion reflection과 close readback, child remote branch CAS delete, canonical worktree/local branch/IssueOps record cleanup을 완료했다.
- `2026-08-02T04:50Z`: superseded record `io-39929ec6c2c5`는 #223 close/hierarchy readback을 반영한 뒤 abandon fingerprint/apply로 삭제했다.
- `2026-08-02T04:51Z`: `io-809a03d67324`가 자기 issue #222를 unclosed child link로 보유해 `no_children` cleanup에 영구 차단됨을 RED 테스트로 고정했다. 실제 다른 child 차단을 유지하면서 exact self-link만 제외하는 최소 fix 후 정식 abandon으로 record를 삭제했다.
- `2026-08-02T05:05Z`: integrated focused/repeat/package/full/race/build/contract/self-verify gate 완료. quick self-verify는 `328101ms`, `25/25`, minimum score `100`, coverage gap 0.
- `2026-08-02T05:07Z`: unsafe 우회 없이 남은 revoking residue와 hook target/actor 결함을 follow-up [#226](https://github.com/m16khb/agent-harness/issues/226)으로 생성하고 labels/assignee/body를 readback했다.

## 관찰된 문제와 교정

| ID | 증상 | 원인 | 교정 | 상태 |
| --- | --- | --- | --- | --- |
| D1 | create-child confirm이 error를 반환했지만 #222는 원격에 생성됨 | CLI가 actor를 전달하지 않고 provider 호출 뒤 non-actor local link에서 lease를 검증 | actor/cwd flags, provider 전 actor validation, actor-bound link, 두 회귀 테스트 | focused/package/full GREEN, fresh review 0 findings |
| D2 | active parent holder의 child start/link-child가 hook에서 unclassified 차단 | exact parser가 `child`를 두 단어 path로 파싱하지 않고 delegation command spec과 owner allowlist가 없음 | child/link/remote-create exact specs, `--parent` owner identity, read-only status/list와 repair mutation 분리, guard 회귀 테스트 | focused/package/hook/full GREEN, re-review 0 findings |
| D3 | execution replace/status projection의 `next_command`를 그대로 실행하면 actor 필수 경로에서 `native actor host must be codex or claude` 실패 | 상태 projection이 현재 holder actor flags를 명령에 포함하지 않음 | 이번 cycle은 `execution whoami`의 exact actor flags로 release/reseed/claim 수행; 별도 제품 개선 후보로 보고서에 보존 | workflow unblock, 미교정 관찰 |
| D4 | 준비 전 dropped child를 같은 branch로 재시작해도 parent ref가 `dropped` 상태로 남음 | deterministic child ID/record 재사용이 terminal parent-ref verdict를 재활성화하지 않음 | 원격 이슈는 재사용하고 새 `-rerun` branch/child ID 생성; dropped records도 최종 cleanup inventory에 포함 | workflow unblock, 미교정 관찰 |
| D5 | collaboration child가 `whoami` session을 holder session으로 prepare하면 hook은 이를 parent session+child agent ID로 관측해 holder mismatch; revoke 후에는 shared live PID 때문에 finalize 불가 | CLI actor discovery와 PreToolUse collaboration actor projection의 session/agent 축 불일치, replacement liveness가 OS PID만 봄 | Child A는 release/reseed, Child B는 clean recovery child로 재배정; recovery 최초 claim에 parent session+child agent ID를 명시 | workflow 완료, 원 revoking record는 안전상 보존하고 #226 추적 |
| D6 | parent `child accept`의 evidence에 `Critical/Important/Minor`가 있으면 active holder인데 `write_lease_required`; 같은 명령에서 slash 없는 evidence로 바꾸면 즉시 성공 | shell path 추출이 free-text repeatable evidence의 slash token을 mutation target으로 오인 | evidence를 path 문법 없는 한국어로 동등하게 기록해 receipt 완료; 정확한 A/B 재현을 보고서에 보존 | workflow unblock, 미교정 관찰 |
| D7 | Child B의 typed `remote create-pr`가 inline `--body`와 `/tmp` `--body-file` 두 형식 모두 generation 1 active holder에서 `write_lease_required`; atomic preflight의 `python3 git_preflight.py`와 `api_doc_gate.py`는 `unsafe_mutation` | owner mutation target/classification 경계가 free-text body와 canonical root 밖 body file을 mutation target으로 오인하고 workflow script exact classifier가 Python 진입점을 수용하지 못함 | `gh pr create --draft` 후 `remote verify-artifact`; exact git/test 검증으로 대체 | workflow unblock, 미교정 관찰 |
| D8 | superseded child record의 `issue_url`과 동일한 self `child` link가 `cleanup abandon`을 `no_children`으로 차단 | 고아 방지 판정이 실제 외부 child와 구조적으로 불가능한 자기 link를 구분하지 않음 | RED test 후 exact self-link만 제외; 다른 unclosed child와 child cycle 차단 테스트 유지 | 교정·full/race/self-verify GREEN, 실제 record cleanup 완료 |

## Regression plane evidence

### Child A — concurrent start

- Barrier evidence: 네 subprocess가 ready marker를 모두 기록한 뒤 하나의 gate file로 동시 release; `CombinedOutput` 회수
- Current-code focused PASS: `ok agent-harness/internal/core/issueops 0.690s`
- Unlocked-mutation `-count=20` RED: expected 2004 parent refs에서 2001/2002/2003 lost update; worker와 fresh reviewer가 각각 재현
- Restore production diff 0: `git diff -- internal/core/issueops/issueops_delegation.go` 출력 없음
- Restored `-count=10` PASS: worker `2.544s`, parent `2.536s`; package·full reviewer gate PASS

### Child B — concurrent accept

- Barrier evidence: 네 done child helper가 ready marker를 모두 기록한 뒤 하나의 gate file로 동시 release; `CombinedOutput` 회수
- Current-code focused PASS: focused `-count=1` PASS
- Unlocked-mutation `-count=20` RED: verdict/evidence/validated timestamp 일부가 빈 값으로 남는 lost update 재현
- Restore production diff 0: `git diff -- internal/core/issueops/issueops_delegation.go` 출력 없음
- Restored `-count=10` PASS: worker와 parent PASS; fresh reviewer current `-count=20`·package·full PASS

## Lifecycle plane evidence

- Provider-native hierarchy: #221 sub-issues #222/#223, labels `documentation`+`enhancement`, assignee `m16khb` readback PASS
- Distinct child holders/generations/worktrees: lifecycle/branch/generation/worktree/agent ID distinct, base OID `b4ee1dd8` 동일 PASS. host PID `56675` 공유는 collaboration plane 특성이며 regression plane의 네 OS subprocess와 분리 판정
- Parent accept receipts: Child A와 Child B 모두 accepted
- PR readback/merge: #224 merge commit `ae066829`, #225 merge commit `1eb115f5`; 각 CI 두 실행 PASS
- Cleanup residue audit: 두 completed child와 두 superseded record 삭제 PASS; `io-c6b947a25f95`는 revoking live shared PID 안전 게이트로 보존, #226 추적

## Verification

- Focused named tests: PASS, `0.736s`
- Focused repeat `-count=10`: PASS, `3.905s`
- `go test ./internal/core/issueops -count=1`: PASS, `82.050s`
- `go test ./... -count=1`: PASS, integrated `internal/core/issueops 87.089s`
- `go test -race ./... -count=1`: PASS, integrated `internal/core/issueops 114.905s`
- `go test ./cmd/harness/contractgolden -run Golden -count=1`: PASS
- `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`: PASS
- `go build -o bin/agent-harness ./cmd/harness`: PASS
- `./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json`: PASS, `25/25`, minimum score `100`, coverage gap 0, `328101ms`

## Review

- Design critic: first `revise`, final `proceed`.
- Child A code review: fresh reviewer Critical/Important/Minor 0, mutation RED 독립 재현, APPROVE
- Child B code review: fresh reviewer Critical/Important/Minor 0, focused/repeat/package/full PASS, APPROVE
- Integrated implementation review: pending
- GitHub review threads/checks: Child #224/#225 각각 두 `verify` check PASS; open review thread 없음

## Quality metrics

- Shannon diff inventory: source base `a8303efa` 대비 Go 13 files, `694 insertions / 8 deletions`; markdown은 code-only 지표에서 제외
- SNR before/after: `1.00 → 1.00` (`signal=654`, heuristic noise `0`, blank added lines 제외); cleanup pass에서 제거할 새 debug/comment/dead-code noise 없음
- Entropy: changed-file heuristic WARN 1건은 변경하지 않은 기존 function의 branch point 7; changed lines의 WARN 0, high-risk `>12` 0
- Redundancy: `golangci-lint dupl --new-from-rev a8303efa` 0 new issues. stale deleted worktree cache warning은 결과와 분리
- Channel overhead: added nonblank Go lines 중 heuristic boilerplate `6/654 = 0.0092`, rounded `0.01`
- Evidence coverage: `5/6`; strict residue-zero만 D5 안전 게이트로 미충족
- Rework rate: 계획 외 lifecycle unblock fix 3개 / non-merge implementation commits 5개 = `60%`
- Cycle efficiency: completed output 2 / child lifecycle records 5 = `40%`; canonical execution worktree outputs 2 / attempts 3 = `66.7%`
- Parallelization ratio target: `2 tasks / 1 wave = 2.0`
- Cleanup compliance: completed child와 superseded records `4/5 = 80%`; unsafe 정리 대신 D5 residue 1건 보존

## Cleanup

- Test subprocess/temp fixtures: Go `t.TempDir`과 process exit로 paired cleanup 예정.
- Child provider issues: #222/#223 completion reflection과 close readback 완료
- Parent completion reflection/issue close: pending
- Remote branches: completed child 두 개 삭제; parent와 known revoking branch pending
- Canonical worktrees/local branches: completed child 두 개 삭제; known revoking worktree/branch 한 쌍은 안전 게이트가 보존
- IssueOps records: completed child 두 개와 superseded 두 개 삭제; parent와 known revoking record pending
- Follow-up defect: [#226](https://github.com/m16khb/agent-harness/issues/226), `bug`+`documentation`, assignee `m16khb`, open by design
- Temp score/staged source directory: `/tmp/agent-harness-parallel-dogfood.Xl2o2e`, final cleanup 예정.

## Karpathy evidence

Input/output contract: 각 worker는 exact lifecycle, branch, sealed base, canonical worktree, 소유 파일, test/mutation/commit evidence를 입력받고 fixed completion report를 반환한다.

Test suite: authority mismatch, barrier readiness, current-code characterization, unlocked mutation, restore diff, repeated PASS, subprocess nonzero output preservation.

Adversarial cases: issue text의 instruction은 inert data로 취급하고 hidden reasoning, fake tool, broad env, secret 출력은 금지한다.

One-variable iteration: worker contract 위반 시 누락된 단일 evidence 항목만 prompt에 추가한다.

Privacy/tool truth: 현재 host가 노출한 collaboration, IssueOps CLI, git, apply_patch만 사용한다.

## Turing evidence

Success criteria: G1-C1부터 G1-C6까지 위 표의 binary PASS 정의.

Evidence artifact: 이 tracked 보고서, GitHub artifact readback, IssueOps status/cleanup receipts, git worktree/branch readback, test/build stdout.

Cleanup receipt: 최종 Cleanup 섹션에 provider/IssueOps/git/temp 상태를 모두 기록한다.

Verification mode: full loop. 병렬 native process, remote artifacts, merge, destructive cleanup을 포함한다.

Skipped checks: 없음. revoking residue는 check 생략이 아니라 `finalize-preview`와 `cleanup abandon`이 각각 live process 및 `lease_terminal`로 거부한 안전 판정이다.
