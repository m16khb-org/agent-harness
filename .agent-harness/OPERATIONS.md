---
name: OPERATIONS.md
description: Install, sync, runtime, and operational procedures.
---

# Operations Map

`agent-harness` gives Codex and Claude Code the same Go binary, MCP schema, command policy, state store, shared skills, lifecycle hooks, and project-doc workflow.

Use this file as the quick map. Read the focused operation file that matches the task:

| Task | Read |
|------|------|
| Install, bootstrap, and refresh | `.agent-harness/operations/install.md` |
| Release checklist and clean-machine install reproducibility | `.agent-harness/operations/release-reproducibility.md` |
| Release dogfood transcripts and observed release UX gaps | `.agent-harness/operations/release-dogfood-notes.md` |
| Codex/Claude native skills, MCP registration, lifecycle hooks | `.agent-harness/operations/hosts.md` |
| Direct CLI, daemon-backed MCP, command policy, guard, worker commands | `.agent-harness/operations/cli-and-mcp.md` |
| Web-fetch deterministic benchmark and opt-in live parity | `.agent-harness/operations/web-fetch-live-parity.md` |
| self-verify, self-augment, API documentation gates, smoke checks | `.agent-harness/operations/verification.md` |
| project bootstrap, project-doc routing, MCP document update rules | `.agent-harness/operations/project-docs.md` |

## Core Surfaces

1. Native skills: `atomic-commit-push`, `issueops`, `self-augment`, `project-bootstrap`, `self-verify`, `stability-audit`, plus the named specialist skills in `skills/`.
2. MCP stdio proxy: `agent-harness mcp` starts or connects to the shared user-level `agent-harness daemon`.
3. CLI: `agent-harness inspect/preflight/status/verify-work/doctor/docs/project/policy/guard/state/issueops/loop/contract/daemon/worker/self-verify/self-augment/api-doc/hook`.
4. Loop contracts: `agent-harness loop start/record-attempt/status/stop` records verify-until-done state and strict readiness gates without executing verification commands.

## Daily Commands

```bash
agent-harness bootstrap --dry-run --json
agent-harness bootstrap
agent-harness project bootstrap --repo /path/to/repo --dry-run --json
agent-harness project bootstrap --repo /path/to/repo --sync --json
agent-harness doctor --repo . --json
agent-harness status --json
agent-harness docs --json
```

## Operational Health and One-Time Reconciliation

`agent-harness doctor` is the sole public cross-system health gate for canonical Git state, all IssueOps records/bindings, optional Orca inventory, and unexpected user-state artifacts. Invocation-only preservation never writes state:

```bash
agent-harness doctor --repo . --preserve-cycle EXACT_CYCLE_ID --preserve-terminal EXACT_HANDLE --json
```

- An active IssueOps execution is live only with a complete generation, native process receipt, canonical worktree, and mode-specific resource identity. Process absence alone never authorizes interrupt, deletion, or lease replacement; use the previewed generation-CAS replacement sequence and prove quiescence.
- Preserve flags are repeatable exact values for one doctor invocation. They do not create persistent exceptions or cure incomplete/duplicate identity.
- Orca remains optional. Absence is healthy only when no durable cycle claims Orca resources; otherwise inventory is unknown and doctor fails closed.
- The stability audit builds the binary, then delegates operational judgement to `doctor`. `--preserve-terminal EXACT_HANDLE` is a singular explicit assertion and takes precedence over the inherited environment; only when it is absent does a non-empty `ORCA_TERMINAL_HANDLE` remain the fallback. Sealed reconciliation passes its resolved `manifest.current_terminal.handle` explicitly and does not overwrite the environment variable.

The approved one-time full reconciliation uses an external mode-`0700` bundle at `~/.local/state/agent-harness-backups/<repo-fingerprint>/<UTC-timestamp>/`, not a product cleanup command. Git and SQLite backups are restore-tested; Orca snapshots are archival evidence only because the installed CLI exposes global reset but no conditional reset/import/restore. Stop before reset if the final full digest drifts. After a reset or crash seam, resume the sealed append-only journal and complete idempotently forward; do not infer a partial rollback.

## Tool Contract Conformance

```bash
agent-harness contract conformance baseline --json
HARNESS_TOOL_CONFORMANCE_LIVE=1 agent-harness contract conformance live \
  --hosts codex,claude \
  --model codex=default \
  --model claude=default \
  --profile clean \
  --target-completed 1 \
  --max-attempts-per-case 3 \
  --evidence-dir .agent-harness/evidence/tool-conformance \
  --json
agent-harness contract conformance replay --fixture PATH --json
```

`baseline`과 `replay`는 deterministic local gates다. `live`는 외부 model 비용 경계이며 opt-in env가 없으면 host process를 시작하지 않는다. Codex는 ephemeral/read-only/ignore-user-config 실행, Claude는 strict temp MCP config와 settings-source isolation을 사용한다. 사용자 MCP 등록이나 credential DB는 수정하거나 복사하지 않는다.

Initial live report가 `defer_hardening`이면 현재 preregistered matrix에서 confirmed drift가 없다는 뜻이며 production contract를 변경하지 않는다. `needs_reproduction`이면 report가 지정한 한 host+fixture만 별도 10/20-completed batch로 재현한다. `authorize_hardening`은 같은 normalized signature가 두 번 이상 관측된 경우에만 가능하다. 상세 denominator와 fixture promotion 규칙은 `.agent-harness/TESTING.md`를 따른다.

## State Store Maintenance

The sqlite-backed state stores accumulate WAL frames and sidecar files that need periodic checkpointing. Two surfaces handle this:

**Automatic (session-start hook):** `MaybeMaintainStateStores` runs WAL truncate + permission repair at most once per 24h via a `.last-store-maintain` sentinel in the state root. No user action needed.

**Manual CLI:**
```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json

```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows.
IssueOps v1 lease recovery is not part of store maintenance and never happens
from a time threshold.

## Kubectl Live-Access Approval

With `--enforce-gitops-kubectl`, live access requires explicit confirmation. Claude uses its native `ask`. Codex cannot emit native PreToolUse `ask`, so the first eligible request blocks with a short instruction such as `승인 AH-XXXXXX`.

Codex can reuse approval only for exact-allowlisted read-only exec diagnostics that state both kube context and namespace. For example:

```bash
kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user
kubectl --context bc-stgdev -n stg exec -c linkerd-proxy deploy/rest-api-gateway -- curl -fsS http://localhost:4191/metrics
```

Enter the exact token in the same session. The approval must be activated by an allowlisted diagnostic within 10 minutes. The first allowed command and each later allowed command refresh a 30-minute idle TTL for the same session, canonical repo, context, and namespace; workload target and container may change. Changing context or namespace, allowing the TTL to expire, or losing state requires a new token. Runtime state uses mode `0600` and stores only request/scope fingerprints, never raw commands or cluster identifiers.

Codex `kubectl port-forward` remains exact-command one-shot: the next identical request consumes its 10-minute grant. Unsafe or unclassified Codex exec, including generic shells, interactive flags, arbitrary file/env reads, redirects, and non-allowlisted curl/dig options, blocks without an approval token. Do not remove `--enforce-gitops-kubectl` or use a generic shell as routine recovery. Direct mutating kubectl commands remain blocked and must go through GitOps.

## Release Smoke

```bash
scripts/release-repro-smoke.sh
```

Use `.agent-harness/operations/release-reproducibility.md` before deciding Homebrew, tarball, or other release packaging.

## Invariants

- Default install writes only user-level host configuration. Target repos get files only through explicit project bootstrap or project-local opt-in.
- Host adapters are thin wrappers around the same CLI/core behavior. They must not duplicate policy, schema, or state semantics.
- Hooks provide routing, lifecycle state, and bounded reminders only. They must not create issues/PRs, run tests, edit shared docs, or perform long network/file reads.
- IssueOps implementation must pass durable design, compatibility, devil's-advocate, and execution v1 gates. Hooks do not decide compatibility, side effects, sub-agent usage, or lease ownership. `issueops execution prepare/status/claim/release/replace/reconcile/complete` and MCP `issueops_execution` are the single execution contract.
- Native install/update paths are standalone. External tools are neither installed nor required by `agent-harness`; use their own setup paths when a separate workflow needs them.
- Worker functionality remains policy-gated and state-first until write/network/background execution has explicit audit, timeout, cancellation, and redaction coverage.

## Optional Orca execution v1

Orca is user-installed and optional. Preview
`agent-harness issueops execution prepare --id ID --mode auto ... --json`,
review the mode, branch, base SHA, canonical worktree, and owner model, then
repeat the identical request with `--confirm`. `auto` selects Orca only when
readiness succeeds before mutation; otherwise it selects direct. The only
first-party owner hosts are Codex and Claude.

엄브렐라 자식 cycle은 `issueops branch prepare`에 canonical
`--parent-worktree`를 기록하고 preview의 `workspace.parent_worktree`가 부모
통합 worktree를 가리키는지 확인한다. 내부 delegation이 없는 provider-native
자식도 같은 계약을 사용한다. confirm은 Orca
`worktree create --parent-worktree path:<부모>`를 사용하고 생성 lineage의
`explicit-cli-flag`/`explicit` 영수증이 없으면 fail-closed한다. 독립 cycle은
기존처럼 top-level worktree로 생성된다.

GitLab-linked cycle은 host가 관찰한 `issue_snapshot` 또는 provider adapter의 일반
`glab api` read로 issue 본문을 봉인한 뒤 Orca를 사용할 수 있다. 특정 MCP server
이름이나 개인 wrapper를 하네스에 고정하지 않는다. host agent는 semantic leaf
`glab_api`와 실제 schema로 compatible tool을 찾고, 개인 wrapper도 같은
capability를 노출하면 사용할 수 있다. MCP snapshot이 있으면 source는
`glab_mcp`, 없으면 generic CLI fallback 결과는 `glab_cli`로 응답에 남는다.
`/issues/:iid`와 `/work_items/:iid`는 exact authority(명시 port 포함), project
path, IID가 모두 같을 때만 equivalent identity다.

**GitHub + Orca 모드는 브랜치 생성 순서를 뒤집는다.** IssueOps 정식 순서는 linked branch를
먼저 만들지만, Orca `worktree create`는 언제나 새 브랜치를 만들므로 이름이 겹쳐
`orca_branch_name_taken`으로 막힌다(#149·#152·#154).

`createLinkedBranch`는 `oid`에서 새 브랜치를 만들기 때문에 **원격에** 같은 이름이 있으면
실패하지만 **로컬에만 있으면 성공한다**(#163 실측). Orca는 로컬 워크트리와 로컬 브랜치만 만들고
push하지 않으므로 이 순서가 성립한다. 실제 값은 앞 단계 출력에서 그대로 옮겨 적는다 — 아래
꺾쇠 자리는 셸 변수가 아니다.

```bash
agent-harness issueops branch prepare --id ID --base-sha <BASE_HEAD> ...    # 기록만, 브랜치 없음
agent-harness issueops execution prepare --id ID --mode orca ... --confirm  # Orca가 로컬 브랜치 생성
gh api repos/<OWNER>/<REPO>/issues/<NUMBER> --jq .node_id                   # 다음 단계의 issueId
gh api graphql -f 'query=mutation($issueId:ID!,$oid:GitObjectID!,$name:String!){createLinkedBranch(input:{issueId:$issueId,oid:$oid,name:$name}){linkedBranch{ref{name target{oid}}}}}' -F issueId=<NODE_ID> -F oid=<BASE_HEAD> -F name=<BRANCH>
agent-harness issueops branch prepare --id ID ... --link-verified           # 추적 확인 후 갱신
```

`branch prepare`는 브랜치 존재를 검증하지 않고 이름 규칙과 레코드 일치만 보므로 1단계가
성립한다. 이 순서로 **Orca 모드와 이슈-브랜치 추적을 둘 다 얻는다.**

**`gh issue develop`을 쓰지 않는 이유는 base가 갈리기 때문이다(#176).** 그 CLI의 `--base`는
브랜치 이름만 받고 GitHub이 **그 시점** 브랜치 HEAD를 `oid`로 쓴다. Orca는 봉인된 base SHA에서
로컬 브랜치를 만들므로, 그 사이 base 브랜치가 진행하면 push가 `non-fast-forward`로 거부되고
봉인 가드가 `merge`를, 안전 훅이 force push를, `sync-base`가 completion 이전 실행을 막아 발행
경로가 사라진다. `oid`는 `createLinkedBranch`의 필수 필드이므로 봉인 SHA를 직접 넘기면 그
갈림이 원리적으로 생기지 않는다.

두 가지 형태 제약이 있다:

- **GraphQL 변수는 단일 인용해야 한다.** `$issueId` 같은 토큰이 인용 밖에 있으면 lifecycle
  가드가 셸 파라미터 확장으로 판정해 명령을 거부한다. 같은 이유로 `oid`·`name`·`issueId` 값도
  셸 변수로 넘기지 않고 리터럴로 적는다.
- **node ID 조회는 별도 단계다.** 한 명령에 `$(...)`를 넣으면 가드가 정적으로 분류할 수 없다.
  출력값을 그대로 옮겨 적는다.

`branch prepare`가 이 두 명령을 실행 가능한 형태로 `Steps`에 렌더하므로 `--base-sha`를 준
사이클은 그 출력을 그대로 따르면 된다. 문서와 `Steps`가 어긋나면 `Steps`가 원본이다.

GitLab은 브랜치 이름 규칙이 연결 수단이라 GitHub linked-branch 순서 문제는 없다.
그래도 base 못박기는 같은 계약이다. GitLab의 `ref`는 commit SHA를 받으므로
`--base-sha`를 주면 `Steps`가 그 값을 `ref`로 안내한다(`#180`).

GitLab issue snapshot은 다음 순서로 준비한다.

1. canonical worktree의 선택 문서 `.agent-harness/VCS.md`를 읽는다.
2. 현재 host의 trusted tool 중 semantic leaf `glab_api`와 실제 input schema가
   맞는 후보를 찾는다. server namespace는 capability identity가 아니다.
3. `projects/<URL-escaped-project>/issues/<iid>`를 읽고, schema가 지원하면
   `flags.hostname`으로 target host를 명시한다. 응답의 `web_url`,
   `description`, `state`를 exact linked identity와 대조한다.
4. MCP 호출이면 `issueops_execution.issue_snapshot`에 다섯 필드
   (`provider=gitlab`, `source=glab_mcp`, `web_url`, `body`, `state`)만 넘긴다.
   IssueOps CLI 호출이면 같은 객체를 mode `0600` private file에 쓰고
   `--issue-snapshot-file`로 넘긴다.
5. 후보 부재나 auth/permission/transport/schema 호출 실패 뒤에도 successful
   exact-identity MCP evidence를 얻지 못했을 때만 snapshot을 생략한다. 이 경우
   provider adapter가 일반 `glab api`를 사용한다. 이미 공급한 invalid evidence는
   CLI fallback하지 않고 fail-closed한다.

성공한 portable recipe는 canonical worktree에서 `project_docs_read` 후
`project_docs_update` SHA-CAS로 `.agent-harness/VCS.md`에 기록한다. GitLab과
GitHub가 함께 들어갈 수 있는 provider-neutral 문서이며, GitHub CLI recipe는
검증된 `gh issue view <url> --json url,body,state`를 사용한다. 실제로 관찰하지
않은 MCP 이름, 개인 wrapper 경로, token/profile/server namespace는 기록하지
않고 OpenWiki 자동 update도 실행하지 않는다.

For Orca mode, follow `skills/issueops/references/execution.md`. Preparation
seals the remote issue body, context packet, fully rendered owner prompt, and
private claim-token file before launch. The durable Orca binding stores
artifact identity version 1 plus the issue-body, packet, and prompt SHA-256 values before the terminal preparation
intent is deleted. Resume verifies those stored values and never rerenders the
prompt with the currently installed template. The fresh owner runs the exact
`issueops execution claim` command. Only the active
generation holder implements, verifies, creates the draft PR/MR, and completes
from the canonical worktree. The source main worktree remains available for
unrelated cycles.

Every external mutation stores intent first. Timeout or ambiguous output is not
absence: inspect `issueops execution status` and use preview/confirm
`issueops execution reconcile`; never repeat create or switch to direct after a
possible Orca mutation. Failed-holder recovery uses the ordered generation-CAS
replacement sequence and proves the prior process/resource is quiescent before
creating a new claimable generation.

When a coordinator sees claimable Orca status return an exact generation-bound
`execution resume --confirm` next command, execute that command unchanged. A
dispatched owner is different: when its sealed packet injects a claim command,
that claim is its only next action and it must not recursively execute status'
recovery resume. Resume accepts either
the complete explicit `ACTOR_FLAGS` receipt or no actor flags; the actor-free
form observes the current Codex/Claude session, native host process ancestry,
and canonical process cwd. Supplying only part of `ACTOR_FLAGS` is rejected.
An older v1 Orca binding with no identity version marker and no digests remains
readable but is not resumable as-is. A versioned all-empty, unversioned-complete,
partial, or future-version identity is an invariant violation. Legacy status returns `execution replace
--preview`; execute each emitted command through generation-CAS `--reseed`,
then execute the emitted resume command. Do not trust the current worktree
files or add digest fields manually.

완료 영수증이 있는 `released` 또는 `claimable` execution을 `--reseed`하면 기존 영수증은
generation, reason, reopen time과 함께 append-only `execution.completion_history`로 이동하고
현재 `completion`은 비워진다. 같은 raw-CAS commit이 phase를 `implement`로 되돌리고 현재
HEAD에 묶인 AI-slop/implementation-review/remote-completion proof를 제거하며, `implement`
이후 ledger entry를 `stale: completed execution reseed (<old> -> <new>)`로 표시한다. Branch,
worktree, PR/MR artifact, feedback, decision, sync-base event와 이전 history는 보존된다. History가
없는 기존 schema v1 record도 계속 읽을 수 있다. 새 completion은 receipt에 lease generation을
직접 기록한다. Generation이 없거나 0인 current completion은 invalid v1 state이며 요청의
`--completion-generation` 값으로 보정하지 않는다. Preview와 reseed는 durable stamped generation만
권위로 사용하고, 누락하거나 충돌하면 artifact prepare와 record CAS 전에 fail-closed한다.
Preview의 `next_command`는 stamped generation을 그대로 보존한다. Status가 반환한 exact generation-bound
`resume` 또는 `claim`을 실행한 뒤 새 HEAD에서 구현 검증, AI-slop proof, implementation review,
정상 forward phase를 다시 획득하고 새 evidence로 `execution complete`한다. 이전 completion을
재시도하거나 state JSON을 직접 지우지 않는다.

`issueops remote create-pr`와 `issueops execution reconcile`의
`remote_pr_create` 경로는 같은 publication capability handler를 사용한다.
초기 생성은 CLI에만 있고, 복구는 CLI와 MCP `issueops_execution` 양쪽에서 같은
reconcile handler로 흐른다. Handler가 조립되지 않은 compatibility/test wrapper는
provider를 직접 resolve하거나 legacy create/reconcile로 우회하지 않고 기존
provider-unavailable 계약으로 fail closed한다.

`issueops execution complete` requires phase `pr`, the exact active generation,
final HEAD, committed Turing report, verification evidence, and the verified
durable remote artifact URL. It records `done` and releases the lease
atomically. It never merges or deletes a worktree, branch, terminal, or remote
resource. Operational release evidence includes fake-provider recovery
matrices, installed Codex/Claude hook smokes, and disposable live Orca
ready/absent scenarios. Native installation and `self-verify` do not require
Orca availability.

`issueops execution sync-base` is the post-completion conflict-resolution
surface (#114, #318). Completed replacement preview fetches and checks the
recorded parent first. If the parent is not an ancestor of the completed work
HEAD, it returns `post_completion_sync_base_required` and the exact
`--preview --completion-generation N` command instead of reseed. Run that
preview from the canonical worktree, then execute its generation- and
fingerprint-fenced `--apply --confirm --fingerprint` command. Conflicts stop as
a merge-in-progress; the emitted exact generation-bound `--finalize` commits
and pushes, while `abort_command` withdraws. Apply/finalize/abort accept the
existing active holder path or a released current completion with matching
stamped generation and live native actor. They append only `sync_base_events`;
current completion, history, and phase remain immutable. After successful sync,
run completed replacement preview, reseed/claim, verify, and re-complete.
Rebase and force-push remain rejected. `execution reconcile
--preview` output is a constant, not an inventory observation — do not cite it
as residue evidence. Confirm reports `external_state_inspected=true` only after
the Orca inventory transport was actually attempted; local marker/request
validation failures remain false.

The #303 provenance boundary covers the typed
`BaseSyncRequiredError.next_command` on both reachable CLI and MCP error paths.
On the CLI sync-base success path, finalize and abort commands from one response
share one canonical executable/hash/generation observation. If observation or
binding fails, the adapter returns the structured provenance error without an
unbound command fallback. Per #326, the public MCP action enum has no sync-base
success action/result: AC-06 host parity is the exact CLI command plus the
Codex/Claude hook classifier, while MCP binds reachable resume/replace
`BaseSyncRequiredError` output only.

`issueops execution switch-mode` changes a prepared cycle between `direct` and
`orca` (#167). `prepare` seals the mode at first run and afterwards **rejects a
different `--mode` instead of silently ignoring it**, so this is the only way to
move a cycle that was prepared in the wrong mode. Run `--preview` for the gate
report and fingerprint, then `--apply --confirm --fingerprint`. Gates require
the mode to actually change, no writer to hold the lease, no pending external
intent, a clean worktree with commits pushed, and — switching to `orca` — a free
branch name, because the switch deletes the local branch Orca must recreate.

Do not run `execution replace --revoke` against your own live lease. That leaves
the cycle `revoking` with a live holder and closes every exit; `release
--generation N` is the correct command and the guard now refuses the self-revoke
with that instruction (#170). Note the actor identity too: `--session-id` is the
host session identifier, not a value you compose — it survives a session restart
while the PID changes, so `holder_identity_mismatch` after a restart means the
wrong id was recorded, not that the holder died.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.

## Orca owner sequence

Use this order:

```text
provider branch + base SHA
-> execution prepare preview/confirm
-> sealed packet/prompt + native owner launch
-> execution claim with token file and both digests
-> staged plan link + phase=implement
-> TDD/verification in the canonical worktree
-> AI-slop evidence + phase=ai-slop-clean
-> implementation review + atomic commit/push + phase=pr
-> generation-fenced draft PR/MR + readback
-> execution complete
-> separate human merge and cleanup choice
```

Preparation seals the caller-selected owner host, model, and effort before
terminal creation. Orca must expose `terminal create --command`; a launch shape
that cannot carry the exact model contract is rejected. The write fence is the
exact lifecycle ID, generation, native process receipt, canonical worktree, and
persisted Orca identity. The source main worktree remains available for
unrelated work throughout the sequence. Plan 연결과 세 phase 전이는 sealed
owner packet의 exact command를 사용한다. Owner가 phase를 추론해 건너뛰거나
cleanup evidence 기록이 phase를 자동 전이한다고 가정하지 않는다.

## IssueOps 이원 구조 운영 (planner/implementer)

- 스폰 준비: `issueops artifact stage --id ID --name plan|spec|turing-loop --file PATH` (prepare 전에만; 잘못 올렸으면 `artifact unstage`) → `issueops execution prepare --id ID --mode auto ...` (`--owner-model` 생략 시 host implementer 기본값: codex `gpt-5.6-terra`/xhigh, claude `claude-sonnet-5`/high; Claude planner/reviewer 기본값은 `claude-opus-5`/high이며 Fable 5는 명시적 수동 지정만 허용).
- 다중 사이클 조망: `issueops list [--repo PATH] --json` — claimable/cleanup/unreflected 플래그와 scanned_records 비용을 함께 노출한다.
- 하위 세션 publication(orca): `issueops implementation-review record --id ID --verdict pass --finding ... --evidence ... --reviewer-model <planner급>` 기록 후에만 `remote create-pr`가 통과한다. diff가 바뀌면 stale로 다시 막힌다.
- 머지 후 정리(순서 고정): `cleanup status --merged` → `cleanup close-children --merged --confirm` → `remote reflect-completion --confirm` → `remote close-issue --confirm` → `cleanup finish --preview` → `cleanup finish --apply --confirm --fingerprint FP`. finish 재실행 전에는 preview로 fingerprint를 재발급한다.
- Turing 리포트는 **원격 아티팩트(PR/MR) 생성 이전에** 커밋한다. `execution complete`가 리포트를 요구하는 시점이 머지 이후면 그 커밋이 어느 아티팩트에도 실리지 못해 두 번째 PR/MR이 필요해진다(GitHub PR에서 실측, #153). 원인은 provider가 아니라 `execution complete`의 리포트 요구 시점이므로 GitLab MR에서도 같을 것으로 보이나 실환경에서 확인하지 않았다. #153이 그 상황을 회복 가능하게 고쳤지만(원격 tip이 base에 도달했으면 통과) 애초에 만들지 않는 편이 낫다.
- 하네스 코드를 고친 뒤에는 **설치본을 재빌드한다**: `go build -o bin/agent-harness ./cmd/harness`. `~/.local/bin/agent-harness`는 그 경로를 가리키는 심볼릭 링크이므로 머지만 하고 재빌드를 빠뜨리면 고친 동작이 나오지 않는다 — #153 cleanup에서 로컬 `main`이 두 머지 뒤처져 #154의 진단 필드가 출력되지 않았고 원인을 찾는 데 시간이 들었다.
- 생성된 IssueOps `next_command`의 absolute 첫 token과 끝의 `--generated-by-executable`, `--generated-by-sha256`, `--generated-for-generation`은 제거하거나 수동 보정하지 않는다. 첫 token이 생성 바이너리를 정확히 선택하므로 stale PATH에서도 동일 바이너리를 실행하며, 현재 바이너리는 subcommand 실행 전에 envelope를 canonical executable·SHA-256·durable generation과 비교한다. Hook은 durable worktree/source의 canonical binary 밖 absolute target, wrapper, substitution을 먼저 거부한다. 관측 실패나 mismatch가 나면 PATH를 바꾸거나 다른 바이너리로 우회하지 말고 structured error를 그대로 복구 근거로 사용한다. 사람이 직접 실행하는 일반 `agent-harness …` 명령은 계속 PATH를 사용한다. `execution switch-mode --apply` 성공은 execution authority를 제거하므로 실행 command 대신 `next_action` 안내를 반환하며, 새 prepare는 사용자가 일반 PATH 명령으로 시작한다.
