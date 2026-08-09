# Issue 439 Orca parallel handoff Turing report

## Verdict

The clean-state parallel exercise found a real handoff reliability defect. Two of three independent Orca IssueOps owners completed their bounded work, but the third owner failed the sealed claim boundary four times. Therefore the result is **partial failure**, not an all-green handoff claim. The accepted child changes and the bounded third task were integrated and verified on the parent branch; the parent PR is intentionally not merged to `main` without separate user approval.

## Initial-state reset

- Dirty pre-existing Issue 439 work was archived at `/tmp/agent-harness-issue439-backup.LvwDxs/issue439-wip.tar.gz` with SHA-256 `b13a794677297b9233946467e4d1bb0173e47bf13fe9c025382f8c9b0984a49a` before removal.
- Pre-existing user state was moved to `/tmp/agent-harness-state-backup.U74LIx/agent-harness`.
- `orca orchestration reset --all --json` succeeded before the new exercise.
- The initial operational doctor completed in 5.96 seconds. Its sole inventory problem was `operational_inventory_unknown: orca_gates_failed`; there were no old IssueOps cycles or child worktrees left in the exercised scope.

## Sealed planning artifacts

| Artifact | SHA-256 |
| --- | --- |
| `.agent-harness/artifact/plan.md` | `c4d3bf31bfd11e7d2f54aa08a78f44b41e1f939e82876c769ddd85edc7c18769` |
| `.agent-harness/artifact/spec.md` | `483636efe0c5a6927afd26b54b7f17e83c7b6707c69d3acc81e952dd1bb66285` |
| `.agent-harness/artifact/turing-loop.md` | `f204f42535da9b348d0670a574c76c97c7c4cfde3d63032d1e687a66c3b1db96` |

The parent froze the Run-inventory port at commit `d2a4b2349370929429cc95ecc34e700141d20513` before dispatching the independent children.

## Parallel execution evidence

| Child | Lifecycle / Run / Task / Dispatch | Result | Artifact |
| --- | --- | --- | --- |
| #440 legacy UTC timestamp | `io-70ae2229985c` / `run_11a7508c7007` / `task_8381a523436d` / `ctx_32facc69c829` | completed, lease released, parent accepted | commit `73376e70c2a73905fe1ae97fa9169b34da0dee6a`, draft PR #443 |
| #441 Run-scoped inventory | `io-09f44130b26a` / `run_5038df14fcc9` / `task_78ca3cf82be1` / `ctx_2df8d094d046` | completed, lease released, parent accepted | commit `92e8eb9342329d62293eb3774dcf05bf9d16b8b8`, draft PR #444 |
| #442 full self-verify admission | `io-5eaff2697547` / `run_b84ae567eab0` / `task_a61438c93abe` / `ctx_35b21fbc0e0b` | failed at sealed claim after four fresh owner attempts; parent dropped child | no production commit or PR |

The two successful workers used distinct lifecycles, Runs, tasks, dispatches, terminals, worktrees, and branches. Their terminal sessions overlapped in wall-clock time, and both emitted `worker_done`, completed their IssueOps execution generations, pushed their heads, and produced passing GitHub checks before parent acceptance.

## Failed handoff diagnosis

The #442 generated artifacts were correct:

- the deterministic claim-token file was `.agent-harness/state/issueops-v1/03756896d76c921d/lease-1.token`, mode `0600`;
- sealed context and owner prompt: the same root token path;
- required behavior: copy the exact claim command without editing its arguments.

The native Codex owner transcript at `/Users/m16khb/.codex/sessions/2026/08/09/rollout-2026-08-09T21-38-02-019fe687-bbb3-77d3-981f-4fcc7ab63e18.jsonl` instead inserted `generation-1/` and invoked `.agent-harness/state/issueops-v1/03756896d76c921d/generation-1/lease-1.token`. The CLI correctly failed closed with `claim_token_file must be the deterministic current-generation path`. A successful #441 transcript used the exact root path. Repeated retries reached the same claim boundary, so the parent recorded the child as dropped rather than hiding or relabeling the failure.

`orca orchestration check --json` later replayed the #442 `worker_done` failure message with delivery `delivery_efde155e7998`, confirming that coordinator messaging worked and the defect occurred before lease ownership.

## Integrated implementation

- Legacy UTC completion timestamps are parsed without weakening RFC3339Nano handling.
- One typed Run inventory is shared request-locally, validated against Status even for zero Runs, cloned at each concurrent reader boundary, and consumed by three independent bounded readers.
- Server-filtered dispatched queries and their independent cross-check remain intact.
- Git and IssueOps/Orca inventories overlap; the independent Orca reads reach the planned concurrency without changing timeouts or adding global caching.
- Exact full self-verification reads accept `--full --iterations=10 --progress=jsonl`; duplicates, invalid iterations, write flags, redirection, and control operators remain rejected.
- Owner inspection treats a missing bound Run as authoritative resource absence only after a complete terminal inventory; transport failures and incomplete terminal inventory remain fail-closed. This unblocked typed cleanup after an explicit orchestration reset.

## TDD and review evidence

- Timestamp RED/GREEN tests covered RFC3339Nano, exact legacy UTC, and malformed timestamps.
- Run reader tests cover one `run-list`, exact per-Run task/dispatched/gate calls, cap 8, stable Run order, runtime/count/identity/duplicate fail-closed behavior, and lowest Run-index error selection.
- Collector tests cover shared inventory, zero-Run runtime fencing, server-filtered mismatch detection, cloned reader inputs, seven-way independent Orca overlap, two-way Git/IssueOps-Orca overlap, and ordered dispatch projection.
- Full self-verify admission tests first failed on the new exact form, then passed after the bounded parser change.
- Cleanup reproduction first returned `run_not_found` as ambiguous after `orca orchestration reset --all`; the new owner-inventory test failed before the fix and passes afterward, including negative transport and incomplete-inventory cases.
- Independent code review reported no critical findings. Its mutable-slice alias finding and explicit lowest-index-error coverage request were fixed and rechecked with focused race tests.

## Verification receipts

- `go test ./... -count=1`: pass.
- `go test -race ./... -count=1`: pass before the final review patch; affected packages passed race again after that patch.
- `go vet ./...`: pass.
- `go test ./cmd/harness/contractgolden -run Golden -count=1`: pass.
- `go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1`: pass.
- `go build -o /tmp/agent-harness-parent-439 ./cmd/harness`: pass.
- Focused post-review tests and race checks for `operationalhealth`, `orca`, `commandparse`, and `lifecycle`: pass.
- `git diff --check`: pass.
- The first two exact full self-verification attempts failed closed at redaction audit because this report used a claim-token label followed by a colon before a non-secret path, then described that literal label in the first retry note. Both secret-assignment-shaped phrases were removed; no secret value had been recorded.

## Cleanup receipts

- Parent draft PR: https://github.com/m16khb/agent-harness/pull/445, targeting `main`; it remains unmerged pending separate user approval.
- Orca stopped six child terminals, removed the three child worktrees, reset orchestration state, and read back only the baseline `run_legacy_local`, zero workers, and the current coordinator terminal.
- Typed `cleanup abandon` deleted child lifecycle records `io-70ae2229985c`, `io-09f44130b26a`, and `io-5eaff2697547` after fingerprinted previews. The remaining IssueOps inventory contains only the active parent lifecycle.
- Child PRs #443 and #444 are closed as superseded by #445. Child issues #440 and #441 are closed completed; #442 is closed not planned with the sealed-claim failure recorded.
- Local and remote child branches `440-orca-legacy-utc-timestamp`, `441-orca-run-inventory`, and `442-full-self-verify-admission` are absent. Git and Orca inventories contain only `main` and the preserved parent worktree.

Final full self-verification and the post-cleanup stability receipts are emitted from this committed, unchanged report tree and recorded in the parent handoff response.
