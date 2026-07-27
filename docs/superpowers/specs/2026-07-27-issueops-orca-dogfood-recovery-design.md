# IssueOps Orca Dogfood Recovery Design

## Goal

Repair the IssueOps and Orca regressions observed while starting child issue
#190, activate the repaired harness in both native hosts, and then resume the
#117 child DAG from a newly sealed parent-integration base.

## Confirmed failures

1. `execution prepare` preview skips the CWD and remote issue contract checks
   that confirm performs.
2. Orca preparation rejects an existing remote branch, but the Orca adapter
   still requires that same remote branch as `UpstreamBranch`.
3. A newly created terminal that is temporarily absent from terminal inventory
   fails immediately instead of using the existing bounded settle window.
4. The owner prompt requires a live GitHub issue read and Orca orchestration
   messages, but the lifecycle hook classifies both as unsafe while the lease
   is claimable.
5. IssueOps skill documentation still advertises the removed in-process
   `--judge llm --model` interface.
6. `sqlstore.Open` permanently caches handles for deleted temporary state
   roots, causing full IssueOps tests to accumulate thousands of SQLite
   connection goroutines and exceed five minutes.

## Working boundary

The current native coordinator owns the active #117 direct execution lease.
The repair is made in that clean parent integration worktree and committed to
`117-hexagonal-architecture-migration`. The dirty source checkout on `main`
contains unrelated changes in `execution_lease.go` and
`execution_process.go`; those bytes are not modified, staged, or reset.

#190 remains generation 1 and claimable until the repaired binary, hooks, MCP
server, and daemon have been activated and verified. Its Orca resources are
not deleted or reused without exact durable/inventory reconciliation.

## Design

### Preview and confirm parity

Move the read-only Orca preparation checks before the preview return:

- canonical prepare CWD;
- remote issue body snapshot and required acceptance/verification contract;
- branch-name availability and workspace collision checks already used by
  mode resolution.

Preview remains mutation-free and does not require a native process receipt.
Confirm adds only native-actor validation, durable intent persistence, and
external Orca mutation.

### Local-first Orca branch

Keep `ensureOrcaBranchIsFree`: an Orca worktree must start with neither a local
nor a remote branch of the requested name. Remove `UpstreamBranch` from both
Orca worktree creation paths. Orca creates the local branch directly from the
sealed `BaseHead`.

After preparation, GitHub `createLinkedBranch` creates the remote branch at the
same sealed SHA. A later normal `git push -u` can fast-forward it and establish
upstream without force or topology rewriting. This is the #163 ordering plus
the #176 base-pinned mutation, not a new branch topology.

### Bounded terminal appearance

Treat a zero PTY match as a transient state inside
`reconcileCreatedTerminal`. Continue read-only inventory polling until the
existing 12-second deadline. Multiple matches remain an immediate ambiguity
failure. If the terminal never appears or never acquires the sealed tab title,
return the existing bounded diagnostic with attempt count and duration.

### Exact owner control-plane commands

Extend the hook with two narrow classifiers:

- a read-only GitHub issue-body lookup for a literal
  `repos/<owner>/<repo>/issues/<number>` target and fixed body projection;
- Orca orchestration `send`, `ask`, and `check` forms used by the injected
  worker contract, with literal tokens and an enumerated flag set.

Arbitrary `gh api`, arbitrary GraphQL, shell substitution, redirects,
detachment, and unrelated Orca mutations remain blocked. These commands mutate
only the coordinator control plane, not the sealed Git worktree.

### Removed sqlstore roots

Keep the existing per-root handle cache and in-process span identity for live
state roots. Before opening another root, close and remove only cached handles
whose directories no longer exist. Permission or inspection errors do not
authorize eviction. This bounds ephemeral test and retired-state resources
without weakening active-root locking.

### Remote score documentation

Document the current two-stage contract:

1. run deterministic `--judge none`;
2. run `--judge prompt` to render the read-only host-agent judge prompt;
3. obtain a fresh independent result and pass the resulting JSON through
   `--judge file --judge-file`.

Remove every active IssueOps instruction that tells an agent to invoke the
removed `--judge llm` or `--model` flags.

## Activation and recovery

After targeted, full, race, build, and contract-golden verification:

1. create atomic commits and push the parent integration branch;
2. run `ah update` from the repaired canonical worktree;
3. restart the harness daemon and re-check both Codex and Claude native MCP
   registration;
4. verify the installed CLI and a fresh MCP process, not only the source tree;
5. reconcile and retire the blocked #190 generation using its exact Orca
   inventory;
6. reseed #190 from the new parent integration head and dispatch a fresh Orca
   owner.

## Verification

- RED/GREEN tests for preview CWD and remote issue parity.
- RED/GREEN adapter tests for no nonexistent upstream and absent-then-present
  terminal inventory.
- RED/GREEN lifecycle tests for exact GitHub body reads and Orca worker
  messaging, including near-miss rejection.
- Documentation search proving active IssueOps instructions contain no
  `--judge llm` or `--model glm-5-turbo`.
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- response-contract golden tests and `go build`.
- installed CLI, daemon, and fresh MCP smoke checks.
