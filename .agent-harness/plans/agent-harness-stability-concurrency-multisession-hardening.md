# Agent Harness 동시성·멀티세션·유실·고아·품질 강화 계획

## TL;DR

> **기준점**: `main` / `88c29161f08c46b153278aa80e383e58e1198eaa` (2026-07-10)
>
> **실행 기준점**: T00 착수 시 `origin/main`은 `ec865def01a14d66eedff05c24657c908b400349`까지 전진했다. 추가된 `mcp_gateway` doctor probe는 live stateful gateway에 `initialize`를 보내므로, 진단 자체가 세션/FD를 누적하지 않는다는 계약을 T20에 포함한다.
>
> **결론**: P0는 확인되지 않았다. 그러나 현재 HEAD에는 실제 재현된 MCP connection-slot 고갈, `read_only` policy의 임의 실행·쓰기·workspace 밖 읽기 우회, worker secret 원문 저장, audit 중단 시 고아 프로세스 잔류가 있다. 소스 경로로 확정된 P1에는 PID 재사용 시 무관한 프로세스 종료, 병렬 세션이 sibling worktree를 편집하는 guard 허용, 재사용된 worktree 강제 삭제, state prune/migrate lost update, workpool/loop contract 덮어쓰기, install/update의 비원자 갱신 및 정상 세션 종료가 포함된다.
>
> **전략**: 공통 identity와 fail-closed 경계를 먼저 고정하고, 그 위에서 daemon·policy·IssueOps·state·workpool·install을 독립 worktree로 병렬 수정한다. 마지막에 host parity와 self-verify/stability-audit의 "검증이 실제로 검증하는가"를 교정한다. 전면 재작성은 하지 않는다.
>
> **Deliverables**: 25개 구현/검증 task, 7개 실행 wave, 3개의 명시적 설계 결정(daemon instance identity, canonical repo identity, dispatch freeze/epoch), runtime adversarial regression, CLI/MCP/host response-contract 갱신, 문서 reconciliation.
>
> **Critical path**: T00 → T01 → T02 → T03 → T19 → T20 → T21 → T22 → T23 → T24.
>
> **실행 계약**: 구현을 시작할 때는 `superpowers:subagent-driven-development`의 task별 구현/검토 루프를 사용하되, 이 저장소 규칙에 따라 모든 구현 child는 격리 worktree에서 실행하고 main agent만 integration을 수행한다.

## 1. 감사 범위와 증거 등급

### 1.1 확인한 표면

- 공통 runtime: daemon, MCP stdio proxy, PID/socket/lock, update/cleanup
- 상태: SQLite `sqlstore`, generic state, project lifecycle, IssueOps/session, workpool, loop, worker
- 멀티세션: repo/worktree identity, binding, resume, stale cleanup, parent/child pool gate
- 실행 경계: command policy, path containment, secret redaction, output budget, timeout/process tree, audit log
- 세 호스트: Codex, Claude Code, GJC install/update/hook/MCP/skill discovery
- 검증 체계: response contract, self-verify, stability-audit, doctor, race/crash/parallel isolation
- 프로젝트 문서: CONSTITUTION, ARCHITECTURE, CONVENTIONS, TESTING, CAUTIONS, ADR, OPERATIONS, AGENT_WORKFLOW, PROJECT_AUDIT, ISSUEOPS_AUDIT

### 1.2 증거 등급

| 등급 | 의미 | 이번 감사의 대표 항목 |
|---|---|---|
| R | 현재 HEAD runtime 재현 | MCP 64-slot 고갈 후 idle timeout 미회수, policy executable spoof/`sed -i`/`awk system`, bare symlink 외부 읽기, worker secret SQLite 유출, stability-audit 중단 후 고아 `self-verify` |
| S | 현재 HEAD의 완전한 제어 흐름으로 확정 | PID identity 없는 signal, sibling worktree allow, orphan path 재사용 삭제, state prune/migrate race, workpool/loop contract 덮어쓰기 |
| T | 테스트/진단 자체의 계약 결손 | self-verify false-positive, fake parallel isolation, sqlstore의 가짜 cross-process test, stability-audit의 live binary/daemon 오염 |
| A | 수용된 제약 또는 이론적 위험 | NFS `O_EXCL`, 48/64-bit truncated hash collision, `synchronous=NORMAL`의 전원 장애 시 최신 commit 유실 가능성 |

### 1.3 현재 baseline 검증

- `main == origin/main`, HEAD는 위 SHA다.
- 감사 시작 시 active IssueOps binding은 없었다: `issueops resume --repo "$PWD" --json` → `{"ok":false,"bound":false}`.
- focused 기존 테스트는 통과했다: `internal/core/{policy,state,workpool,looprun,sqlstore}`, `cmd/harness/{daemoncli,updatecli}`. 이는 아래 결함을 잡는 regression이 없다는 뜻이지 결함이 없다는 뜻이 아니다.
- stability-audit 중단으로 남은 감사 전용 temp process group은 정확한 temp 경로와 PID/PGID를 재검증한 뒤 종료했다. 실제 사용자 daemon이나 다른 프로세스는 건드리지 않았다.
- T00 실행 직전 upstream delta `88c2916..ec865de`를 재검토했다. `doctor`의 loopback MCP `initialize` probe가 응답 body만 닫고 stateful session 종료를 보장하지 않아, 반복 진단이 검사 대상의 FD 고갈을 악화시킬 수 있는 새 경계가 확인됐다.

## 2. 통합 Risk Register

중복 finding은 공통 원인 단위로 합쳤다. 괄호의 ID는 감사 중 사용한 원시 finding이다.

| Risk | 우선순위 | 증거 | 현상·영향 | 핵심 근거 |
|---|---|---|---|---|
| R01 daemon admission 복구 실패 | P1 | R | idle socket 64개가 timeout 뒤에도 slot을 점유하며 65번째 연결은 평문 오류와 exit 0을 받는다. status/doctor는 healthy로 오판한다. | `cmd/harness/daemoncli/daemon_server.go:14-27,101-185`, `daemon_proxy.go:30-70`, `daemon_status.go:10-35` (MCP-1/2) |
| R02 process identity 부재 | P1 | S | PID 파일이 숫자뿐이라 PID reuse 시 무관한 프로세스를 stop/update/cleanup이 종료할 수 있다. live PID+missing socket은 duplicate daemon도 허용한다. | `cmd/harness/daemoncli/daemonpaths/paths.go:45-66`, `daemon_status.go:38-66` (OWN-1, HI-02) |
| R03 graceful shutdown/update 부재 | P1 | R/S | SIGTERM handler가 없어 active MCP가 즉시 끊기며 server 30초/CLI 3초 timeout도 불일치한다. | `daemon_server.go:72-125`, `update_bootstrap_daemon.go:23-47` (SIG-1, HI-04) |
| R04 cleanup/refresh 범위 과대 | P1 | S | 정상 `agent-harness mcp` 세션과 dbhub/context7/kordoc 같은 외부 npx MCP까지 signal 후보가 된다. | `cmd/harness/updatecli/update_bootstrap_mcp.go:42-164,365-370` (CLEAN-1, HI-01) |
| R05 read-only policy 우회 | P1 | R | basename spoof executable, `sed -i`, `awk system()`, bare symlink operand로 arbitrary write/exec/outside read가 가능하다. | `internal/core/policy/policy_{command_classification,paths,run}.go` (SRB-01/02/03) |
| R06 secret·runner·audit 경계 | P1/P2 | R/S | worker가 policy 전에 raw argv를 DB/JSON에 저장한다. redactor가 split token/Bearer/JWT를 놓치고, 출력은 사후 truncate, timeout은 child tree를 남기며 실행 audit append가 없다. | `internal/core/worker/read_only.go:33-45`, `policy_run.go:61-116`, `audit/audit.go:39-53` (SRB-04~10) |
| R07 canonical repo identity 부재 | P1 | S | relative/absolute/symlink가 다른 cycle/binding/loop/pool namespace를 만들고 동시 start와 cleanup을 분리한다. | `internal/core/issueops/start/start.go:23-36`, `issueops/session/session.go:52-59,368-370`, `workpool/workpool.go:192-201`, `looprun/lifecycle.go:171-180` (MS-01) |
| R08 host session 소유권 부재 | P1 | S | 현재 session을 식별하지 않고 repo-wide active linked worktree면 허용한다. session A가 sibling B worktree를 편집해도 기존 테스트가 통과하도록 고정돼 있다. | `internal/core/lifecycle/lifecycle_worktree_guard.go:68-90,138-208`, `lifecycle_worktree_guard_linked_test.go:238-274` (MS-02) |
| R09 resume/binding 복구 오류 | P2 | S | stale/done primary binding이 live sibling 탐색을 가리고, `resume --id`는 실제 persistence 없이 `bound:true`; stored repo/cycle identity도 검증하지 않는다. | `internal/core/issueops/package.go:545-637`, `issueops/session/session.go:104-120,192-202` (MS-05/06, SS-05) |
| R10 stale/orphan cleanup 안전성 | P1/P2 | S | 과거 경로가 active worktree로 재사용돼도 `git worktree remove --force`; fresh 재분류가 skip해도 receipt는 released로 기록한다. | `internal/core/issueops/issueops_stale_scan.go:60-83,131-159` (MS-03/04) |
| R11 generic state lost update | P1 | S | prune/migrate/delete가 `StateWrite/Update` span 밖에서 snapshot/delete/write해 최신 checkpoint를 지우거나 덮을 수 있다. | `internal/core/state/state_{prune,migrate,io}.go` (SS-01) |
| R12 workpool contract/원자성 | P1 | S | 같은 deterministic ID pool을 무조건 덮어써 기존 task와 새 contract를 섞고, cross-repo parent를 허용하며 pilot task/pool 2-write가 원자적이지 않다. | `internal/core/workpool/workpool.go:25-79,92-139,174-189` (WP-1/2, SS-02) |
| R13 loop contract 혼선 | P1 | S | 같은 repo+name active loop면 goal/verify argv/max attempts 불일치도 조용히 기존 loop에 합류한다. | `internal/core/looprun/lifecycle.go:18-67` (LOOP-1) |
| R14 parent readiness TOCTOU | P2 | S | pool/loop scan과 IssueOps PR phase write가 다른 root/transaction이라 scan 직후 새 dispatch가 생겨도 PR 전이가 통과한다. | `internal/core/issueops_facade.go:265-318`, `workpool/gate.go:9-60`, `looprun/gate.go:14-44` (ORCH-1) |
| R15 compaction/project state 유실 | P2 | S | PostCompact compare 뒤 concurrent PreCompact가 새 capsule을 쓰면 새 파일을 삭제할 수 있고, existing profile RMW도 lock이 없어 metadata rollback이 가능하다. | `internal/core/lifecycle/compact/compact.go:125-175`, `lifecycle_project_state_store.go:52-119` (SS-03/06) |
| R16 SQLite 유지보수·privacy·crash test | P2/P3 | S/T | loop 및 project-scoped store maintenance 누락은 보완됐다. existing permissive mode는 즉시 교정되지 않고, nested span은 self-deadlock하며, 실제 process crash test는 없다. | `internal/core/state/state_maintain.go:18-82`, `sqlstore/sqlstore.go:70-151`, `sqlstore_test.go:96-140` (SS-04/07/08, QCT-08) |
| R17 install/update 비원자성 | P1/P2 | S | 전체 lock/rollback 없이 in-place write·remove/create를 하며 host 실패를 숨기고 GJC install을 중복 소유한다. crash/concurrent update가 mixed host state를 만든다. | `internal/core/install/install.go:80`, `internal/adapter/installutil/install_util.go:40-77`, `scripts/install-native.sh:95-135` (HI-03/08) |
| R18 standalone/host parity drift | P1/P2 | S | 기본 install이 외부 glab MCP를 등록하고, GJC hook은 모든 callback에서 `undefined`; live Claude collision 대신 fixture를 검사하며 managed stale link manifest가 없다. | `scripts/install-native.sh:135`, `gjc-plugin/hook.ts:19-40`, `validation_native_integration*.go` (HI-06/07/09/10) |
| R19 self-verify false-positive | P1/P2 | T | 실패 전용 필드가 contract hash에서 빠지고, clean tree에서는 race/vet 미실행도 covered, parallel isolation은 실제 self-verify를 돌리지 않으며 binary drift는 mtime만 본다. | `cmd/harness/selfworkflow/summary/self_verify_summary_contract.go`, `riskqa/*.go`, `validation_parallel_*.go`, `internal/core/doctor/checks.go:142-170` (QCT-01~04) |
| R20 stability-audit가 시스템을 오염/오판 | P2 | R/T | repo `bin/agent-harness`를 재빌드해 live symlink 대상을 바꾸고, interruption cleanup이 없어 실제 orphan을 남겼다. proxy/socket FD를 세지 않고 live user daemon RSS를 요구한다. | `skills/stability-audit/scripts/e2e_stability_audit.py:131-139,199-218,321-382` (SA-01~04, QCT-05~07) |
| R21 fuzz·문서·계약 drift | P3 | T | native `Fuzz*`가 0개인데 deterministic battery를 fuzz로 부르고, Go toolchain/audit 문서와 현재 코드가 어긋난다. | `cmd/harness/selfworkflow/steps/self_verify_steps.go:79`, `.agent-harness/TECH_STACK.md:28` (QCT-09, DOC-01) |
| R22 live doctor probe의 상태 누적 | P1/P2 | S | `mcp_gateway` 진단이 stateful streamable-HTTP endpoint에 `initialize`를 보낸 뒤 session teardown을 보장하지 않아, 반복 doctor가 gateway session/FD 고갈을 증폭할 수 있다. | `internal/core/doctor/checks.go:260-274` (execution-base delta) |

## 3. 해결됐거나 수용된 항목

다음은 재개방하지 않는다. 이 구분이 없으면 계획이 불필요한 재작성으로 팽창한다.

- SQLite 전환 전 JSON+flock의 non-Unix process gap, orphan `.lock`, 동일 absolute identity의 IssueOps start race는 해소됐다.
- workpool claim/heartbeat/reap의 stale-worker fencing, pool-size serialization, pilot 선행 gate는 현재 테스트와 소스에서 유효하다.
- stale cycle force-release 직전 fresh reclassification은 존재한다. 남은 문제는 worktree path identity와 receipt 정확성이다.
- 기본 install의 project-local no-write와 dry-run no-mutation 경계는 유지된다.
- daemon connection cap 64 자체는 존재한다. 문제는 idle 회수, admission error, health 관측이다.
- NFS/FUSE `O_EXCL`, truncated hash의 수학적 collision, SQLite `synchronous=NORMAL`의 전원 장애 특성은 이번 계획의 구현 bug로 승격하지 않는다.
- generic worker의 running cancellation 미지원은 새 capability다. T07의 timeout descendant cleanup과 혼동하지 않으며 별도 제품 결정 전에는 scheduler 기능을 확장하지 않는다.

## 4. 결정한 접근과 기각한 대안

| 선택지 | 판정 | 이유 |
|---|---|---|
| A. 위험 우선·호환 유지형 점진 강화 | **채택** | 현재 core/port 구조를 유지하면서 identity, transaction, fail-closed test를 좁게 추가할 수 있다. task별 rollback과 병렬 worktree가 가능하다. |
| B. 관측/테스트만 먼저 추가하고 production 수정 연기 | 기각 | MCP saturation과 policy arbitrary execution은 이미 runtime 재현된 P1이라 탐지만으로 안전 경계를 회복하지 못한다. 단, 각 task 안에서는 반드시 failing regression을 먼저 쓴다. |
| C. daemon/state/IssueOps를 단일 supervisor DB로 전면 재작성 | 기각 | 문제의 대부분은 누락된 identity·transaction·grammar다. 전면 재작성은 migration과 mixed-binary 위험을 키우며 이번 요청의 최소 변경 원칙에 반한다. |

## 5. 목표와 완료 정의

### 5.1 불변식

1. `read_only` 실행은 신뢰된 executable과 명시된 안전 grammar만 실행하며 workspace 밖 canonical path를 읽거나 쓰지 않는다.
2. 프로세스 signal은 PID만으로 보내지 않는다. instance nonce, start time, executable, protocol/build identity가 일치해야 한다.
3. daemon은 포화·draining·version mismatch를 machine-readable하게 보고하며 abandoned session은 bounded time 안에 slot을 반환한다.
4. 같은 물리 repo는 relative/absolute/symlink에서 하나의 canonical identity를 갖는다.
5. host session A는 A가 bind한 cycle/worktree만 mutate할 수 있다. ambiguity는 fail-closed다.
6. destructive cleanup은 fresh immutable identity를 재검증하며 재사용 경로나 dirty active worktree를 삭제하지 않는다.
7. multi-record invariant는 한 data transaction에서 commit되거나 아무것도 남지 않는다.
8. install/update는 같은 HOME에서 직렬화되고, crash 시 이전 generation 또는 완전한 새 generation 중 하나만 관측된다.
9. self-verify와 stability-audit는 claimed check를 실제 실행하며 사용자 binary/daemon/session을 오염하지 않는다.

### 5.2 Definition of Done

- 아래 T01~T23의 happy/failure QA가 모두 실제 temp HOME/state/socket/git repo에서 통과한다.
- `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, golden/response-contract, native host contract matrix가 통과한다.
- 두 실제 subprocess가 같은 SQLite/workpool/install 경계를 경합하고 holder kill 뒤 재획득되는 test가 통과한다.
- 두 실제 self-verify process가 격리 HOME/state/daemon에서 동시에 완료되고 descendant/process/socket/temp artifact가 0개 남는다.
- `./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json`이 실제 executed coverage를 보고한다.
- docs/audit reconciliation이 이전 "resolved" claim 중 재개방된 MCP shutdown/compact CAS를 현재 상태로 수정한다.
- 모든 task commit은 Conventional Commit subject + Lore body이며 task별 rollback이 가능하다.

## 6. 실행 Guardrails

- production 기능 수정은 반드시 isolated child worktree에서 한다. 여러 구현 agent가 같은 checkout을 공유하지 않는다.
- main agent만 parent IssueOps, workpool accept/reject, integration rebase/cherry-pick, 최종 verification을 소유한다.
- 3개 이상 fan-out은 pilot task 하나를 먼저 구현·검토·accept한 뒤 확장한다.
- 각 task는 failing test → 최소 구현 → focused race/test → self-review → spec review 순서다.
- sub-agent는 다른 sub-agent를 spawn하지 않는다. 전체 아키텍처 판단, task 경계 변경, cross-root schema 결정은 main agent가 한다.
- user HOME, 기본 daemon, Codex/Claude/GJC live config를 test fixture로 사용하지 않는다. `mktemp` HOME/state/socket과 temp git repo만 쓴다.
- PID/command-name만으로 kill하지 않는다. test-owned instance token과 process group을 확인한다.
- mixed-binary compatibility가 필요한 schema는 additive read-old/write-new로 배포하고 한 release 뒤 legacy read 제거 여부를 별도 결정한다.
- `git add -A`, unrelated cleanup/refactor, worker scheduler 확장, external tool 기능 복제는 금지한다.

## 7. Sub-agent 실행 모델

```text
Main agent
  ├─ parent IssueOps + integration worktree + workpool/loop
  ├─ pilot child: T01 daemon identity
  │    ├─ implementation agent (isolated worktree)
  │    ├─ fresh-context spec reviewer
  │    └─ fresh-context quality/security reviewer
  ├─ pilot accept 후 각 wave child tasks fan-out
  └─ wave barrier마다 targeted integration + conflict check
```

### 7.1 Task handoff contract

각 child prompt에는 다음을 반드시 넣는다.

- parent cycle ID, child cycle ID, exact worktree path/branch, task ID
- 허용 파일 목록과 금지 파일 목록
- 재현할 failure scenario와 acceptance command
- expected response-contract/schema 변경 여부
- evidence 경로 `.agent-harness/evidence/stability-hardening/Txx-*`
- "remote mutation/push/merge 금지, parent에게 commit SHA와 evidence만 제출"

### 7.2 Review contract

- Spec reviewer: task 요구사항 누락, 범위 확장, backward compatibility만 판정한다.
- Quality reviewer: race/TOCTOU/process cleanup/secret/golden/negative test만 판정한다.
- main agent는 두 리뷰를 현재 source로 검증한 뒤 accept/reject한다. agent의 자기 선언만으로 accepted 처리하지 않는다.

## 8. Wave와 의존성

| Wave | 병렬 lane | Task | 선행 조건 | Barrier |
|---|---|---|---|---|
| W0 | main only | T00 | 없음 | clean integration worktree, parent cycle, evidence baseline |
| W1 pilot | A | T01 | T00 | process identity contract accept |
| W1 fan-out | B/C/D/E | T05, T09, T13, T17 | T00 + T01 pilot accept | foundational tests pass |
| W2 | A/B/C/D | A: T02 ∥ T04; B: T06 → T07; C: T10 → T11 → T12; D: (T14 → T18) ∥ T15 | 각 task 표기 | 각 lane 내부 shared-file 순서와 P1 invariant suites pass |
| W3 | A/B/C | T03, T08, T16 | W2 관련 선행 | daemon rollout, audit, cross-root design accept |
| W4 | 단일 host lane | T19 → T20 → T21 | T01/T03 및 host 선행 | temp-HOME 3-host install/hook parity pass |
| W5 | 단일 verification lane | T22 → T23 | runtime/host 안정화 | self-verify/stability claims become executable evidence |
| W6 | A then main | T24 | T01~T23 | full verification, docs reconciliation, rollout decision |

### 8.1 주요 dependency

- T01 → T02, T03, T04, T19, T20, T22
- T05 → T06 → T07 → T08
- T09 → T10 → T11 → T12; T09 → T14, T15
- T10 + T14 + T15 → T16
- T13 + T14 → T18
- T02 + T03 + T19 + T21 → T22 → T23
- T01~T23 → T24

### 8.2 현실적 effort envelope

- P1 경계 복구(T00~T15): 약 18~28 engineer-days.
- cross-root/host/install hardening(T16~T21): 약 12~20 engineer-days.
- verification/audit/rollout(T22~T24): 약 7~12 engineer-days + 최소 48시간 dogfood soak.
- 총량은 약 37~60 engineer-days다. pilot 승인 뒤 4개 독립 lane을 유지해도 shared-file barrier와 full-race 비용 때문에 달력 기준 3주 미만 완료를 약속하지 않는다.

## 9. 실행 Tasks

### T00. Parent IssueOps와 격리 실행 기반 준비

- **What**: 현재 plan을 review/commit한 뒤 provider issue 번호를 prefix로 둔 `<issue-number>-stability-concurrency-multisession-hardening` branch에서 parent cycle을 시작하고 전용 integration worktree를 만든다. T01을 pilot로 둔 workpool을 생성하고 나머지 task는 pilot accept 전 claim 불가로 등록한다. 기준 HEAD, active processes, user daemon/config hash는 read-only snapshot만 남긴다.
- **Owner**: main agent only. **Depends**: 없음.
- **References**: `.agent-harness/AGENT_WORKFLOW.md`, `SUB_AGENT_PATTERNS.md`, `skills/issueops`, `internal/core/workpool/gate.go`.
- **Must not**: main checkout에서 implementation edit, user daemon restart, existing untracked 파일 stage, remote push.
- **Acceptance**: parent/child/workpool JSON에 `schema_version`, exact repo/worktree/branch가 있고 `prepare-tools`와 strict readiness가 통과한다.
- **QA**: happy—source checkout edit가 guard에 막히고 integration worktree edit만 허용; failure—다른 worktree path를 넘기면 block.
- **Commit**: plan 문서만 별도 `docs(plan): define stability and multisession hardening program`; 실행 state는 commit하지 않음.

### T01. Daemon instance identity와 protocol health 계약

- **What**: PID 숫자 파일을 additive instance record(`pid`, process start time, canonical executable, instance nonce, build SHA, protocol version, generation)로 교체한다. daemon initialize/status handshake가 같은 identity를 반환하고 stop/start/update는 socket handshake와 OS process identity가 모두 일치할 때만 signal한다. legacy integer PID는 status-only로 읽되 destructive action은 fail-closed한다.
- **Owner**: pilot deep agent. **Depends**: T00.
- **References**: `cmd/harness/daemoncli/daemonpaths/paths.go:45-66`, `daemon_status.go:10-66`, `daemon_start.go:43-76`, `daemon_server.go:72-103`.
- **Must not**: `/proc` 전제; PID-only fallback kill; daemon state를 SQLite로 옮기는 확대.
- **Acceptance**: PID reuse, live PID+missing socket, executable mismatch, nonce mismatch, build mismatch에서 signal 0회; exact identity에서만 stop 성공. CLI/MCP status에 protocol/build/generation이 동일하다.
- **QA**: happy—temp daemon start/status/stop round-trip; failure—PID file을 unrelated live child PID로 바꾼 fixture가 `instance_identity_mismatch`로 거부되고 child가 생존.
- **Commit**: `fix(daemon): bind lifecycle actions to verified instance identity`.

### T02. MCP admission, idle 회수, 구조화된 포화 상태

- **What**: SDK serve session을 cancellable하게 소유하고 idle expiry가 connection close와 slot release까지 보장되게 한다. admission 거부는 유효한 JSON-RPC error 또는 proxy stderr+nonzero로 변환한다. status/doctor에 `active_connections`, `max_connections`, `accepting`, `draining`을 추가한다.
- **Owner**: deep agent. **Depends**: T01.
- **References**: `daemon_server.go:14-27,101-185`, `daemon_proxy.go:30-70`, `internal/core/doctor/doctor.go:69-100`.
- **Must not**: cap 단순 증가; timeout을 test-only env에만 의존; 평문을 MCP stdout에 쓰기.
- **Acceptance**: 64 idle socket이 injected timeout 뒤 모두 회수되고 65번째 initialize가 성공한다. 포화 중 status는 `accepting:false`; proxy는 malformed stdout 없이 명시적 실패한다.
- **QA**: happy—actual Unix socket 64→timeout→re-entry; failure—65번째 initialize에서 JSON parser가 항상 성공하고 CLI exit가 0이 아님.
- **Commit**: `fix(mcp): recover idle daemon slots and expose admission health`.

### T03. Graceful shutdown과 generation handoff

- **What**: SIGTERM/SIGINT 또는 shutdown RPC가 listener close → admission drain → active session bounded drain/cancel → socket/PID cleanup 순서를 수행하게 한다. CLI와 server timeout을 하나의 상수/contract로 통일하고 proxy가 daemon generation 교체 시 bounded reconnect 또는 명시적 reconnect response를 제공한다.
- **Owner**: deep agent. **Depends**: T01, T02.
- **References**: `daemon_server.go:72-125`, `daemon_status.go:53-66`, `update_bootstrap_daemon.go:23-47`, `daemon_proxy.go:30-70`.
- **Must not**: 무제한 drain; active request silent drop; old/new daemon 동시 socket unlink.
- **Acceptance**: active MCP request 중 update를 실행해 request가 완료되거나 구조화된 retryable error를 받고, old PID/socket/goroutine이 사라지며 new generation만 reachable하다.
- **QA**: happy—subprocess active request + update; failure—hung client에서 deadline 후 descendant/socket이 0이고 강제 종료가 audit에 기록.
- **Commit**: `fix(daemon): drain active sessions across controlled generation handoff`.

### T04. Orphan proxy cleanup과 update refresh target 축소

- **What**: process snapshot에 PID/PPID/start time/executable/instance token/registered endpoint를 포함한다. `mcp cleanup --apply`는 dead-parent인 exact agent-harness proxy만 signal 직전 재검증해 종료한다. update refresh는 user `agent_harness` endpoint만 대상으로 하고 외부 npx MCP는 제외한다.
- **Owner**: deep agent. **Depends**: T01.
- **References**: `cmd/harness/updatecli/update_bootstrap_mcp.go:42-164,365-370`, `harnessapp/mcp_command_facade.go:21-46`, `.agent-harness/CAUTIONS.md` orphan proxy contract.
- **Must not**: command substring만으로 signal; live parent proxy 종료; dbhub/context7/kordoc cleanup.
- **Acceptance**: dead-parent exact proxy만 `terminated`; live parent, PID/PPID reuse, external MCP, current process는 `skipped` reason을 갖고 생존한다.
- **QA**: happy—test-owned orphan proxy cleanup; failure—mixed process table에서 외부 marker processes 전부 생존.
- **Commit**: `fix(update): target only verified orphan harness proxies`.

### T05. Read-only executable identity·command grammar·path containment

- **What**: command별 trusted executable resolver와 안전 argv grammar를 도입한다. separator가 있는 `argv[0]`, workspace/PATH shadow executable, executable symlink를 거부한다. `sed -i`, `awk system/getline pipe`, mutating git/go flags를 deny하고 positional/flag file operand를 cwd 기준 `EvalSymlinks` 후 workspace containment 검사한다.
- **Owner**: deep security agent. **Depends**: T00; W1 pilot accept 후 착수.
- **References**: `internal/core/policy/policy_command_classification.go:8-55`, `policy_catalog.go:33-38`, `policy_paths.go:51-99`, `policy_run.go:63-69`.
- **Must not**: basename allowlist 유지; 모든 arg를 path로 오인; shell parser 전체 구현.
- **Acceptance**: fake `cat`, PATH shadow, symlink executable, `sed -i`, `awk system`, bare/nested symlink operand가 모두 deny되고 marker/outside sentinel이 없다. 기존 안전 `git status`, `rg`, `cat`은 허용된다.
- **QA**: happy—command별 safe grammar table; failure—감사에서 사용한 5개 exploit fixture 재실행 시 side effect 0.
- **Commit**: `fix(policy): enforce trusted executables and read-only command grammars`.

### T06. Secret redaction과 worker persistence 순서

- **What**: pair-aware argv redactor(`--token VALUE`), Bearer/JWT/provider prefix를 공통 redactor로 통합한다. worker는 policy 평가·redaction 후에만 job을 저장하고 public `Command`에는 redacted argv만 둔다. CLI/legacy MCP/SDK MCP/worker의 denial exit·`isError` 의미를 통일한다.
- **Owner**: deep agent. **Depends**: T05.
- **References**: `internal/core/worker/read_only.go:33-45`, `worker/worker.go:20-35`, `policy/policy_env_redaction.go:55-68`, `cmd/harness/mcpcli/mcp_tool_policy_state.go`.
- **Must not**: raw secret를 test log/fixture에 하드코딩; redaction을 response 직전에만 수행.
- **Acceptance**: synthetic secret가 stdout/stderr/status/list/SQLite/WAL/audit/MCP 어디에도 byte sequence로 존재하지 않는다. denied는 CLI/worker exit 3, MCP `isError:true`로 일치한다.
- **QA**: happy—assignment/split/Bearer/JWT/provider matrix; failure—DB/WAL raw byte scan count 0.
- **Commit**: `fix(worker): redact command data before persistence and align denials`.

### T07. Bounded output과 subprocess-tree 소유권

- **What**: policy runner와 commandstep에 drain을 계속하면서 bounded head/tail와 total byte count만 저장하는 writer를 쓴다. Unix process group + bounded `WaitDelay`로 timeout/interruption 시 descendants를 종료하고 platform adapter로 분리한다.
- **Owner**: deep agent. **Depends**: T06.
- **References**: `internal/core/policy/policy_run.go:61-116`, `cmd/harness/commandstep/run.go:22-37`.
- **Must not**: pipe를 조기 close해 child를 의도치 않게 SIGPIPE; unbounded `bytes.Buffer`; Unix syscall을 공통 파일에 노출.
- **Acceptance**: budget 50배 출력에서도 retained capacity가 상한 내이고 total/truncated metadata가 정확하다. background child는 timeout 후 marker를 쓰지 못하며 PID/FD가 남지 않는다.
- **QA**: happy—large stdout/stderr both drained; failure—grandchild+inherited pipe fixture timeout 후 process tree 0.
- **Commit**: `fix(runner): bound captured output and terminate descendant processes`.

### T08. 실행 audit의 내구성·권한·rotation

- **What**: 실행 전 decision과 실행 후 exit/timeout/redacted byte metadata를 같은 audit ID로 append한다. audit append 실패 시 실행 전 failure는 fail-closed, 실행 후 failure는 result의 explicit audit error로 정의한다. parent 0700/file 0600을 재보증하고 bounded rotation을 추가한다.
- **Owner**: deep agent. **Depends**: T06, T07.
- **References**: `internal/core/policy/policy_run.go:35-94`, `internal/core/audit/audit.go:39-59`, CONSTITUTION command execution invariant.
- **Must not**: raw output/argv audit; silent audit failure; unbounded JSONL.
- **Acceptance**: success/denial/timeout 각각 decision+completion pair가 정확히 1개씩 있고 secret 0, permissive preexisting path가 즉시 private mode로 교정되며 rotation 후 latest records가 유지된다.
- **QA**: happy—three outcome audit correlation; failure—unwritable audit root에서 command 미실행과 명시적 error.
- **Commit**: `fix(audit): make command execution records durable private and bounded`.

### T09. Canonical physical repository identity와 legacy 호환

- **What**: `Abs + Clean + EvalSymlinks` 및 git common-dir/fingerprint를 사용하는 공통 `repopath.Identity`를 만든다. IssueOps, session key, active/resume, workpool, loop, lifecycle 비교가 동일 helper를 사용한다. 기존 absolute/raw ID는 read alias로만 한 release 지원하고 신규 write는 canonical ID만 쓴다.
- **Owner**: main agent가 identity/compatibility 결정을 고정하고, isolated deep agent가 그 결정만 구현. **Depends**: T00; W1 pilot accept 후 착수.
- **References**: `internal/core/repopath/path.go`, `issueops/start/start.go:23-36`, `issueops/package.go:212-233`, `issueops/session/session.go:368-374`, `workpool/workpool.go:192-201`, `looprun/lifecycle.go:171-180`.
- **Must not**: git repo가 아닌 workspace를 무조건 거부; silent duplicate migration; ID 길이/format을 즉시 깨기.
- **Acceptance**: relative/absolute/symlink/worktree path가 같은 physical repo ID/fingerprint를 얻는다. 동시 start는 한 record만 만들고 legacy record는 발견되지만 새 duplicate write가 없다.
- **QA**: happy—temp git repo alias matrix; failure—다른 git common-dir은 경로가 유사해도 identity mismatch.
- **Commit**: `fix(identity): canonicalize repository ownership across harness state`.

### T10. Session-aware binding과 worktree guard fail-closed

- **What**: 먼저 설치된 Codex/Claude/GJC hook input 구현에서 stable session identifier의 실제 제공 여부와 전 lifecycle 전달 여부를 검증한다. 세 host에서 안정적으로 전달되는 ID가 있을 때만 optional `(host, session ID)` binding key를 추가한다. 공통 최소 안전책은 host ID와 무관하게 exact expected-worktree, current cwd/branch binding, target owner를 대조하고 active worktree가 복수인데 exact owner가 없으면 ambiguity block하는 것이다. repo-wide "any linked worktree" allow fallback과 그 테스트를 제거한다.
- **Owner**: deep agent. **Depends**: T09.
- **References**: `internal/core/issueops/session/session.go`, `internal/core/lifecycle/lifecycle_worktree_guard.go:68-90,138-208`, `lifecycle_worktree_guard_linked_test.go:238-274`, hook adapters.
- **Must not**: 존재하지 않는 공통 host session ID를 발명; 환경변수 하나만 유일한 truth로 사용; session ID 없는 host에서 임의 cycle 선택; single-cycle recovery 회귀.
- **Acceptance**: 동시 session A/B는 자기 worktree만 mutate한다. compaction/env loss 후 source checkout resume에서도 명시 bind 전 sibling edit는 block; single unambiguous cycle은 기존 recovery를 유지한다.
- **QA**: happy—두 host-session/두 branch matrix; failure—session ID missing+2 cycles에서 actionable ambiguity JSON.
- **Commit**: `fix(issueops): scope worktree ownership to the active host session`.

### T11. Resume read-repair와 binding identity/JSON semantics

- **What**: binding read 시 stored repo/cycle/schema를 요청 key와 대조해 fail-closed한다. missing/done primary binding은 locked compare-delete 후 live candidate 탐색을 계속한다. `selected`와 persisted `bound`를 분리해 `--bind` 성공 후에만 `bound:true`를 반환한다.
- **Owner**: focused agent. **Depends**: T09, T10.
- **References**: `internal/core/issueops/package.go:545-637`, `issueops/session/session.go:96-120,190-202,333-365`, CLI resume handler.
- **Must not**: concurrent rebind 삭제; done cycle 자동 부활; schema migration 확대.
- **Acceptance**: ghost/done binding+live sibling에서 live ID를 제안하고 stale row를 안전 정리한다. concurrent rebind는 보존된다. `resume --id`/`--bind` JSON 의미가 정확하다.
- **QA**: happy—read-repair round-trip; failure—repo/cycle mismatch row가 error이며 guard allow로 전환되지 않음.
- **Commit**: `fix(issueops): repair stale bindings and report persisted resume state`.

### T12. Stale/orphan worktree 제거의 immutable identity 검증

- **What**: orphan breadcrumb에 original gitdir/common-dir/branch/fingerprint를 저장한다. apply 직전 해당 identity와 다른 non-done record ownership, dirty 상태를 재검증하고 불일치는 `needs-review`로만 보고한다. 실제 release 성공일 때만 receipt `released`에 append한다.
- **Owner**: deep agent. **Depends**: T09, T11.
- **References**: `internal/core/issueops/issueops_stale_scan.go:60-83,131-159`, `issueops_force_release.go:48-49`.
- **Must not**: path existence만으로 `git worktree remove --force`; dirty active worktree 자동 삭제; snapshot receipt 신뢰.
- **Acceptance**: old orphan path를 새 active branch/worktree가 재사용해도 파일·worktree가 보존되고 `needs-review`; unchanged confirmed orphan만 제거된다. live reclassification skip은 `released`에 없다.
- **QA**: happy—identity-matched clean orphan cleanup; failure—reused dirty active path 보존 및 byte hash 동일.
- **Commit**: `fix(issueops): verify immutable worktree identity before orphan cleanup`.

### T13. Generic state prune/migrate/delete 직렬화

- **What**: `StatePrune`, `StatePrunePrefix`, `StateMigrate`, `StateDelete`가 key/root span 안에서 fresh read/revalidate 후 destructive mutation하도록 바꾼다. snapshot은 후보 탐색에만 쓰고 결정은 lock 안에서 한다.
- **Owner**: deep agent. **Depends**: T00; W1 pilot accept 후 착수.
- **References**: `internal/core/state/state_prune.go:30-130`, `state_migrate.go:23-66`, `state_io.go:177-195`.
- **Must not**: global process mutex로 SQLite cross-process lock 대체; 모든 state API transaction 재설계.
- **Acceptance**: barrier 기반 prune-vs-refresh, migrate-vs-update, delete-vs-update에서 최신 write가 유실되지 않는다. actual two-process test에서도 동일하다.
- **QA**: happy—stale untouched record만 prune; failure—snapshot 후 refreshed record가 보존되고 result count도 정확.
- **Commit**: `fix(state): serialize destructive operations with concurrent updates`.

### T14. Workpool create/parent/pilot atomic invariant

- **What**: deterministic pool ID가 이미 있으면 exact contract의 explicit resume만 허용하고 그 외 `pool_exists/contract_mismatch`; pool repo와 parent cycle canonical repo를 대조한다. pilot task row와 pool `PilotTaskID`는 좁은 sqlstore data transaction/batch로 한 번에 commit한다.
- **Owner**: deep agent. **Depends**: T09, T13.
- **References**: `internal/core/workpool/workpool.go:25-79,92-139,174-189`, `sqlstore/sqlstore.go:143-171`.
- **Must not**: 모든 `WithSpan`을 data transaction으로 전환; closed pool 자동 재개방; cross-repo parent 허용.
- **Acceptance**: active/closed/task-bearing pool recreation은 기존 row/task를 변경하지 않는다. cross-repo parent 거부. second-write fault에서 task와 pool 어느 쪽에도 partial state가 없다.
- **QA**: happy—exact explicit resume; failure—SQLite trigger/fault injection 후 orphan pilot task 0.
- **Commit**: `fix(workpool): preserve pool contracts and atomically assign pilots`.

### T15. Loop start contract idempotency

- **What**: active loop resume 시 normalized `goal`, `verify_argv`, `max_attempts`, repo/name contract를 모두 비교한다. exact match만 idempotent success, 불일치는 `loop_contract_mismatch`와 필드 diff를 반환한다. explicit resume-by-ID가 필요하면 CLI/MCP에 additive하게 둔다.
- **Owner**: focused agent. **Depends**: T09.
- **References**: `internal/core/looprun/lifecycle.go:18-67`, loop CLI/MCP facade/goldens.
- **Must not**: 기존 active loop overwrite; terminal loop 자동 재개; secret 원문 diff.
- **Acceptance**: 각 필드 불일치와 두 subprocess 동시 start에서 한 contract만 승리하며 다른 요청은 mismatch. exact duplicate만 같은 ID를 반환한다.
- **QA**: happy—exact retry; failure—goal/argv/max 각각 mismatch table test.
- **Commit**: `fix(loop): reject incompatible active-loop resumes`.

### T16. Parent dispatch freeze/epoch로 PR gate 원자화

- **What**: 먼저 ADR에서 parent cycle의 dispatch epoch/frozen 상태와 pool/loop create 권한을 결정한다. PR transition은 parent를 freeze하고 같은 invariant/epoch에 연결된 open pool/loop가 0임을 검증한 뒤 commit한다. freeze 뒤 create는 거부하거나 명시 unfreeze가 필요하다. enforcement 필드는 IssueOps schema를 bump해 구 binary가 future record를 fail-safe로 거부하도록 하고, coordinated binary generation 전에는 새 schema write를 활성화하지 않는다.
- **Owner**: architecture/deep agent; Brooks design review 필수. **Depends**: T10, T14, T15.
- **References**: `internal/core/issueops_facade.go:265-318`, `workpool/gate.go:9-60`, `looprun/gate.go:14-44`, ADR/IssueOps schema.
- **Must not**: 단순 두 번 scan으로 TOCTOU를 해결했다고 주장; 세 DB를 분산 transaction으로 가장; silent backward incompatibility.
- **Acceptance**: barrier에서 gate scan 직후 concurrent create를 시도해 create 또는 PR transition 중 정확히 하나만 성공한다. old record는 unfrozen epoch 0으로 읽고 future schema는 구 binary와 신 binary 모두에서 fail-closed된다. mixed-generation fixture가 freeze를 우회하지 못한다.
- **QA**: happy—closed dispatch set 후 PR; failure—freeze-vs-create 1,000회 race에서 invariant 위반 0.
- **Commit**: `docs(adr): define parent dispatch freeze semantics` 후 `fix(issueops): make dispatch creation and PR readiness mutually exclusive` 두 커밋.

### T17. Compaction capsule과 project profile RMW 잠금

- **What**: `read capsule → nonce/created_at compare → remove` 전체를 project `compact-capsule` key span에 넣는다. existing profile read/merge/write도 project span 안에서 fresh read한다. queue append와의 lock order를 문서화해 nested span을 피한다.
- **Owner**: deep agent. **Depends**: T00; W1 pilot accept 후 착수.
- **References**: `internal/core/lifecycle/compact/compact.go:125-175`, `lifecycle_project_state_store.go:52-119`, `lifecycle/docupkeep/store.go`.
- **Must not**: timestamp-only CAS로 회귀; project 전체를 단일 long-held lock; capsule을 repo source에 저장.
- **Acceptance**: compare barrier 뒤 concurrent PreCompact의 새 nonce capsule이 보존된다. stale profile writer가 newer metadata를 되돌리지 못한다.
- **QA**: happy—PreCompact/PostCompact normal consume; failure—interleaving write와 concurrent bootstrap/draftwiki init 1,000회에서 context/metadata loss 0.
- **Commit**: `fix(lifecycle): lock compact consume and project profile updates`.

### T18. SQLite maintenance·privacy·process-crash 계약

- **What**: loop root와 existing direct project store discovery는 maintenance catalog에 반영됐다. 남은 범위로 store open 직후 owned root 0700, DB/WAL/SHM/lock 0600을 보증한다. actual helper process로 acquire/block/kill/reacquire를 검증한다. nested `WithSpan`은 감지해 fail-fast error를 내고 busy wait는 context-aware로 만든다. process-lifetime handle cache는 FD-growth probe만 추가하고, 실제 상한 위반이 측정될 때에만 별도 task로 eviction/close 설계를 연다.
- **Owner**: deep agent. **Depends**: T13, T14.
- **References**: `internal/core/state/state_maintain.go:18-35`, `sqlstore/sqlstore.go:51-151`, `sqlstore/maintain.go:37-55`, `sqlstore_test.go:96-140`.
- **Must not**: scheduler timing 200ms만으로 lock test; nested span 무기한 hang 유지; unrelated root chmod.
- **Acceptance**: loop 및 existing project WAL maintenance와 lifecycle-only project non-materialization은 retained tests로 확인됐다. 남은 acceptance는 two-process holder kill 뒤 bounded time 재획득, nested span 즉시 typed error, cancelled wait 조기 반환, permissive pre-created root 교정이다. 반복 open의 FD 수는 측정·기록되며 근거 없이 cache semantics를 바꾸지 않는다.
- **QA**: happy—cross-process serialization/crash recovery; failure—nested/cancel/busy/permission table.
- **Commit**: `fix(sqlstore): harden process locking maintenance and private modes`.

### T19. Install/update transaction과 단일 단계 소유권

- **What**: HOME/install root 단위 lock, staged temp+fsync+rename, symlink atomic replace, build temp rename, generation receipt를 추가한다. harness-owned 파일은 prepare가 모두 성공한 뒤 commit하고 실패 시 이전 파일로 복원한다. 외부 Codex/Claude/GJC CLI side effect는 전역 transaction으로 가장하지 않고 각 adapter 한 곳만 소유하는 idempotent step으로 실행하며 `absent=skipped`, `command failed=recoverable_partial`과 정확한 rerun/rollback recipe를 반환한다. daemon refresh는 모든 필수 step 성공 뒤 T03 handoff로만 수행한다.
- **Owner**: deep agent. **Depends**: T01, T03.
- **References**: `internal/core/install/install.go:80`, `internal/adapter/installutil/install_util.go:40-77`, `cmd/harness/installcli/install_native_path.go:90`, `scripts/install-native.sh:95-135`.
- **Must not**: in-place config truncate; symlink remove 후 create gap; `|| true`로 실제 host 실패 숨김.
- **Acceptance**: harness-owned 파일의 단계별 fault injection에서는 old/new generation 중 하나만 complete하다. 외부 host CLI 실패는 성공으로 보고되지 않고 daemon refresh 전 멈추며 idempotent rerun으로 수렴한다. 두 process concurrent install에서 duplicate PATH와 missing link가 없다.
- **QA**: happy—temp HOME 3-host fake CLI install/update; failure—각 rename/adapter/daemon refresh 지점 crash matrix 후 invariant scan.
- **Commit**: `fix(install): make native host updates atomic and recoverable`.

### T20. Standalone scope·managed link·비누적 live host 진단

- **What**: native install에서 implicit `sync-glab-mcp.sh`를 제거하고 별도 explicit command로 둔다. harness-owned link manifest로 removed/broken/wrong-root links를 diagnose/선택 cleanup한다. doctor/self-verify는 fixture가 아니라 read-only live config/CLI readback으로 user/project endpoint collision, installed build SHA, protocol generation을 검사한다. live MCP reachability probe는 state를 만들지 않는 transport check를 우선하고, protocol `initialize`가 불가피하면 반환된 session ID를 정상 teardown해 반복 실행 전후 session/FD가 증가하지 않게 한다.
- **Owner**: deep agent. **Depends**: T01, T19.
- **References**: `scripts/install-native.sh:135`, `internal/adapter/install_contract_matrix_test.go`, `validation_native_integration*.go`, `internal/core/doctor/checks.go:142-274`.
- **Must not**: 외부 glab 기능을 core에 복제; 명시 승인 없이 live config 수정; mtime을 build identity로 사용; 진단용 stateful MCP session을 남기기.
- **Acceptance**: default install은 agent-harness integration만 생성한다. stale managed link는 dry-run에서 정확히 보고되고 apply 전에는 보존. live no-conflict/conflict/dogfood-name/build mismatch fixture가 구분된다. stateful fake gateway에 doctor를 반복 실행해도 active session과 FD count가 baseline으로 복귀한다.
- **QA**: happy—temp HOME manifest update와 teardown 가능한 fake MCP gateway probe; failure—external MCP/config byte hash가 install 전후 동일하고, teardown 실패는 healthy로 오판하지 않으며 session/FD 증가가 없다.
- **Commit**: `fix(install): keep native setup standalone and diagnose managed host state`.

### T21. GJC와 공통 hook lifecycle parity

- **What**: 설치된 GJC HookAPI의 실제 input/blocking return schema를 binary/package source로 먼저 확정한다. `hook.ts`가 stdin/event JSON과 shared enforcement flags를 Go hook CLI에 전달하고 allow/block/ask/context/stop을 GJC 형식으로 반환한다. adapter `Host`에 GJC를 명시하고 SessionStart/PreToolUse/PostToolUse/PreCompact/PostCompact/Stop/UserPromptSubmit 계약을 세 host matrix로 고정한다.
- **Owner**: deep host-integration agent. **Depends**: T17, T20.
- **References**: `gjc-plugin/hook.ts:19-40`, `internal/adapter/hook/output.go:8-166`, hook CLI, `.agent-harness/operations/hosts.md`.
- **Must not**: Claude schema를 GJC에 추정 복사; 모든 callback `undefined`; host plugin에 core policy 복제.
- **Acceptance**: 같은 logical request가 세 host에서 의미상 동일 allow/block/ask/context/stop 결과를 낸다. compaction capsule과 pending upkeep context가 각 host restart/compact 후 한 번만 전달된다.
- **QA**: happy—7-event golden matrix; failure—unknown/malformed/timeout event가 host를 crash시키지 않고 fail-closed 또는 documented noop.
- **Commit**: `fix(gjc): enforce shared hook lifecycle decisions across hosts`.

### T22. Self-verify contract 진실성·실제 병렬 격리·build identity

- **What**: failure-only `failure_class`, `rerun_commands`를 conditional contract schema/hash에 포함한다. risk QA 결과를 `executed/skipped/not_applicable`로 구분하고 실제 race/vet command evidence만 coverage로 인정한다. parallel isolation은 격리 HOME/state/socket에서 실제 self-verify subprocess 2개를 실행한다. binary/daemon drift는 build SHA/protocol generation으로 판단한다.
- **Owner**: deep verification agent. **Depends**: T01, T02, T19, T21.
- **References**: `cmd/harness/selfworkflow/summary/self_verify_summary_contract.go`, `riskqa/risk_qa*.go`, `validation_parallel_*.go`, candidate catalog, response goldens.
- **Must not**: label 존재만으로 covered; dummy state probe를 parallel self-verify로 명명; skipped race를 OK executed로 표시.
- **Acceptance**: failure field 제거 fixture가 contract test를 실패시킨다. clean tree에서 race/vet 미실행은 covered가 아니며, 두 real process 결과·경로·daemon·cleanup이 격리된다.
- **QA**: happy—actual dual self-verify completes; failure—의도적 shared HOME/socket fixture가 isolation check를 실패시키고 rerun recipe 제공.
- **Commit**: `fix(self-verify): require executable evidence for claimed coverage`.

### T23. Stability-audit 자기격리·중단 정리·정확한 측정

- **What**: repo/live symlink 대상이 아닌 temp binary를 빌드하고 시작/끝 hash+inode invariant를 확인한다. 모든 subprocess를 process group으로 소유해 SIGINT/timeout/exception에서 TERM→bounded KILL→wait한다. proxy/socket FD/daemon capacity를 hygiene에 추가하고 RSS는 temp stress daemon을 측정한다. hook output schema는 T21의 single source를 사용하며 command별 timeout을 Go budget과 맞춘다.
- **Owner**: deep stability agent. **Depends**: T02, T03, T22.
- **References**: `skills/stability-audit/scripts/e2e_stability_audit.py:131-139,199-218,321-382`, skill tests.
- **Must not**: `ROOT/bin/agent-harness` overwrite; user daemon required; raw `subprocess.run(timeout=...)` 예외로 JSON report 유실.
- **Acceptance**: 정상·timeout·SIGINT 세 경로 모두 final JSON report를 내고 child/grandchild/socket/temp daemon이 0개다. repo binary hash/inode와 user daemon 상태는 전후 동일하다.
- **QA**: happy—full temp audit; failure—self-verify 중 SIGINT 주입 뒤 `ps/lsof` exact token scan 0, report에 structured interrupted status.
- **Commit**: `fix(stability-audit): isolate binaries and reap interrupted process trees`.

### T24. 회귀 매트릭스·native fuzz·문서 reconciliation·최종 rollout

- **What**: policy path/argv, JSON-RPC admission, versioned record decoding에 bounded Go fuzz target을 추가하고 deterministic battery와 명칭을 분리한다. 모든 task branch를 integration branch에 순서대로 통합한 뒤 focused→race→full→host smoke→stability audit 순으로 검증한다. PROJECT_AUDIT/ISSUEOPS_AUDIT/CAUTIONS/ADR/TESTING/TECH_STACK/OPERATIONS를 현재 결과로 갱신한다.
- **Owner**: main agent + fresh-context final reviewers. **Depends**: T01~T23.
- **References**: `.agent-harness/TESTING.md`, `PROJECT_AUDIT.md`, `ISSUEOPS_AUDIT.md`, `CAUTIONS.md`, `ADR.md`, response-contract golden.
- **Must not**: fuzz를 무제한 CI gate로 설정; golden을 무검토 대량 재생성; 실패한 verification을 문서로 덮기; 자동 push/merge.
- **Acceptance**: 아래 Final Verification Wave 전부 통과, diff-to-requirements traceability 100%, P1/P2 risk register가 resolved/accepted/deferred 근거와 함께 갱신된다.
- **QA**: happy—clean temp HOME/state에서 full program; failure—각 핵심 invariant에 mutant/negative fixture가 실제 gate를 실패시킴.
- **Commit**: `test(harness): add adversarial concurrency and boundary regressions`, `docs(harness): reconcile stability audit and operating contracts`로 분리.

## 10. Final Verification Wave

순서를 바꾸지 않는다. 앞 단계 실패 시 다음 단계로 진행하지 않는다.

1. **Diff/scope**
   - `git diff --check`
   - task별 허용 파일과 actual diff 교집합 검수
   - secret scan, generated binary/config/untracked artifact 확인
2. **Focused invariant suites**
   - daemon/MCP actual socket admission·shutdown·PID reuse
   - policy exploit matrix와 worker DB/WAL secret scan
   - IssueOps two-session/two-worktree, orphan path reuse
   - state/workpool/loop actual two-process races
   - compact/profile barrier tests, install fault matrix
   - repeated doctor MCP gateway probe의 session/FD 비누적
3. **Static and contracts**
   - `go vet ./...`
   - `go test ./cmd/harness -run Golden -count=1`
   - `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`
   - install/hook three-host golden matrix
4. **Race/full**
   - `go test ./... -count=1`
   - `go test -race ./... -count=1`
   - `go build -o <temp>/agent-harness ./cmd/harness`
5. **Runtime isolation**
   - actual dual self-verify with separate HOME/state/socket
   - `self-verify --full --iterations=10 --seed=100 --target-score=95`
   - interrupted stability-audit cleanup probe
6. **Process/file hygiene**
   - exact instance token 기준 child/grandchild/zombie/proxy/socket FD 0
   - repo binary, user config, user daemon hash/state 전후 동일
7. **Independent reviews**
   - fresh Brooks conceptual-integrity review
   - fresh verifier가 모든 acceptance/evidence/Not-tested를 대조
8. **Rollout decision**
   - local dogfood update 1회 → 세 host readback → soak
   - 이상 없을 때만 PR/merge/push를 사용자 결정으로 제시

## 11. Commit과 integration 순서

- 각 task는 최대 1개 production commit + 필요 시 선행 failing-test commit 1개다.
- T16은 ADR commit과 implementation commit을 분리한다.
- T24의 fuzz/test와 docs reconciliation도 분리한다.
- integration 순서: T01 → T05/T09/T13/T17 → T02/T04/T06/T07/T10/T11/T12/T14/T15/T18 → T03/T08/T16 → T19/T20/T21 → T22/T23 → T24.
- child는 push/merge하지 않고 commit SHA만 parent에 제출한다. main agent가 rebase/cherry-pick 전 source worktree와 target branch를 다시 확인한다.
- rollback은 task commit 단위 revert를 기본으로 하되, schema/generation task는 이전 binary가 new additive records를 읽는지 compatibility test를 먼저 확인한다.

## 12. 계획 밖으로 명시적으로 남기는 항목

- generic worker의 running job interactive cancellation/재개 scheduler
- NFS/FUSE에서의 daemon lock 강보장
- SQLite를 단일 distributed transaction/state service로 교체
- truncated hash 길이 확대 migration(실제 collision evidence 전까지)
- 외부 glab/dbhub/context7/kordoc 기능을 agent-harness core에 복제
- machine-wide kernel/process tuning

이 항목들은 현재 확인된 결함을 고치는 데 필요하지 않으며 별도 제품/아키텍처 결정 없이는 구현하지 않는다.
