---
type: Reference
title: Source Map
description: Navigable map of the agent-harness repository tree, with key directories, entrypoints, and source files organized by domain.
tags: [source-map, navigation, repository]
---

# Source Map

This page maps the repository structure to functional areas, helping engineers and agents find the right starting point.

## Top-Level Layout

```
agent-harness/
├── cmd/harness/              Single Go binary — CLI, MCP, daemon, hook entrypoints
├── internal/core/            Host-neutral use cases, policy, state, workflow
├── internal/port/            Core interfaces and DTOs (no concrete adapters)
├── internal/adapter/         CLI, MCP, host, provider, Orca boundary adapters
├── internal/testsupport/     Shared test helpers (stdout capture, fixtures)
├── configs/                  Codex and Claude config templates
├── skills/                   21 shared skills (single source of truth)
├── .agent-harness/           Project docs: architecture, operations, testing, ADR
├── scripts/                  Install, release, smoke, validation scripts
├── testdata/                 IssueOps benchmark fixtures
├── docs/                     Auxiliary docs and assets
├── go.mod / go.sum           Go 1.26 module (agent-harness)
├── install.sh                First-time install entry point
├── README.md / README.en.md  Project documentation (Korean / English)
├── AGENTS.md                 Root agent rules and verification priorities
└── CLAUDE.md                 Claude Code-specific entry
```

## cmd/harness/ — Binary Entrypoints

| Subdirectory | Role |
|-------------|------|
| `main.go` | Calls `harnessapp.RunRootCommand(os.Args[1:])` |
| `harnessapp/` | Root command construction, dependency wiring, facades |
| `rootcmd/` | Command struct and dispatch |
| `issueopscli/` | IssueOps CLI subcommands (start, execution, cleanup, benchmark) |
| `mcpcli/` | MCP SDK server, transport, tool dispatch, resources |
| `hookcli/` | Lifecycle hooks (pre/post tool use, stop, user prompt) |
| `workercli/` | Worker CLI (enqueue, status, draft-wiki) |
| `daemoncli/` | Daemon start/stop/status |
| `installcli/` | Native install CLI |
| `updatecli/` | Bootstrap and update CLI |
| `statecli/` | State read/write/list/prune/doctor/migrate |
| `policycli/` | Policy check and fake-run |
| `basiccli/` | Inspect, preflight, doctor, docs, trace |
| `loopcli/` | Loop start/record-attempt/status/stop |
| `contractcli/` | Contract schema and check |
| `selfworkflow/` | Self-verify and self-augment loops |
| `apidoc/` | API documentation generation and review |
| `contractgolden/` | Golden contract test harness |
| `testdata/` | Golden files: usage.golden.txt, response_contracts.golden.json |

## internal/core/ — Domain Logic

### Facades (public surface)

| File | Exposes |
|------|---------|
| `issueops_facade.go` | IssueOps lifecycle, phase, execution, readiness |
| `issueops_remote_facade.go` | Remote artifact creation and verification |
| `workflow_facade.go` | Loop contract management |
| `policy_facade.go` | Command policy evaluation |
| `state_trace_facade.go` | State operations and trace |
| `utility_facade.go` | Hook failure stats, hook metrics |
| `draft_wiki_facade.go` | Draft wiki queue and processing |
| `project_doc_facade.go` | Project bootstrap and doc routing |
| `loop_facade.go` | Loop delegation |

### Key Subpackages

| Subpackage | Role |
|-----------|------|
| `issueops/` | Issue-driven workflow: phase machine, execution lease, readiness, cleanup |
| `issueops/model/` | Core types: IssueOpsRecord, Execution, WriteLease, phases |
| `lifecycle/` | Pre/post tool-use guards, project lifecycle, doc upkeep |
| `state/` | State read/write/list/prune/doctor/migrate |
| `sqlstore/` | SQLite key-value store with BEGIN IMMEDIATE spans |
| `policy/` | Command classification and evaluation |
| `guard/` | Code anti-pattern detection (block/warn/review) |
| `worker/` | Local job lifecycle and draft-wiki queue |
| `inspect/` | Installation diagnostics |
| `preflight/` | Readiness checks |
| `docs/` | Document indexing |
| `install/` | Native install orchestration |
| `operationalhealth/` | Cross-system health classifier |
| `toolconformance/` | Cross-host tool contract validation |
| `doctor/` | Comprehensive diagnostics |

## internal/port/ — Interfaces

| File | Defines |
|------|---------|
| `install.go` | `HostInstaller` interface, `NativeInstallRequest/Result` |
| `provider.go` | `IssueProvider` interface for GitHub/GitLab |
| `execution_workspace.go` | `ExecutionWorkspaceProvisioner`, workspace access probes |
| `orca.go` | Orca provisioner and inspector interfaces |
| `tool_conformance.go` | Tool conformance probe interfaces |

## internal/adapter/ — Boundary Implementations

| Subdirectory | Role |
|-------------|------|
| `cli/` | Command catalog (`usage.go`), flag parsing |
| `mcp/` | MCP tool schemas and dispatch groups (`catalog.go`) |
| `codex/` | Codex user-level skill/MCP/hook installation |
| `claude/` | Claude user-level skill/MCP/hook installation |
| `provider/github/` | GitHub issue/PR operations via `gh` CLI |
| `provider/gitlab/` | GitLab issue/MR operations via `glab` CLI |
| `orca/` | Orca supervised execution adapter |
| `hook/` | Host-specific hook output formatters |
| `hostprobe/` | Isolated Codex/Claude live probes |
| `operationalhealth/` | Read-only health inventory collector |
| `installutil/` | Shared install plan builder |

## skills/ — Pioneer Skills

21 skills, each with `SKILL.md` and references. Key operational skills:

| Skill | Role |
|-------|------|
| `issueops/` | Workflow orchestration (references for each phase) |
| `turing/` | Evidence-bound execution hub |
| `torvalds/` | Git operations (atomic commit, bisect, worktree) |
| `self-verify/` | Harness quality verification |
| `self-augment/` | Improvement candidate identification |
| `stability-audit/` | Operational health audit |
| `atomic-commit-push/` | Commit policy enforcement |

## scripts/ — Operational Scripts

| Script | Purpose |
|--------|---------|
| `install-native.sh` | Native skill/MCP/hook installation |
| `release-repro-smoke.sh` | Release reproducibility smoke test |
| `release-build-matrix.sh` | Release build matrix verification |
| `validate-skill.py` | Skill structure validation |
| `remote-artifact-gate-smoke.sh` | Remote artifact gate smoke test |
| `sync-glab-mcp.sh` | GitLab MCP sync helper |

## .agent-harness/ — Project Documentation

| File | Content |
|------|---------|
| `CONSTITUTION.md` | Instruction hierarchy, safety/accuracy principles |
| `ARCHITECTURE.md` | Component boundaries and responsibilities |
| `OPERATIONS.md` | Install, host, CLI/MCP, runtime operations map |
| `TESTING.md` | Test and verification gates |
| `CONVENTIONS.md` | Coding conventions, package boundaries |
| `ADR.md` | Structural decisions, rationale, rejected alternatives |
| `CAUTIONS.md` | Failure/regression prevention rules |
| `COMMIT_POLICY.md` | Conventional commit policy |
| `TECH_STACK.md` | Technology stack and commands |
| `operations/` | Focused operation docs (install, hosts, cli-and-mcp, verification) |
