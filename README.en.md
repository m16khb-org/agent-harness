<p align="center">
  <img src="docs/assets/issueops-hero.png" alt="Multiple AI coding agents sharing one local harness core" width="100%" />
</p>

<h1 align="center">IssueOps</h1>

<p align="center">
  Connect Codex, Claude Code, and Omo native to one execution contract,<br />
  with workflow state, safety boundaries, and evidence preserved locally
</p>

<p align="center">
  <a href="README.md">한국어</a>
  ·
  <a href="README.en.md"><strong>English</strong></a>
</p>

> [!IMPORTANT]
> issueops 0.1.0 is an actively developed local tool. The default install
> updates user-level host configuration and command shims under
> `~/.local/bin`. Review the complete plan first with
> `./install.sh --dry-run --json`.

## At a glance

**IssueOps** gives a human shell and multiple coding agents the same Go
core, CLI/MCP contracts, command policy, user-state store, and skill source
tree. It does not replace a host or auto-approve work. It preserves execution
evidence outside the host so the same work contract survives session changes.

| Capability | What it provides |
|---|---|
| Cross-host integration | Codex, Claude Code, and Omo native share one core and response contract |
| CLI, MCP, and daemon | Human-facing CLI and agent-facing MCP through a shared daemon |
| IssueOps | Durable state from problem and issue through plan, worktree, feedback, PR/MR, and cleanup |
| Project docs | Create, route, and incrementally maintain `AGENTS.md` and `.issueops/` |
| Execution safety | Workspace/cwd, write/network intent, timeout, redaction, and executable-fence policy |
| Verification and improvement | Contract, quality, self-verify, self-augment, and benchmark evidence |
| Shared skills | One `skills/` source linked into every first-party host |
| Browser QA | Functional, UI/UX, and combined web QA skills; Aside is an optional external prerequisite |

## Quick start

Requirements:

- Git
- Go 1.26.3
- Any host you plan to use: Codex, Claude Code, or Omo (optional)

From a fresh clone, review the install plan before applying it:

```bash
./install.sh --dry-run --json
./install.sh
./bin/issueops inspect --json
./bin/issueops doctor --repo . --json
```

Run the issueops quality gate:

```bash
./bin/issueops self-verify \
  --seed=100 \
  --target-score=95 \
  --llm-eval=false \
  --json
```

Refresh installed integrations from the current checkout:

```bash
git pull --ff-only
io update
io inspect --json
```

`issueops` is the canonical command; `io` is the short symlink managed by the installer. Installation fails instead of overwriting an existing `io` file or a different symlink. `io update` rebuilds the current checkout and refreshes user-level integrations. It does not run `git pull`.

`install` supports `--interactive`, `--project-local`, and
`--path-mode=auto|manual|skip`; `bootstrap` also supports `--sync`.
`--project-local` creates `.claude/skills/*`, `.mcp.json`, `.omo/skills/*`, and
`.omo/mcp.json`. Use `--adopt-command-file` only for an existing file with a
verified harness build identity.

`install` and `update` run optional upstream provisioning after native activation.
Missing Claude plugins declared in [`configs/upstream.json`](configs/upstream.json)
are installed through the Claude CLI; declared Git skills are fetched into the
issueops state cache and linked into Claude's user skill directory. Existing
entries are skipped, and upstream or network failures are reported without
failing native installation. Provisioning currently targets Claude Code only.

## Basic workflow

### Connect project docs to a repository

Preview the change, then create the managed `AGENTS.md` routing block,
`.issueops/` document family, and repository profile. Existing documents
are not replaced wholesale.

```bash
issueops project bootstrap --repo . --dry-run --json
issueops project bootstrap --repo . --json
issueops project route-docs --repo . --task "test" --json
```

`project-docs-bootstrap` owns first creation, `project-docs-update` owns
incremental maintenance, and `project-docs-optimize` restructures oversized
document families.

### Check daily health

```bash
io status --json
io doctor --repo . --json
io docs --json
io daemon status --json
```

`doctor` diagnoses installation, state, hooks, MCP, daemon, and project docs
together. `status` is the daily summary; `inspect` is the detailed installation
and native-integration projection.

### Start an IssueOps cycle

Ask first, whatever stage you are in. The command is read-only: it uses the durable
record and local observation, never the network.

```bash
issueops next --json
```

```text
stage 3/10 plan.review  cycle io-xxxx  phase plan  lease active(gen 1, self)
missing: devils_advocate_review
next: issueops devils-advocate review --id io-xxxx --reviewer-context subagent ...
exits: pause=issueops execution release --id io-xxxx --generation 1 ... abandon=issueops cleanup abandon --id io-xxxx --reason <TEXT> --preview takeover=-
```

With no cycle in the repository, `next` returns the command that starts one.

```bash
issueops start --repo "$PWD" --branch "123-short-description" --json
```

IssueOps keeps this workflow in one durable record and one generation-fenced
`Execution`. `issue` is the remote artifact/linkage step; `cleanup` runs after
`done`. The durable phase enum is
`problem|grill|plan|compatibility-review|implement|ai-slop-clean|feedback|pr|done`:

```text
problem → grill → issue → plan → compatibility-review → implement
        → ai-slop-clean → feedback → pr → cleanup
```

Remote issue/PR/MR creation and cleanup default to preview or dry-run. External
changes require explicit `--confirm` plus the matching fingerprint and actor
contract.

## Host integrations

The default installer connects three first-party host adapters to the same execution contract.

| Host | Default user-level integration |
| --- | --- |
| Codex | `~/.codex/skills/`, MCP config, `SessionStart` hook |
| Claude Code | `~/.claude/skills/`, user-scope MCP, `SessionStart` hook |
| Omo native | `~/.omo/agent/skills/`, `~/.omo/mcp.json`, lifecycle extension |

Installation is user/global by default. Project-local opt-in does not add
repo-local hook registration; host hooks and the Omo lifecycle-extension
registration remain user-level.

## Architecture

```mermaid
flowchart LR
    Codex["Codex"] --> Host["Thin host adapters<br/>skills · hooks · MCP wiring"]
    Claude["Claude Code"] --> Host
    Omo["Omo native"] --> Host
    Shell["Human shell"] --> Surface["issueops<br/>CLI · MCP proxy · daemon"]
    Host --> Surface
    Surface --> Core["Host-neutral Go core"]
    Core --> Policy["policy · guard · contracts"]
    Core --> Flow["IssueOps · loop"]
    Core --> State["SQLite user state · audit"]
    Core --> Worker["policy-gated worker"]
```

The following boundaries are deliberate:

1. Core behavior belongs in Go, not in a host plugin or hook.
2. CLI JSON, MCP responses, and daemon responses keep the same meaning.
3. Host adapters cannot bypass authentication, command policy, or workspace boundaries.
4. Hooks provide only the `SessionStart` project-doc context; they never block a tool call and do not create issues or PRs, edit files, or run tests for the agent.
5. The worker manages lifecycle jobs and policy-gated read-only evidence commands. It is not a general writable shell runner.

## Core surfaces

| Area | Representative commands | Purpose |
| --- | --- | --- |
| Install and refresh | `install`, `update`, `bootstrap`, `version` | Refresh the binary, skills, hooks, and MCP wiring; report version |
| Health and docs | `inspect`, `status`, `doctor`, `docs` | Inspect installation, daemon, state, and project docs |
| Safety and quality | `policy`, `guard`, `quality`, `verify-work`, `trace`, `contract`, `api-doc`, `preflight` | Check execution policy, change quality, evidence, public contracts, and pre-commit repository state |
| Workflows | `issueops`, `loop`, `gates`, `channel` | Manage durable workflows, task gate ledgers, and cross-session message channels |
| Docs and hooks | `project`, `hook` | Create, route, and maintain project docs; host `SessionStart` context hook entry point |
| State and runtime | `state`, `daemon`, `mcp`, `worker` | Manage user state, the MCP backend, and constrained local jobs |
| Improvement and research | `self-verify`, `self-augment`, `web-fetch` | Verify the harness, record improvements, and fetch public web content resiliently |

At runtime, the daemon supports `start|status|stop`; the worker supports
`enqueue`, read-only `run`, `status`, `list`, `cleanup-stuck`, and `cancel`;
`mcp cleanup` removes stale proxy processes in dry-run or apply mode.

Read the complete CLI and MCP contracts from the running binary:

The current checkout's response-contract schema pins 29 top-level CLI commands and 51 MCP tools.

```bash
issueops --help
issueops contract schema --json
issueops contract check --json
```

## Current verified state

These values come from the running binary's contract and quality projection.
They are not a separately maintained README score.

| Verification axis | Current state |
| --- | --- |
| Public contract | 29 CLI commands, 51 MCP tools |
| Quality collection | `ok` |
| Quality health | `needs_attention` |
| Quality gate | `report_only` |
| Open verification/augmentation candidates | 0 / 0 |
| Tracked quality candidates | 0 |
| Active audit P1/P2 findings | 0 |

`needs_attention` reports 3 low-coverage packages and branch-complexity debt but
does not block the gate. On collection failure,
`collection_status=error`, `health_status=unknown`, and `gate_status=block`.

Recompute the current projection with:

```bash
issueops contract schema --json
issueops quality inspect --json
issueops benchmark run \
  --fixtures testdata/issueops/fixtures \
  --json
```

`quality inspect` reports `collection_status`, `health_status`, and `gate_status` separately. Collection failure blocks the gate; non-blocking debt such as existing low coverage remains `report_only`.

## IssueOps

IssueOps records conversational work context as issues, plans, worktrees, feedback, and verification evidence so the same work contract survives session and host changes.

Users see ten stages, one skill each. `issueops next` owns the stage
decision, so any agent on any host gets the same answer.

| Stage | Skill | What it does |
|---|---|---|
| 1 confirm and create the issue | `issueops-create-issue` | settle the contract through research and blocking questions, then publish the issue |
| 2 prepare the branch | `issueops-prepare` | pin the base SHA and link the branch to the issue |
| 3 documents, plan, review, handoff | `issueops-plan` | read the operating docs, write the plan, pass review, create the implementation session |
| 4 implement | `issueops-implement` | TDD inside the canonical worktree |
| 5 AI slop clean | `issueops-clean` | remove residue and seal the change set |
| 6 project documents | `issueops-docs` | record decisions and pitfalls, then re-seal |
| 7 verify | `issueops-verify` | re-verify, review, and prove readiness without touching a file |
| 8 commit and push | `atomic-commit-push` | commit the sealed change set |
| 9 publish and complete | `issueops-create-pr`, `issueops-complete` | open the draft, seal the completion evidence |
| 10 post-merge cleanup | `issueops-cleanup` | close the issue, reclaim the worktree and branch |

`issueops-abandon` owns the exit from any stage: pause releases the lease, abandon
closes the draft PR/MR, the issue, and the remote branch as chosen, then deletes the
record. Shared procedures live in `issueops-review` (adversarial review),
`gates-ledger` (gate ledgers), and `issueops-remote-write` (remote write protocol).

The durable phase enum is unchanged. `issue` is the linkage step, not a durable
phase; `cleanup` runs after `done`:

```text
problem → grill → issue → plan → compatibility-review → implement
        → ai-slop-clean → feedback → pr → cleanup
```

IssueOps records remote issues, branches/worktrees, design reviews, Brooks
devil's-advocate reviews, plan links, and execution decisions as durable evidence
and gates for each phase. Its CLI/MCP owns fail-closed checks and mutation
fences; hooks inject only static project-doc context.

Remote issue creation is a dry run by default. The `--confirm` path stores project authority, request digest, and operation marker as a durable intent before invoking the provider. An ambiguous result blocks automatic retry; `reconcile-issue` adopts only one live candidate from the same project.

Inspect a cycle with
`issueops status --id "<cycle id>" --json`. `execution prepare`
previews the mode and readiness fingerprint; its returned `next_command`
performs the confirmed transition. `direct` and `orca` are execution adapters,
while IssueOps remains the durable authority.

Without `--confirm`, `create-issue` prints a preview and does not create an intent. Use `reconcile-issue` only when a confirmed remote call has left a durable intent with an ambiguous result; adopting a candidate requires a separate `--confirm`. See the [IssueOps provider guide](.issueops/operations/guides/issueops-providers.md) for complete command and provider constraints.

Each cycle's tracked plans/specs/gates and ignored sealed artifacts are
namespaced under `.issueops/issues/<provider-issue-number>/`.
`cleanup finish` rechecks its preview/fingerprint, stops worktree processes and
Orca terminals, then removes the worktree, branch, and record in order.

See [`skills/issueops/SKILL.md`](skills/issueops/SKILL.md) and the [operations map](.issueops/OPERATIONS.md) for the complete cycle and remote-artifact rules.

## Skills

[`skills/`](skills/) is the single source of truth for 33 shared skills. The installer links that directory into each host's user-level skill path.

- Planning and critique: `implementation-planning`, `requirements-analysis`, `design-review`, `prompt-engineering`
- Execution and verification: `verified-execution`, `issueops-debugging`, `algorithm-optimization`, `database-design`, `code-quality-metrics`
- Research and collaboration: `web-research`, `meeting-notes`, `slack-delegate`
- Git and workflow operations: `git-operations`, `atomic-commit-push`, `gitlab-usecase`
- IssueOps stages: `issueops` (router), `issueops-create-issue`, `issueops-prepare`, `issueops-plan`, `issueops-implement`, `issueops-clean`, `issueops-docs`, `issueops-verify`, `issueops-create-pr`, `issueops-complete`, `issueops-cleanup`, `issueops-abandon`
- IssueOps shared: `issueops-review`, `gates-ledger`, `issueops-remote-write`, `issueops-sync-issue`, `issueops-sync-pr`
- Project docs: `project-bootstrap`, `project-docs-bootstrap`, `project-docs-update`, `project-docs-optimize`
- Browser QA: `aside-functional-qa`, `aside-visual-qa`, `aside-web-qa`, `read-public-artifact`
- Code review: `pr-review`, `review-agent-feedback`
- Operational improvement: `io-update`, `self-verify`, `self-augment`, `stability-audit`
- Korean writing: `fluent-korean`
- Diagrams and visualization: `diagram-design`

Each skill's `SKILL.md` is its authoritative usage contract.

The 12 pioneer skills are evaluated across primary, boundary, and operational cases. Committed cases are reproduction inputs, not answer fixtures. Execution receipts, case hashes, and semantic verdicts live under [`testdata/pioneer-holdouts/`](testdata/pioneer-holdouts/).

## Local data and safety boundaries

- Default installation updates user-level host configuration only. Target repositories change only through explicit bootstrap or project-local opt-in.
- Runtime state is stored in SQLite under `~/.local/state/issueops/` by default and can be isolated with `ISSUEOPS_STATE_DIR`.
- Command execution follows policy for workspace root, cwd, write/network/shell intent, timeouts, and redaction.
- MCP tool arguments reject unknown fields and missing or incorrectly typed fields against the published schema.
- Executable shell fences are checked without execution for syntax, swallowed failures, destructive commands, dynamic shells, and symlink bypasses.
- Raw secrets do not belong in documentation, state responses, audit logs, or test fixtures.
- Native install, update, readiness, and self-verification do not depend on
  external tools; optional Claude Code upstream provisioning runs after
  activation and is non-fatal.
- Integrations such as Orca supervised execution are optional adapters; IssueOps remains the durable authority.

## Repository map

```text
cmd/issueops/          Composition root and CLI/MCP/daemon/hook entry points
internal/contract/    Versioned DTOs shared by transports and persistence
internal/domain/      Pure rules, reducers, and classifiers
internal/application/ Use cases that compose domain logic with ports
internal/port/        External capability interfaces and error contracts
internal/adapter/     Host, filesystem, process, database, and network boundaries
internal/architecture/ Production import-graph fitness tests
configs/              Codex, Claude Code, and Omo configuration templates
skills/               Skill source shared by every host
.issueops/       Architecture, operations, testing, and ADR project docs
scripts/              Install, release, smoke, and validation scripts
docs/                 Supporting documents and assets
openwiki/             Code documentation wiki (OpenWiki) quickstart and pages
```

## Release and rollback

issueops is an actively developed `0.1.0` project. **Current distribution
decision**: prefer a tarball or manual archive and defer Homebrew until the
release gates are sufficiently validated. The release matrix cross-builds
`darwin/arm64`, `darwin/amd64`, `linux/amd64`, and `linux/arm64`.

Release verification refreshes local build artifacts, while rollback changes
the checkout and installed state. Review the
[release reproducibility and rollback criteria](.issueops/operations/release-reproducibility.md)
before either operation. This README does not provide destructive rollback
commands.

## Verification

Documentation-only changes still run the project's minimum gate:

```bash
./bin/issueops contract check --json
./bin/issueops docs --json
./bin/issueops inspect --json
./bin/issueops quality inspect --json
./bin/issueops self-verify \
  --seed=100 \
  --target-score=95 \
  --llm-eval=false \
  --json
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
git diff --check
```

For Go or public-contract changes:

```bash
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
```

See [`.issueops/TESTING.md`](.issueops/TESTING.md) for change-specific verification requirements.

## Troubleshooting

| Symptom | What to check |
|---|---|
| `io` is missing after installation | Open a new shell or refresh its command cache, then verify `~/.local/bin` is on PATH |
| Installation refuses an existing `io` or `issueops` | This is the installer protecting unrelated files; inspect the conflict path in `--dry-run --json` |
| A host does not show a new MCP tool | Run `io update`, reopen the host session, then inspect catalog/config state with `io inspect --json` |
| Daemon health is abnormal | Run `io doctor --repo . --json` and `io daemon status --json` |
| Self-verify appears idle | Add `--progress=jsonl` to display per-step heartbeat events |
| Project docs are stale | Update one document with `project-docs-update`; use `project-docs-optimize` for structural problems |

## Project docs

| Document | Purpose |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | Repository rules and verification priorities |
| [`.issueops/CONSTITUTION.md`](.issueops/CONSTITUTION.md) | Instruction hierarchy and safety principles |
| [`.issueops/ARCHITECTURE.md`](.issueops/ARCHITECTURE.md) | Component boundaries and responsibilities |
| [`.issueops/OPERATIONS.md`](.issueops/OPERATIONS.md) | Install, host, CLI/MCP, and runtime operations map |
| [`.issueops/TESTING.md`](.issueops/TESTING.md) | Test and verification gates |
| [`.issueops/operations/quality-dashboard.md`](.issueops/operations/quality-dashboard.md) | Quality projections and pioneer evidence interpretation |
| [`.issueops/ADR.md`](.issueops/ADR.md) | Structural decisions, rationale, and rejected alternatives |
| [`openwiki/quickstart.md`](openwiki/quickstart.md) | OpenWiki entry point for code structure and workflows |

Installation and operational procedures are split into [install](.issueops/operations/install.md), [hosts](.issueops/operations/hosts.md), [CLI/MCP](.issueops/operations/cli-and-mcp.md), and [verification](.issueops/operations/verification.md) guides.

## License

MIT. See [`LICENSE`](LICENSE).
