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
├── internal/domain/          Pure domain logic: state machines, classifiers, transitions
├── internal/application/     Use case orchestration
├── internal/contract/        Shared DTOs and vocabulary types across layers
├── internal/port/            Port interfaces (no concrete adapter types)
├── internal/adapter/         Boundary implementations: issueops, lifecycle, cli, mcp, host, provider, orca
├── internal/architecture/    AST-level dependency direction and convention tests
├── internal/testsupport/     Shared test helpers (stdout capture, fixtures)
├── configs/                  Codex and Claude config templates
├── skills/                   Shared skills (single source of truth)
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
| `harnessapp/` | Root command construction, dependency wiring, adapter tails |
| `rootcmd/` | Command struct and dispatch |
| `issueopscli/` | IssueOps CLI subcommands (start, execution, cleanup, benchmark) |
| `mcpcli/` | MCP SDK server, transport, tool dispatch, resources |
| `hookcli/` | Lifecycle hooks (context injection, catalog dispatch) |
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
| `validationcli/` | Native integration contract validation |
| `qualitycli/` | Quality gate |
| `riskqa/` | Risk QA tier |
| `webfetchcli/` | Resilient public web fetch |
| `contractgolden/` | Golden contract test harness |
| `apidoc/` | API documentation generation and review |
| `commandstep/` | Command step composition for self-verify pipeline |
| `pathutil/` | Shared path utilities |
| `projectcli/` | Project bootstrap, draft-wiki, doc routing CLI |
| `statuscli/` | Daemon and session status CLI |
| `testdata/` | Golden files: usage.golden.txt, response_contracts.golden.json |

## internal/domain/ — Pure Domain Logic

| Subpackage | Role |
|-----------|------|
| `issueops/` | IssueOps phase machine, phase types, intent class |
| `issueopslease/` | Lease state machine, claim/release/reseed/resume/reconcile transitions |
| `issueopscompletion/` | Completion gate decisions |
| `issueopspreparation/` | Preparation planner gates and decisions |
| `issueopslinkedbranch/` | Linked branch state classification (healthy, orphan, mismatched) |
| `issueopspublication/` | Publication decisions |
| `issueopsremote/` | Remote artifact classification |
| `lifecycle/` | Lifecycle state types and project state |
| `policy/` | Policy decision and redaction logic |
| `guardpattern/` | Guard anti-pattern definitions |
| `commandparse/` | Command parsing and read-only classification |
| `commandguard/` | Command guard logic |
| `operationalhealth/` | Health classifier (findings, gate residue, inventory) |
| `state/` | State validation logic |
| `toolconformance/` | Tool conformance domain types |
| `mcp/` | MCP domain types |
| `hook/` | Hook domain types |
| `prompt/` | Prompt domain types |
| `judgement/` | Judgement domain types |
| `nextaction/` | Next-action suggestion domain |
| `qualitycatalog/` | Quality catalog domain types |
| `contextregion/` | Context region domain types |
| `draftmeta/` | Draft metadata domain types |
| `artifacttemplate/` | Artifact template domain types |
| `remoteparse/` | Remote artifact parsing domain |
| `searchrouting/` | Search routing domain types |
| `shelltoken/` | Shell token domain types |
| `statepath/` | State path domain types |
| `stringlist/` | Sorted string list domain |
| `traceclassification/` | Trace classification domain |
| `webfetch/` | Web fetch URL validation domain |
| `projectdoc/` | Project document domain types |
| `auditid/` | Audit ID generation domain |

## internal/application/ — Use Case Orchestration

| Subpackage | Role |
|-----------|------|
| `issueopslease/` | Lease orchestration: reconcile service, resume, claim/release coordination |
| `issueopscompletion/` | Completion service (orchestrates done transition, no inline task settlement) |
| `issueopspreparation/` | Preparation service |
| `issueopsprovenance/` | Provenance tracking |
| `issueopspublication/` | Publication service |
| `state/` | State use case |
| `webfetch/` | Web fetch use case |
| `nativeactivation/` | Native activation use case |

## internal/contract/ — Shared DTOs

Holds DTOs and vocabulary types shared across all layers. Key subpackages mirror domain areas: `issueops/`, `issueopslease/`, `issueopscompletion/`, `issueopspreparation/`, `lifecycle/`, `policy/`, `state/`, `install/`, `operationalhealth/`, `mcp/`, `hook/`, `commandparse/`, and more.

## internal/port/ — Port Interfaces

| File | Defines |
|------|---------|
| `install.go` | `HostInstaller` interface, `NativeInstallRequest/Result` |
| `provider.go` | `IssueProvider` interface for GitHub/GitLab |
| `execution_workspace.go` | `ExecutionWorkspaceProvisioner`, workspace access probes |
| `execution_dependencies.go` | Execution dependency bundles, `TransactionalRecordStore` |
| `orca.go` | Orca provisioner, inspector, and task settlement interfaces |
| `orca_context.go` | `OrcaRunInventoryReader` — Run-scoped inventory reader port |
| `tool_conformance.go` | Tool conformance probe interfaces |

## internal/adapter/ — Boundary Implementations

| Subdirectory | Role |
|-------------|------|
| `cli/` | Command catalog (`usage.go`), flag parsing |
| `mcp/` | MCP tool schemas and dispatch groups (`catalog.go`) |
| `codex/` | Codex user-level skill/MCP/hook installation |
| `claude/` | Claude user-level skill/MCP/hook installation |
| `issueops/` | IssueOps adapter: execution, lease, cleanup, delegation, linked branches, regress |
| `lifecycle/` | Lifecycle execution guard, worktree guard, project state, doc upkeep |
| `provider/github/` | GitHub issue/PR operations via `gh` CLI |
| `provider/gitlab/` | GitLab issue/MR operations via `glab` CLI |
| `orca/` | Orca supervised execution adapter, Run inventory reader |
| `policy/` | Policy catalog, evaluation, command classification |
| `guard/` | Guard anti-pattern scanner |
| `install/` | Native install orchestration, build-generation skew detection |
| `installutil/` | Shared install plan builder, hook generation |
| `operationalhealth/` | Read-only health inventory collector (Run snapshot reuse) |
| `inbound/` | Inbound adapters (issueops lease claim/release/reseed/resume, completion) |
| `outbound/` | Outbound adapters (sqlstore, issueops lease/completion/preparation effects) |
| `inspect/` | Installation diagnostics |
| `doctor/` | Comprehensive diagnostics |
| `worker/` | Local job lifecycle and draft-wiki queue |
| `looprun/` | Loop run adapter |
| `preflight/` | Readiness checks |
| `docs/` | Document indexing |
| `hostprobe/` | Isolated Codex/Claude live probes |
| `toolconformance/` | Cross-host tool contract validation |
| `audit/` | Audit record adapters |
| `commitsuggest/` | Commit suggestion adapter |
| `failurecause/` | Failure cause analysis adapter |
| `gitworktree/` | Git worktree operations adapter |
| `hookfailure/` | Hook failure analysis adapter |
| `hookmetrics/` | Per-event hook telemetry recording and aggregation |
| `hookprompt/` | Hook prompt adapters |
| `lintdiagnose/` | Lint diagnosis adapter |
| `projectbootstrap/` | Project bootstrap adapter |
| `projectdoc/` | Project document adapters |
| `projectdocs/` | Project document file-state adapters |
| `remoteartifact/` | Remote artifact adapter |
| `repopath/` | Repository path adapter |
| `skillcontract/` | Safety- and contract-critical skill phrase pinning |
| `trace/` | Trace adapter |

## skills/ — Pioneer Skills

20 skills, each with `SKILL.md` and references. Key operational skills:

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
