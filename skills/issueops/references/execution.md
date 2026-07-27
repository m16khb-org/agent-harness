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

`auto` resolves Orca only when its readiness probe succeeds before mutation.
An absent or unready Orca resolves to direct without creating Orca state. Once
an Orca mutation may have happened, ambiguity fails closed and must be
reconciled; it never falls back to direct.

## Prepare

Run the preview first, inspect the selected mode, branch, base SHA, worktree,
owner model, and next command, then repeat the same request with `--confirm`:

```bash
agent-harness issueops execution prepare \
  --id "$ISSUEOPS_ID" \
  --mode auto \
  --owner-host "$OWNER_HOST" \
  --owner-model "$OWNER_MODEL" \
  --owner-effort "$OWNER_EFFORT" \
  $ACTOR_FLAGS \
  --json

agent-harness issueops execution prepare \
  --id "$ISSUEOPS_ID" \
  --mode auto \
  --owner-host "$OWNER_HOST" \
  --owner-model "$OWNER_MODEL" \
  --owner-effort "$OWNER_EFFORT" \
  $ACTOR_FLAGS \
  --confirm \
  --json
```

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

## Status And Claim

Status is the read-only orientation command for either mode:

```bash
agent-harness issueops execution status --id "$ISSUEOPS_ID" --json
```

A direct holder does not claim again. An Orca owner reads the private rendered
prompt and context packet, verifies the issue and packet SHA-256 values, then
runs the exact claim command rendered by preparation/status:

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
   --quiescence-fingerprint HEX --confirm` creates the next claimable
   generation.

Every mutating step also requires `ACTOR_FLAGS`. `--reseed` is limited to the
documented holderless recovery case and still uses generation CAS and confirm.

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

메인(planner) 세션은 prepare **이전에** 계획/스펙/turing loop를 스테이징한다:

```bash
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file plan.md --json
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name spec --file spec.md --json
agent-harness issueops artifact stage --id "$ISSUEOPS_ID" --name turing-loop --file turing-loop.md --json
```

- 이름은 `plan|spec|turing-loop` 고정, 파일당 1MiB 상한, secret 패턴은 거부된다(스크럽 없음).
- prepare가 워크트리 생성 직후 `<worktree>/.agent-harness/artifact/<name>.md`(0600)로
  materialize하고, orca 모드는 `artifact_manifest`(sha256)를 sealed packet에 봉인한다.
  owner claim 시 manifest가 검증되며 불일치는 drift로 read-only 잔류한다.
- prepare 이후 stage/unstage는 명시적으로 거부된다(조용한 no-op 없음). 잘못
  스테이징했으면 prepare 전에 `issueops artifact unstage --id ID --name NAME`.
- `execution replace --reseed`는 artifact를 재-materialize하지 않는다(digest 재검증만).
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
