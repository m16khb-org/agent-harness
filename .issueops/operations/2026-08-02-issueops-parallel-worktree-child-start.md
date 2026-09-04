# IssueOps child start cross-process regression

## Scope

- Lifecycle: `io-65956e6938ea`, generation `2`
- Issue: https://github.com/m16khb-org/issueops/issues/222
- Branch: `222-cross-process-child-start-rerun`
- Base: `b4ee1dd881abaa50335af0358e71117961ff7513`
- Canonical worktree: `/Users/m16khb/Workspace/issueops.worktrees/222-cross-process-child-start-rerun`
- Owned tracked files:
  - `internal/core/issueops/issueops_delegation_start_process_test.go`
  - `.issueops/operations/2026-08-02-issueops-parallel-worktree-child-start.md`

## Actor

- Host: `codex`
- Session: `019fc065-25e9-7613-9de3-86c8b61b502c`
- Agent: `019fc0b2-4cc6-7b02-9b6e-83981cbbe437`
- Process: pid `56675`, started `2026-08-02T02:54:41Z`
- Executable: `/Users/m16khb/Library/pnpm/store/v11/links/@openai/codex/0.146.0-darwin-arm64/da210602f9d4b7f99dd465525dfffb316d51647d9a02bfbedbf7a3a7fd1de092/node_modules/@openai/codex/vendor/aarch64-apple-darwin/bin/codex`

## Barrier Test

Added `TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses` plus helper
`TestIssueOpsDelegationStartProcessHelper`.

The test starts four subprocesses with `exec.Command(os.Args[0],
"-test.run=^TestIssueOpsDelegationStartProcessHelper$")`. Each subprocess
prepares its child record, writes one ready marker, waits on the shared
one-shot gate file, then calls `appendIssueOpsChildRef`. The parent test waits
for all four ready markers before creating the gate and collects each
subprocess with `CombinedOutput`.

The fixture seeds 2000 existing parent refs to make the parent JSON
read-modify-write large enough for the mutation sensitivity check. Production
`withIssueOpsLock` serializes the append span; removing that outer lock loses
one or more child refs.

## Evidence

- `2026-08-02T04:20:15Z` prepare confirm: direct execution, generation 1 active,
  canonical worktree created at sealed base.
- `2026-08-02T04:23:01Z` reseed/claim: generation 2 active after hook-observed
  actor identity re-established.
- `go test ./internal/core/issueops -run TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses -count=1 -v`
  - PASS: `ok issueops/internal/core/issueops 0.690s`
- Temporary mutation: removed only the outer `withIssueOpsLock` in
  `appendIssueOpsChildRef`.
- `go test ./internal/core/issueops -run TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses -count=20 -v`
  - RED: exit 1.
  - Repeated failure: `expected 2004 child refs after process-concurrent start, got 2002`
  - Additional failure: `expected 2004 child refs after process-concurrent start, got 2003`
- Production restore: `git diff -- internal/core/issueops/issueops_delegation.go`
  produced no output after restoring the lock.
- `go test ./internal/core/issueops -run TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses -count=10 -v`
  - PASS: `ok issueops/internal/core/issueops 2.544s`
- `go test ./internal/core/issueops -count=1`
  - PASS: `ok issueops/internal/core/issueops 82.224s`

## Result

The new regression test proves that four OS processes can concurrently append
delegated child starts without losing parent child refs under production
locking, and proves the test is sensitive to the exact `appendIssueOpsChildRef`
outer lock by reproducing child-ref lost updates when that lock is removed.
