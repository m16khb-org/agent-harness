# IssueOps Execution v1

IssueOps v1 stores one `Execution` in each lifecycle record. That execution
owns one canonical worktree and one generation-fenced write lease. At most one
native holder may mutate the record and worktree. The exact lifecycle ID,
generation, native process receipt, and canonical cwd are the authority; a
source checkout, branch name, or host session alone is not.

The two modes are `direct` and `orca`:

- `direct` provisions the sibling worktree and grants generation 1 to the
  calling Codex or Claude session.
- `orca` provisions the same canonical worktree, seals the remote issue and
  owner context packet, launches one native owner, and leaves the lease
  claimable until that owner proves the sealed digests and consumes the token
  file.

Preparation persists the issue-body, context-packet, and fully rendered owner
prompt SHA-256 values together on the generation-bound Orca binding before the
terminal intent is deleted. Resume compares artifact bytes to that durable
identity; it does not rerender the prompt with the currently installed
template.

`auto` resolves Orca only when its readiness probe succeeds before mutation.
An absent or unready Orca resolves to direct without creating Orca state. Once
an Orca mutation may have happened, ambiguity fails closed and must be
reconciled; it never falls back to direct.

Orca 1.4.162 이상에서는 모든 task mutation이 명시적 Run과 그 Run의 current
consumer인 coordinator terminal을 함께 요구한다. IssueOps는 이를 다음의 독립
intent 단계로 기록한다:

```text
worktree_create -> terminal_create -> run_create -> run_bind -> task_create -> dispatch
```

`run_create`는 lifecycle·generation·operation marker를 objective로 가진 새 Run을
만들고, `run_bind`는 현재 coordinator의 정확한 `ORCA_TERMINAL_HANDLE` 형식을
fail-closed gate로 검증한 뒤 `run-current`/`run-use --id <run>`를 현재 process에서
호출한다. 현재 terminal RPC에 `--from`을 붙이면 Orca가 이를 명시적 대리 호출로
해석해 process authority를 보존하지 못할 수 있으므로 생략한다. focus, cwd, 전역
current Run을 권한으로 추론하지 않는다. `run_bind`만 수렴 가능한 재바인딩이므로
불명확한 결과 뒤에도 동일 Run으로 유한 재시도할 수 있고, 자원을 만드는 다른
단계는 종전처럼 불명확한 mutation을 반복하지 않는다.

Run 도입 전 binding의 빈 `run_id`는 읽을 수 있다. 이 경우 task 소유권과 완료
처리는 모든 명시적 Run의 `task-list --run` 결과에서 정확히 하나가 일치할 때만
복구한다. `run_legacy_local`이나 여러 Run의 동명 task는 권한으로 채택하지 않는다.

## GitLab Issue Snapshot

GitLab-linked cycle도 Orca를 사용할 수 있다. agent-harness가 요구하는 것은 특정
MCP server나 wrapper가 아니라 linked issue와 identity가 같은 bounded snapshot이다.
host agent는 먼저 선택 문서 `.agent-harness/VCS.md`를 읽고, 현재 등록 도구 중 실제
schema가 호환되는 semantic leaf `glab_api`를 찾는다. server namespace와 개인
wrapper 이름은 capability identity가 아니며 packet이나 record에 저장하지 않는다.

`glab_api`로 `projects/<URL-escaped-project>/issues/<iid>`를 읽고, schema가
지원하면 `flags.hostname`으로 target host를 명시한다. 응답의 `web_url`,
`description`, `state`에서 다음 다섯 필드만 정규화한다:

```json
{
  "provider": "gitlab",
  "source": "glab_mcp",
  "web_url": "https://gitlab.example.com/group/project/-/issues/69",
  "body": "remote issue description",
  "state": "opened"
}
```

MCP `issueops_execution`을 호출하면 이 객체를 `issue_snapshot`에 넣는다. host가
GitLab MCP를 읽었지만 IssueOps는 CLI로 호출한다면 같은 JSON을 exact mode `0600`,
non-symlink regular file에 쓰고 `--issue-snapshot-file PATH`를 prepare, claim,
`replace --finalize|--reseed`, pending `worktree_create`의
`reconcile --confirm`에 전달한다. Reconcile preview와 다른 pending stage에는
snapshot을 전달하지 않는다. file은 1 MiB 이하이고 unknown field나 trailing
JSON이 없어야 한다. core가
authority(명시 port 포함), project path, IID, non-empty body(512 KiB 이하),
`opened|closed` state를 다시 검증한다.

후보 부재나 auth/permission/transport/schema 호출 실패 뒤에도 successful exact-identity MCP evidence를 얻지 못했을 때만 snapshot 인자를 생략한다.
provider adapter가 일반 `glab api` CLI로 같은 필드를 읽고 성공 결과의
`issue_snapshot_source=glab_cli`를 기록한다.
이미 공급한 invalid evidence는 CLI fallback하지 않고 fail-closed한다. MCP 성공은
`issue_snapshot_source=glab_mcp`로 확인한다.

성공한 provider read가 재사용 가능한 새 recipe라면 canonical worktree에서
`project_docs_read`로 `.agent-harness/VCS.md`의 최신 SHA/content를 읽고,
`project_docs_update` SHA-CAS로 tool leaf, 관찰한 schema, endpoint/필드, CLI
fallback만 기록한다. secret, token, 개인 경로, server namespace는 기록하지
않는다. 이 기록은 OpenWiki 자동 update를 실행하지 않는다.

## GitHub Orca Branch Ordering

GitHub Orca는 #176에서 검증된 순서를 고정한다: `branch prepare` (base SHA only) → `artifact stage --name plan` → `execution prepare --mode orca` → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified`.
첫 `branch prepare`는 issue identity와 exact base SHA만 기록하고 provider branch를
만들거나 `--link-verified`를 기록하지 않는다. plan을 먼저 stage한 뒤 Orca prepare가
같은 이름의 local-only branch/worktree를 만든다. 그 다음 `createLinkedBranch`에 봉인
SHA를 `oid`로 전달해 같은 이름의 remote linked branch를 만들고, exact reader로
연결을 확인한 뒤 `branch prepare --link-verified`를 기록한다. `gh issue develop`은
링크 시점 base branch HEAD를 사용할 수 있어 봉인 SHA와 갈라지므로 이 경로에서 금지한다.

## Prepare

Run the preview first, inspect the selected mode, branch, base SHA, worktree,
owner model, and `next_command`, then execute that exact returned command:

```bash
agent-harness issueops execution prepare \
  --id "$ISSUEOPS_ID" \
  --mode auto \
  --owner-host "$OWNER_HOST" \
  --owner-model "$OWNER_MODEL" \
  --owner-effort "$OWNER_EFFORT" \
  $ACTOR_FLAGS \
  --json
```

The preview response seals the selection evidence in `readiness_fingerprint` and renders the exact confirm as `next_command`. Confirm must supply `--expected-readiness-fingerprint`; any changed probe result, owner profile, provider/issue identity, or explicit-direct reason fails before mutation. Explicit `--mode direct` is reserved for a bounded operator-approved exception and requires `--direct-reason`; it performs no Orca probe. Durable status exposes the same fields under `execution.selection`.

`ACTOR_FLAGS` are the exact native process identity and cwd:

```text
--host codex|claude --session-id ID [--agent-id ID]
--session-pid PID --session-started-at RFC3339
--session-executable PATH --cwd PATH
```

The provisioned path is the fixed sibling
`${repo}.worktrees/<branch-with-slashes-replaced>`. Preparation creates or
reuses only that exact branch/worktree pair and records its base SHA. Do not
create or link another worktree manually.

엄브렐라 자식은 branch prepare에서
`--parent-worktree ${repo}.worktrees/<base-branch-with-slashes-replaced>`를
명시한다. IssueOps는 이를 `workspace.parent_worktree`에 함께 봉인하고 canonical
부모 경로와 다르면 실행 전에 거부한다. 기존 delegation cycle은 이 값이 없어도
같은 경로를 계산해 하위 호환된다. Orca mode는 봉인된 값을
`worktree create --parent-worktree path:<부모>`로 전달하고 lineage의
`capture.source=explicit-cli-flag`, `capture.confidence=explicit`를 검증한다.
따라서 provider-native 엄브렐라 자식도 Orca UI에서 부모 통합 worktree 아래에
표시된다. 독립 cycle은 `--parent-worktree`를 생략해 top-level을 유지한다.

## Status And Claim

Status is the read-only orientation command for either mode:

```bash
agent-harness issueops execution status --id "$ISSUEOPS_ID" --json
```

A direct holder does not claim again after normal preparation. Holderless
direct recovery is the exception: `replace --reseed|--finalize` returns an exact
`claim` command for the new claimable generation. An Orca owner reads the
private rendered prompt and context packet, verifies the issue and packet
SHA-256 values, then runs the exact claim command rendered by
preparation/status:

```bash
agent-harness issueops execution claim \
  --id "$ISSUEOPS_ID" \
  --generation "$GENERATION" \
  --claim-token-file "$CLAIM_TOKEN_FILE" \
  --issue-body-sha256 "$ISSUE_BODY_SHA256" \
  --context-packet-sha256 "$CONTEXT_PACKET_SHA256" \
  $ACTOR_FLAGS \
  --json
```

The token is read from its private file, is never printed or copied into a
prompt, and is consumed exactly once. A digest mismatch leaves the owner
read-only.

## Release, Replacement, And Reconciliation

The active holder may voluntarily release its exact generation:

```bash
agent-harness issueops execution release \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" $ACTOR_FLAGS --json
```

Replacement is a fail-closed sequence. There is no unsafe override:

1. `issueops execution replace --preview` returns the exact generation and
   inventory fingerprint.
2. `issueops execution replace --revoke --expected-generation N
   --inventory-fingerprint HEX --reason TEXT --confirm` revokes that generation.
3. `issueops execution replace --finalize-preview --expected-generation N`
   proves the old process and Orca resource are quiescent and returns a
   quiescence fingerprint.
4. `issueops execution replace --finalize --expected-generation N
   --quiescence-fingerprint HEX --confirm` reseals the generation-specific
   owner packet/prompt and only then makes that generation claimable.
5. `issueops execution resume --expected-generation N --confirm` creates a
   fresh Orca terminal/Run/task/dispatch in the existing canonical worktree and
   records `orca.lease_generation=N`.
6. The new owner claims with the resume result's token path plus issue/packet
   digests. A reseal
   failure preserves the previous durable lease and removes uncommitted
   generation token/packet/prompt files. A retry first recovers exact
   harness-owned residue for that still-uncommitted generation.

Every replacement step requires `ACTOR_FLAGS`. `--reseed` is limited to the
documented holderless recovery case and still uses generation CAS and confirm.
For Orca, `replace --finalize|--reseed` returns resume, not claim, as its next
command. Execute that exact status projection without adding flags:

```bash
agent-harness issueops execution resume \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" --confirm
```

Resume observes the current native Codex/Claude session, host process receipt,
and canonical cwd when actor flags are absent. A complete explicit
`ACTOR_FLAGS` receipt remains valid; a partial receipt is rejected.

A legacy v1 Orca binding with neither an artifact identity version marker nor
stored digests remains readable but cannot resume directly. New prepare and
reseed bindings carry identity version 1 plus all three digests; a versioned
all-empty, unversioned-complete, partial, or future-version binding is an
invariant violation, not a legacy recovery candidate. Legacy claimable status emits `execution replace
--preview`; follow the returned inventory-fenced `--reseed` command and then
the returned resume command. Partial identities are invalid, and neither the
prompt file nor a freshly computed digest is an accepted fallback trust root.

Resume never recreates or reparents the worktree. Its public response includes
`resume_disposition`: `existing_binding`, `reuse_terminal`, or
`create_terminal`. A same-generation live terminal/task pair is an idempotent
`existing_binding` success and allocates no new launch operation. The response
still returns the exact sealed claim command. A dispatched owner must execute
that injected claim command exactly once; status' `execution resume` is a
coordinator recovery projection and is not an owner action. 재실행이 필요하면 기존 terminal을
재사용할 수 있어도 새 generation-specific Run을 만들고 coordinator를 그 Run에
명시적으로 바인딩한 뒤 task/dispatch를 만든다. A live old-generation task or
terminal/task contradiction fails closed. Ambiguous terminal/task/dispatch
mutation stays pending and must be completed with `execution reconcile`; do not
repeat resume.

For direct mode, the same replacement result returns the exact token-backed
claim command instead:

```bash
agent-harness issueops execution claim \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --claim-token-file "$CLAIM_TOKEN_FILE" \
  $ACTOR_FLAGS --json
```

A released direct cycle is recovered through the finite `next_command` chain
returned by each read/mutation result:

```text
execution status
  -> execution replace --preview
  -> execution replace --reseed --inventory-fingerprint <preview fingerprint> --confirm
  -> execution claim --claim-token-file <current generation token>
```

Do not skip the preview or invent the fingerprint. A completed cycle does not
render this recovery chain.

A claimable legacy Orca cycle uses the analogous explicit chain:

```text
execution status
  -> execution replace --preview
  -> execution replace --reseed --inventory-fingerprint <preview fingerprint> --confirm
  -> execution resume --expected-generation <replacement generation> --confirm
```


When workspace provisioning or remote publication may have mutated external
state but the result is ambiguous, inspect and then confirm the exact
reconciliation:

```bash
agent-harness issueops execution reconcile --id "$ISSUEOPS_ID" --preview $ACTOR_FLAGS --json
agent-harness issueops execution reconcile --id "$ISSUEOPS_ID" --confirm $ACTOR_FLAGS --json
```

Do not retry the external create operation before reconciliation reports one
unambiguous result.

## Draft PR/MR And Completion

Only the active generation may create the draft PR/MR. The request carries the
expected generation, exact head/base branches, native actor, canonical cwd,
labels, and assignee, and uses preview then `--confirm`:

```bash
agent-harness issueops remote create-pr \
  --id "$ISSUEOPS_ID" --expected-generation "$GENERATION" \
  --title "$TITLE" --head "$BRANCH" --base "$BASE_BRANCH" \
  --body "$BODY" --label "$LABEL" --assignee "$ASSIGNEE" \
  $ACTOR_FLAGS --confirm --json
```

Completion is allowed only from phase `pr`, with the verified durable remote
artifact at the exact URL, a full final Git SHA, a Turing report, and repeatable
verification evidence:

```bash
agent-harness issueops execution complete \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --final-head "$FINAL_HEAD" \
  --turing-report "$TURING_REPORT" \
  --remote-artifact-url "$PR_URL" \
  --verification "$VERIFICATION" \
  $ACTOR_FLAGS --confirm --json
```

Successful completion records the receipt, moves the lifecycle to `done`, and
releases the lease atomically. It does not merge the PR/MR or delete the
worktree or branch. The owner returns the exact 14-field report defined in
`.agent-harness/karpathy/prompts/issueops-v1-owner-execution-v1.md`.

GitHub의 `branch_prepare.link_verified`가 false이면 sealed owner packet이
`gh issue develop --list <issue> --repo <owner/repo>` exact reader와 branch
prepare recorder를 함께 제공한다. owner는 이 reader를 한 번 실행해 exact branch
연결을 확인한 뒤에만 recorder를 실행하며, 대체 GraphQL이나 다른 provider reader를
추론하지 않는다.

Orca가 worker prompt에 주입하는 현재 제어 명령은 legacy `--to` 대신
`--dispatch-capability`를 사용할 수 있다. capability 경로는 exact `--from`을 함께
전달하고, `worker_done`은 `--outcome succeeded|failed`를 반드시 포함한다. hook은
legacy recipient와 capability recipient 중 정확히 하나만 admit하며 둘을 섞거나
알 수 없는 flag·outcome을 붙인 명령은 fail-closed한다.

## Host-Aware Owner Model Defaults

`--owner-model`/`--owner-effort`를 생략하면 prepare가 host별 implementer
기본값을 적용해 packet과 OrcaBinding에 기록한다. 명시 플래그가 항상 우선한다.

| host | implementer(하위 세션) | planner(리뷰 서브에이전트) |
|---|---|---|
| codex | `gpt-5.6-terra` / `xhigh` | `gpt-5.6-sol` / `xhigh` |
| claude | `claude-sonnet-5` / `high` | `claude-opus-5` / `high` |

planner 값은 owner 프롬프트의 `{REVIEWER_MODEL}`/`{REVIEWER_EFFORT}`로
렌더되어, 하위 세션이 구현 diff의 brooks 적대 리뷰 서브에이전트를 planner급
모델로 띄우는 실행 계약이 된다.

Claude Code의 자동 실행 경로는 `Opus 5 → Sonnet 5`다. Fable 5는 자동
기본값이나 폴백으로 사용하지 않으며, 필요한 경우에만
`--owner-model claude-fable-5`로 명시해 수동 실행한다.

## Artifact Staging And Sealing

메인(planner) 세션은 prepare **이전에** 승인된 child plan을 source checkout 밖의
coordinator 임시 파일에 쓰고 계획/스펙/turing loop를 스테이징한다. Delegation의
`parent_plan_path`는 child plan readiness 입력이 아니다:

```bash
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file plan.md --json
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name spec --file spec.md --json
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name turing-loop --file turing-loop.md --json
```

- 이름은 `plan|spec|turing-loop` 고정, 파일당 1MiB 상한, secret 패턴은 거부된다(스크럽 없음).
- Orca prepare preflight는 non-empty staged `plan`을 remote issue read와 모든 외부
  mutation 전에 요구한다. 이미 `plan_path`가 있으면 canonical child worktree 안의
  regular file이어야 하고 staged bytes와 SHA-256이 정확히 같아야 한다.
- worktree receipt 직후 prepare가 `<worktree>/.agent-harness/artifact/<name>.md`(0600)로
  materialize한다. Fresh Orca plan은 그 deterministic path를 durable `plan_path`로
  같은 CAS에 기록하고, `artifact_manifest.plan`에 같은 SHA-256을 봉인한 뒤에만
  terminal/Run/task/dispatch로 진행한다. 성공 readback 뒤 임시 source를 지워도 된다.
- prepare 전 잘못 스테이징했으면 `issueops artifact unstage --id ID --name NAME`.
  Prepare 후 recovery staging은 **Orca + released + holder/pending/completion 없음**의
  clean generation에서 `--name plan`만 허용한다. 현재 sealed packet은 바뀌지 않으며
  새 입력은 반드시 `execution replace --reseed`로 다음 generation에 재봉인한 뒤
  `execution resume`해야 한다.
- 기존 released cycle에 `plan_path`가 비어 있으면 canonical child worktree에 plan을
  쓰고 exact `link-plan`을 먼저 실행한 다음 그 동일 파일을 stage한다. Active,
  claimable, revoking, completed, direct execution과 path identity 교체는 fail-closed다.
- `execution replace --finalize|--reseed` 재봉인은 staged 원본과 기존 durable
  `plan_path`를 다시 읽어 exact digest identity를 요구한다. Replacement는
  `plan_path`를 새로 만들거나 바꾸지 않으며, immutable artifact writer는 동일
  바이트만 허용한다.
- Resume은 manifest plan digest, sealed plan file, durable in-worktree `plan_path`를
  모두 비교한다. 누락/drift는 operation ID, intent, terminal/Run/task/dispatch,
  lease mutation 전에 `orca_plan_artifact_required`와 replacement-preview action으로
  끝난다.
- 이 디렉토리는 gitignore 대상이다 — 보존은 completion 섹션이 담당한다.

## Implementation Review Gate (orca mode)

orca 모드 사이클의 owner는 publication 전에 planner급 모델 fresh 서브에이전트로
구현 diff의 brooks 적대 리뷰를 수행하고 결과를 기록해야 한다:

```bash
agent-harness issueops implementation-review record --id "$ISSUEOPS_ID"   --verdict pass --finding "..." --evidence "..."   --reviewer-host codex --reviewer-model gpt-5.6-sol   --host codex --session-id "$SESSION" --cwd "$WORKTREE" --json
```

- 게이트: `verdict==pass` + findings/evidence 실질 내용. 기록은 implement phase
  이후에만 가능하며, 리뷰가 검토한 변경 집합 fingerprint가 봉인되어 이후 diff가
  바뀌면 `implementation_review_stale`로 create-pr·strict readiness가 거부한다.
- `reviewer_*`는 감사 기록이다 — 하네스는 모델 자기신고를 검증하지 않는다.
- direct 모드는 이 게이트의 대상이 아니다. direct/무-execution 사이클의 brooks
  리뷰는 devils-advocate ledger 기록으로 남긴다.

## Post-Merge Cleanup Order

휴먼이 PR/MR을 머지한 뒤, 순서가 계약이다(모두 원격 readback 기반 fail-closed):

```bash
agent-harness issueops cleanup status --id "$ISSUEOPS_ID" --merged --json
agent-harness issueops cleanup close-children --id "$ISSUEOPS_ID" --merged --confirm --json
agent-harness issueops remote reflect-completion --id "$ISSUEOPS_ID" --confirm --json   # 보존 먼저
agent-harness issueops remote close-issue --id "$ISSUEOPS_ID" --confirm --json
agent-harness issueops cleanup finish --id "$ISSUEOPS_ID" --preview --json
agent-harness issueops cleanup finish --id "$ISSUEOPS_ID" --apply --confirm --fingerprint "$FP" --json
```

- `reflect-completion`이 최종 head·PR URL·검증 요약·artifact 본문(plan/spec 접힌
  전문)을 이슈 본문의 completion 섹션에 보존한 뒤에만 finish가 통과한다.
- finish apply는 orca 워크스페이스 회수(force=false) → git worktree 제거 → 로컬
  브랜치 CAS 삭제 → 감사 라인 멱등 반영 → **레코드 삭제** 순서로 진행하며, 각
  단계는 멱등이고 실패 시 레코드를 보존한 채 실패 지점을 기록한다. 재실행 전에는
  `--preview`로 새 fingerprint를 발급받아야 한다(이전 값 무효).
- 원격 브랜치는 건드리지 않는다. 다중 사이클 조망과 정리 후보 발견은
  `agent-harness issueops list --repo "$PWD" --json`을 사용한다.

## Parallel Independence

There is one active execution per record, not one global execution per source
repository. Fence only the selected exact lifecycle ID, generation, native
holder, and canonical worktree. Unrelated cycles remain independent. The
source main worktree remains available before, during, and after either mode
for unrelated work, but it must not mutate the selected execution or its
canonical worktree.
