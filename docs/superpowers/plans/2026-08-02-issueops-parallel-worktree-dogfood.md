# IssueOps Parallel Worktree Dogfood Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 두 delegated child를 별도 IssueOps direct execution·canonical worktree·Codex agent로 동시에 실행해 cross-process start/accept 경합 회귀를 추가하고 PR merge·cleanup까지 증명한다.

**Architecture:** parent lifecycle은 계획·provider hierarchy·통합·publication·cleanup을 소유한다. 두 child lifecycle은 parent의 같은 committed HEAD에서 시작하며 서로 다른 test file과 cross-process helper namespace만 소유한다. 현 production이 invariant를 만족하면 test-only로 끝내고, 실패하면 원인과 최소 production fix를 parent가 별도 결정한다.

**Tech Stack:** Go 1.26.3, testing, os/exec, IssueOps CLI/direct execution, Git/GitHub CLI.

## Global Constraints

- Parent issue: `https://github.com/m16khb-org/issueops/issues/221`
- Parent lifecycle: `io-1a6a8e362e51`
- Sealed source base: `a8303efad9e093dcd6e43b0ab2a1a9622ebade9b`
- 모든 원격 artifact는 한국어, `enhancement`·`documentation` labels, `m16khb` assignee를 포함한다.
- child는 자기 canonical worktree와 소유 파일만 수정하고 다른 agent의 변경을 되돌리지 않는다.
- `git add .`, `git commit -a`, force push, raw destructive cleanup을 사용하지 않는다.
- test helper subprocess 실패는 `CombinedOutput`으로 stdout/stderr와 exit error를 함께 보존한다.
- Regression plane과 lifecycle plane을 별도 판정하며 한쪽 성공으로 다른 쪽 실패를 덮지 않는다.
- 계획 commit 또는 실제 lifecycle unblocker fix 직후 standalone `git rev-parse HEAD`의 literal SHA가 두 child의 동일 sealed base다.

---

### Task 1: Parent plan worktree와 provider-native child graph 준비

**Files:**
- Create: `docs/superpowers/specs/2026-08-02-issueops-parallel-worktree-dogfood-design.md`
- Create: `docs/superpowers/plans/2026-08-02-issueops-parallel-worktree-dogfood.md`
- Create: `.issueops/operations/2026-08-02-issueops-parallel-worktree-dogfood.md`

**Interfaces:**
- Consumes: parent issue #221, lifecycle `io-1a6a8e362e51`, linked branch `221-issueops-parallel-worktree-dogfood`.
- Produces: approved design/plan artifacts, parent canonical worktree, committed parent HEAD, two GitHub sub-issues and child lifecycle IDs.

- [ ] **Step 1: Stage spec, plan, and Turing loop before execution prepare**

Run:

```bash
./bin/issueops artifact stage --id io-1a6a8e362e51 --name spec --file /tmp/issueops-parallel-dogfood.Xl2o2e/spec.md --json
./bin/issueops artifact stage --id io-1a6a8e362e51 --name plan --file /tmp/issueops-parallel-dogfood.Xl2o2e/plan.md --json
./bin/issueops artifact stage --id io-1a6a8e362e51 --name verified-execution-loop --file /tmp/issueops-parallel-dogfood.Xl2o2e/verified-execution-loop.md --json
```

Expected: 세 artifact가 SHA-256 manifest와 함께 성공한다.

- [ ] **Step 2: Preview와 동일 request로 parent direct execution을 confirm**

Run `issueops execution prepare`을 먼저 preview한 뒤 같은 actor/model/effort request에 `--confirm`만 추가한다.

Expected: branch `221-issueops-parallel-worktree-dogfood`, generation 1, sibling canonical worktree, direct holder가 기록된다.

- [ ] **Step 3: Canonical worktree에 설계·계획·초기 보고서를 작성하고 commit**

초기 보고서는 objective, criteria, baseline, event ledger, child matrix, verification, cleanup receipt 섹션을 포함한다.

Run:

```bash
git add -- docs/superpowers/specs/2026-08-02-issueops-parallel-worktree-dogfood-design.md docs/superpowers/plans/2026-08-02-issueops-parallel-worktree-dogfood.md .issueops/operations/2026-08-02-issueops-parallel-worktree-dogfood.md
git diff --cached --check
git commit
```

Expected: Conventional Commit subject와 Lore body를 가진 계획 commit 하나, clean worktree. 직후 standalone `git rev-parse HEAD`로 post-plan parent SHA를 읽고 운영 보고서와 두 child branch prepare에 동일 literal을 사용한다.

- [ ] **Step 4: Plan link, compatibility, Brooks verdict, implement phase를 기록**

Run exact actor flags from the active parent holder. Expected: parent readiness에서 design, plan, compatibility, devil's advocate 누락이 사라진다.

- [ ] **Step 4a: Provider side effect 전 actor validation 결함을 TDD로 교정**

첫 child confirm에서 #222가 실제 생성된 뒤 local child link가 active lease actor 부재로 실패했다. 아래 parent-only 파일만 수정한다.

- `cmd/issueops/issueopscli/remotecmd/remote.go`
- `cmd/issueops/issueopscli/remotecmd/remote_test.go`
- `internal/core/issueops/issueops_actor.go`
- `internal/core/issueops_facade.go`
- `internal/adapter/cli/issueops_catalog.go`
- `cmd/issueops/testdata/usage.golden.txt`

active holder confirm 성공과 wrong actor가 provider 호출 전에 실패하는 두 테스트를 먼저 RED로 확인한다. create-child에 holder/canonical cwd flags를 추가하고, provider 호출 전 actor validation과 provider 성공 후 actor-bound link를 적용한다. focused/package/full 검증 뒤 별도 atomic fix commit을 push한다. #222는 readback/reconcile하며 중복 생성하지 않고, fix commit의 exact HEAD를 두 child sealed base로 다시 고정한다.

실제 reconcile에서 `child start`와 `link-child`가 hook exact classifier에서 다시 막히면 아래 네 파일에 delegation command spec과 owner-mutation 분류를 TDD로 추가한다.

- `internal/core/commandparse/issueops.go`
- `internal/core/commandparse/issueops_test.go`
- `internal/core/lifecycle/lifecycle_execution_guard.go`
- `internal/core/lifecycle/lifecycle_owner_mutation_test.go`

`child` 두 단어 path, child start/status/list/accept/reject/drop, link-child, remote create-child의 exact flags를 고정하고, child mutation만 `--parent`를 lifecycle 식별자로 사용한다. status/list는 observation, status `--repair`는 current holder mutation으로 나누며 actor signature 또는 알려지지 않은 flag가 빠지면 계속 차단되어야 한다.

- [ ] **Step 5: 두 `[p]` GitHub sub-issue와 child lifecycle을 만든다**

Child A goal: cross-process concurrent child start parent-ref preservation.

Child B goal: cross-process concurrent child accept verdict/evidence preservation.

Expected: provider-native parent hierarchy, selected labels/assignee, Wave 1/선행 조건 없음, 서로 다른 child issue numbers와 lifecycle IDs. 두 child branch prepare는 `base_branch=221-issueops-parallel-worktree-dogfood`, 동일 post-plan parent SHA, canonical parent worktree를 기록한다.

### Task 2: Child A — cross-process concurrent start

**Files:**
- Create: `internal/core/issueops/issueops_delegation_start_process_test.go`
- Create: `.issueops/operations/2026-08-02-issueops-parallel-worktree-child-start.md`
- Do not modify: every other tracked path.

**Interfaces:**
- Consumes: `createDelegationReadyParentForTest`, `startIssueOpsChildForTest`, `ReadIssueOps`, `childRefByID`.
- Produces: `TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses`와 전용 helper entrypoint.

- [ ] **Step 1: Worker authority를 검증하고 child direct execution을 prepare**

각 명령을 별도로 실행해 `pwd`, git root, branch, HEAD, dirty paths를 확인한다. `issueops execution whoami` actor로 preview/confirm하고 이후 모든 mutation은 returned canonical worktree에서만 수행한다. child HEAD가 parent가 기록한 post-plan SHA와 다르면 edit 전에 중단한다.

- [ ] **Step 2: Characterization test를 작성**

핵심 형태:

```go
func TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses(t *testing.T) {
    stateRoot := t.TempDir()
    parent := createDelegationReadyParentForTest(t, stateRoot)
    // Spawn four copies of the current test binary with unique branches.
    // Each helper writes ready/worker-0 style markers, then polls one gate.
    // The parent waits for all ready markers before creating the gate once.
    // Preserve CombinedOutput for every nonzero exit.
    // Assert all four exact child refs persist once on the parent.
}
```

고장 mutation: `appendIssueOpsChildRef`의 parent-wide outer lock이 사라지면 일부 ref가 lost update로 사라져야 한다.

- [ ] **Step 3: Focused characterization PASS와 mutation RED를 관찰**

Run:

```bash
go test -v ./internal/core/issueops -run '^TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses$' -count=1
```

Expected: named `=== RUN`, PASS, 네 ready marker가 gate 이전에 존재, subprocess error 없음.

그 다음 격리 worktree에서 `appendIssueOpsChildRef`의 outer `withIssueOpsLock`만 direct closure invocation으로 임시 우회하고 동일 test를 `-count=20`으로 실행한다. Expected: nonzero exit와 ref 누락 assertion. 즉시 원래 lock call을 복원하고 아래 두 명령을 실행한다.

```bash
git diff -- internal/core/issueops/issueops_delegation.go
go test ./internal/core/issueops -run '^TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses$' -count=10
```

Expected: production diff 출력 없음, repeated PASS.

- [ ] **Step 4: Atomic commit과 child PR publication**

정확한 test와 child 증거 보고서만 stage하고 staged diff/check를 확인한다. 보고서는 barrier, focused PASS, mutation RED, restore diff 0, repeat PASS, actor/generation/worktree/base SHA를 포함한다. commit subject는 `test(issueops): cover cross-process child starts`이며 Lore에 focused/repeat 결과와 test-only risk를 기록한다. branch를 push하고 parent branch를 base로 하는 draft PR을 생성·readback한다. child execution complete는 committed child 보고서와 verified PR을 사용하되, merge와 cleanup은 parent가 review 뒤 수행한다.

### Task 3: Child B — cross-process concurrent accept

**Files:**
- Create: `internal/core/issueops/issueops_delegation_accept_process_test.go`
- Create: `.issueops/operations/2026-08-02-issueops-parallel-worktree-child-accept.md`
- Do not modify: every other tracked path.

**Interfaces:**
- Consumes: `createDelegationReadyParentForTest`, `startIssueOpsChildForTest`, `acceptIssueOpsChildForTest`, `writeIssueOpsRecordForDelegationTest`, `childRefByID`.
- Produces: `TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses`와 전용 helper entrypoint.

- [ ] **Step 1: Worker authority를 검증하고 child direct execution을 prepare**

Task 2와 같은 독립 startup evidence를 자기 cycle/branch/worktree에 대해 수행하고 HEAD가 동일 post-plan parent SHA인지 확인한다.

- [ ] **Step 2: Characterization test를 작성**

핵심 형태:

```go
func TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses(t *testing.T) {
    stateRoot := t.TempDir()
    parent := createDelegationReadyParentForTest(t, stateRoot)
    // Create four distinct children, persist each as IssueOpsPhaseDone.
    // Each helper writes ready/worker-0 style markers; the parent releases one gate
    // only after all four markers exist.
    // Each helper accepts one child with unique literal evidence.
    // Read the parent and assert every exact ref is accepted with its evidence.
}
```

고장 mutation: `recordIssueOpsChildVerdict`의 parent-wide outer lock이 사라지면 일부 verdict/evidence가 lost update로 사라져야 한다.

- [ ] **Step 3: Focused characterization PASS와 mutation RED를 관찰**

Run:

```bash
go test -v ./internal/core/issueops -run '^TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses$' -count=1
```

Expected: named `=== RUN`, PASS, 네 ready marker가 gate 이전에 존재, 네 accepted receipt 보존.

그 다음 `recordIssueOpsChildVerdict`의 outer `withIssueOpsLock`만 임시 우회하고 동일 test를 `-count=20`으로 실행한다. Expected: nonzero exit와 verdict/evidence 누락 assertion. 즉시 원래 lock call을 복원한다.

```bash
git diff -- internal/core/issueops/issueops_delegation.go
go test ./internal/core/issueops -run '^TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses$' -count=10
```

Expected: production diff 출력 없음, repeated PASS.

- [ ] **Step 4: Atomic commit과 child PR publication**

정확한 test와 child 증거 보고서만 stage한다. 보고서는 barrier, focused PASS, mutation RED, restore diff 0, repeat PASS, actor/generation/worktree/base SHA를 포함한다. commit subject는 `test(issueops): cover cross-process child accepts`이며 Lore에 focused/repeat 결과와 test-only risk를 기록한다. branch를 push하고 parent branch를 base로 하는 draft PR을 생성·readback한다. child execution complete는 committed child 보고서와 verified PR을 사용하되, merge와 cleanup은 parent가 review 뒤 수행한다.

### Task 4: Parent review, accept, integration, quality gate

**Files:**
- Modify: `.issueops/operations/2026-08-02-issueops-parallel-worktree-dogfood.md`
- Integrate: 두 child test files.

**Interfaces:**
- Consumes: 두 child commits, focused evidence, execution status, reviewer verdicts.
- Produces: accepted child receipts, integrated parent HEAD, final tracked Turing/operations report.

- [ ] **Step 1: 각 child diff와 evidence를 별도 reviewer에게 제출**

Expected: Critical/Important finding 0. 확인된 finding은 해당 child를 reject/recover한 뒤 같은 reviewer가 재검토한다.

- [ ] **Step 2: 두 child PR을 parent branch에 merge**

각 merge 전후 child PR base/head/OID와 `git status --short`, `git log -1`, `git diff --stat`을 확인한다. GitHub merge 후 parent worktree를 `--ff-only`로 갱신하며 두 test와 두 child 보고서가 모두 parent branch에 존재해야 한다.

- [ ] **Step 3: Child cleanup과 parent accept receipt를 기록**

각 child에 대해 merged PR/OID를 확인하고 remote issue completion reflection/close, cleanup preview/apply를 수행한다. 그 뒤 archived child record에서 commit SHA, reviewer verdict, readiness barrier, current-code PASS, unlocked-mutation RED, 복원 후 production diff 0, repeated PASS를 evidence로 parent accept에 전달한다.

- [ ] **Step 4: Ordered verification gate**

Run:

```bash
go test -v ./internal/core/issueops -run '^(TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses|TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses)$' -count=1
go test ./internal/core/issueops -run '^(TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses|TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses)$' -count=10
go test ./internal/core/issueops -count=1
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
```

한 단계라도 실패하거나 이후 edit가 생기면 첫 단계부터 다시 실행한다.

- [ ] **Step 5: Shannon/ai-slop/Turing report를 확정하고 commit**

Test-only code diff에는 SNR, entropy, redundancy, overhead를 동일 scope로 측정한다. generic prose, duplicate helper noise, stale claim을 제거한 뒤 report에 event timestamps, lifecycle/worktree/generation matrix, verification output, review verdict, cleanup pending boundary를 기록한다.

### Task 5: PR publication, merge, and cleanup

**Files:**
- Modify before PR only: `.issueops/operations/2026-08-02-issueops-parallel-worktree-dogfood.md`

**Interfaces:**
- Consumes: clean parent worktree, generation, final HEAD, committed report, selected labels/assignee.
- Produces: verified PR, merge commit/OID, closed issues, zero local/remote lifecycle residue.

- [ ] **Step 1: Draft PR 생성과 readback**

정확한 parent generation, head/base, Korean body file, labels, assignee, actor flags로 `issueops remote create-pr` preview/confirm 후 verify-artifact를 실행한다.

- [ ] **Step 2: Execution complete와 PR review**

committed Turing report와 모든 verification command를 `issueops execution complete`에 전달한다. GitHub checks와 review threads를 확인하고 unresolved Critical/Important 0을 증명한다.

- [ ] **Step 3: PR merge**

provider readback으로 mergeable/green을 확인한 뒤 merge한다. merge OID와 mergedAt을 별도 readback한다.

- [ ] **Step 4: 고정 순서 cleanup**

```text
cleanup status --merged
cleanup close-children --merged --confirm
remote reflect-completion --confirm
remote close-issue --confirm
cleanup finish --preview
cleanup finish --apply --confirm --fingerprint 를 실행하되 바로 앞 preview JSON의 fingerprint 문자열을 손대지 않고 그대로 전달
```

각 child lifecycle에도 provider merge/parent integration evidence에 맞는 terminal/cleanup 경로를 적용한다. 이 순차 구간이 계획의 가장 긴 경로임을 운영 보고서에 실제 elapsed time과 함께 기록한다.

- [ ] **Step 5: Residue audit**

`git worktree list`, local branch 목록, remote branch 목록, `issueops list`, GitHub child/parent issue state, PR merged state를 각각 읽는다. parent source checkout은 clean `main`이고 `origin/main`과 동일해야 한다.
