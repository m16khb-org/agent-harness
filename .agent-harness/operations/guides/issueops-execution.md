---
name: issueops-execution
description: IssueOps Orca execution lifecycle, recovery, sync-base, owner sequence, and planner/implementer operations.
---

# IssueOps Execution Lifecycle and Owner Workflow

This guide owns the Orca execution state machine, recovery, completion,
sync-base, mode-switch, the owner sequence, and the planner/implementer
operations. Provider publication and branch linkage contracts live in
[issueops-providers.md](issueops-providers.md). Canonical index:
[../../OPERATIONS.md](../../OPERATIONS.md).

## Execution Lifecycle, Recovery, and Completion

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
form observes the current Codex/Claude/Omo session, native host process ancestry,
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

Resume은 `artifact_manifest.plan`, private sealed plan file, canonical child
worktree 안의 durable `plan_path`가 같은 SHA-256인지 외부 mutation 전에 검증한다.
누락 또는 drift는 `orca_plan_artifact_required`와 replacement-preview action으로
끝나며 operation ID, intent, terminal/Run/task/dispatch, lease를 만들지 않는다.
기존 released Orca cycle에 plan이 없으면 coordinator가 canonical worktree에 child
plan을 작성하고 exact `link-plan`으로 경로를 고정한 뒤 동일 파일을 `artifact stage`
한다. 이 예외는 Orca + released + holder/pending/completion 없음에서 plan에만
허용된다. Stage는 현재 packet을 바꾸지 않으므로 반드시 generation-CAS
`execution replace --reseed` 후 emitted `execution resume`을 실행한다. Active,
claimable, revoking, completed, direct cycle에서는 stage/link가 계속 차단된다.

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

## Orca owner sequence

Use this order:

```text
provider branch + base SHA
-> approved child plan in coordinator temp file + artifact stage
-> execution prepare preview/confirm
-> worktree receipt + durable plan_path + sealed plan/packet/prompt
-> native owner launch
-> execution claim with token file and both digests
-> compatibility/devil's-advocate gates + phase=implement
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
Omo의 non-inject fallback은 official preamble 전체를 bracketed paste 한 번과
Enter 한 번으로 전달한다. Raw multiline send는 줄마다 별도 turn으로 실행될 수
있으므로 사용하지 않는다.

## IssueOps 10단계 운영

단계 판별은 `agent-harness issueops next --json`이 소유한다. 읽기 전용이며 현재 단계,
미충족 게이트, 다음 명령, 탈출 경로를 돌려준다. 세션 경계는 하나뿐이다: 1~3단계는
source checkout의 준비 세션이, 4단계 이후는 canonical worktree의 구현 세션이 수행하며,
그 경계를 만드는 것이 3단계 끝의 `execution prepare --mode auto`다. 어느 단계에서든
빠져나오는 길은 `issueops-abandon`이고, 미머지 사이클의 원격 정리는 `cleanup abandon`의
`--close-pr`·`--close-issue`·`--delete-remote-branch`가 소유한다.

- 스폰 준비: 승인된 child plan을 source checkout 밖의 임시 파일에 작성 → `issueops artifact stage --id ID --name plan --file PATH --json` → `issueops execution prepare --id ID --mode auto ...`. `spec|turing-loop`도 prepare 전에 stage할 수 있고 잘못 올렸으면 `artifact unstage`한다. Clean released Orca에서는 next-generation recovery용 plan stage만 허용되며 반드시 `execution replace --reseed` 후 resume한다. (`--owner-model` 생략 시 host implementer 기본값: codex `gpt-5.6-terra`/xhigh, claude `claude-sonnet-5`/high, Omo `openai-codex/gpt-5.6-sol`/max; Claude planner/reviewer 기본값은 `claude-opus-5`/high이며 Fable 5는 명시적 수동 지정만 허용).
- Orca가 Omo TUI를 native `--inject` 대상으로 인식하지 않는 runtime에서는 harness가 non-inject dispatch의 official preamble을 검증한 뒤 sealed terminal handle에 `terminal send --enter`로 전달한다. 이미 dispatch가 보이지만 prompt delivery receipt가 없는 recovery는 성공으로 간주하지 않고 fence한다.
- 다중 사이클 조망: `issueops list [--repo PATH] --json` — one-query SQLite snapshot에서 물리 `scanned_records`, `read_errors`/`unreadable_ids`/bounded `diagnostics`와 claimable/cleanup/unreflected/pending/failure/cleanup-failure 상태를 함께 노출한다. invalid row의 raw payload나 free-form 오류는 출력하지 않는다.
- 하위 세션 publication(orca): `issueops implementation-review record --id ID --verdict pass --finding ... --evidence ... --reviewer-model <planner급>` 기록 후에만 `remote create-pr`가 통과한다. diff가 바뀌면 stale로 다시 막힌다.
- 머지 후 정리(순서 고정): `cleanup status --merged` → `cleanup close-children --merged --confirm` → `remote reflect-completion --confirm` → `remote close-issue --confirm` → `cleanup finish --preview` → `cleanup finish --apply --confirm --fingerprint FP`. finish 재실행 전에는 preview로 fingerprint를 재발급한다.
- Turing 리포트는 **원격 아티팩트(PR/MR) 생성 이전에** 커밋한다. `execution complete`가 리포트를 요구하는 시점이 머지 이후면 그 커밋이 어느 아티팩트에도 실리지 못해 두 번째 PR/MR이 필요해진다(GitHub PR에서 실측, #153). 원인은 provider가 아니라 `execution complete`의 리포트 요구 시점이므로 GitLab MR에서도 같을 것으로 보이나 실환경에서 확인하지 않았다. #153이 그 상황을 회복 가능하게 고쳤지만(원격 tip이 base에 도달했으면 통과) 애초에 만들지 않는 편이 낫다.
- 하네스 코드를 고친 뒤에는 **설치본을 재빌드한다**: `go build -o bin/agent-harness ./cmd/harness`. `~/.local/bin/agent-harness`는 그 경로를 가리키는 심볼릭 링크이므로 머지만 하고 재빌드를 빠뜨리면 고친 동작이 나오지 않는다 — #153 cleanup에서 로컬 `main`이 두 머지 뒤처져 #154의 진단 필드가 출력되지 않았고 원인을 찾는 데 시간이 들었다.
- 생성된 IssueOps `next_command`의 absolute 첫 token과 끝의 `--generated-by-executable`, `--generated-by-sha256`, `--generated-for-generation`은 제거하거나 수동 보정하지 않는다. 첫 token이 생성 바이너리를 정확히 선택하므로 stale PATH에서도 동일 바이너리를 실행하며, 현재 바이너리는 subcommand 실행 전에 envelope를 canonical executable·SHA-256·durable generation과 비교한다. Hook은 durable worktree/source의 canonical binary 밖 absolute target, wrapper, substitution을 먼저 거부한다. 관측 실패나 mismatch가 나면 PATH를 바꾸거나 다른 바이너리로 우회하지 말고 structured error를 그대로 복구 근거로 사용한다. 사람이 직접 실행하는 일반 `agent-harness …` 명령은 계속 PATH를 사용한다. `execution switch-mode --apply` 성공은 execution authority를 제거하므로 실행 command 대신 `next_action` 안내를 반환하며, 새 prepare는 사용자가 일반 PATH 명령으로 시작한다.
