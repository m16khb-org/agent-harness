# IssueOps Execution v1

IssueOps v1 stores one `ExecutionV1` in each lifecycle record. That execution
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

## Parallel Independence

There is one active execution per record, not one global execution per source
repository. Fence only the selected exact lifecycle ID, generation, native
holder, and canonical worktree. Unrelated cycles remain independent. The
source main worktree remains available before, during, and after either mode
for unrelated work, but it must not mutate the selected execution or its
canonical worktree.
