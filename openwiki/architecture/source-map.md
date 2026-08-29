---
type: architecture-source-map
title: Source Map
description: Maps every major source tree of agent-harness to its responsibility, allowed dependencies, and forbidden imports, with the capability-vertical pattern worked through issueopslease and issueopspublication.
tags: [architecture, source-map, hexagonal, go, package-boundaries, composition-root, capability-vertical]
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T12:09:25.684Z
sources:
  - id: openwiki-source-42b90bfa150819efc9065f4f
    resource: repo://.agent-harness/ARCHITECTURE.md
  - id: openwiki-source-8d31c78479f6d54f47812b54
    resource: repo://.agent-harness/architecture/hexagonal-core.md
  - id: openwiki-source-9be88b82096f247b8b24dc5f
    resource: repo://.agent-harness/architecture/issueops.md
  - id: openwiki-source-e31e8beb2f56c36939086f18
    resource: repo://.agent-harness/architecture/runtime.md
  - id: openwiki-source-c1374c4e652908cbc7ddf941
    resource: repo://.agent-harness/CONVENTIONS.md
  - id: openwiki-source-ca8cd604532ed4e7985b30d4
    resource: repo://.agent-harness/conventions/go-and-packages.md
  - id: openwiki-source-62c942b8d54d1a15a962a832
    resource: repo://.agent-harness/TECH_STACK.md
  - id: openwiki-source-f5a489e5822d87c0b8fc66ef
    resource: repo://.mcp.json
  - id: openwiki-source-8037e2358a2c4f9b2c722a11
    resource: repo://AGENTS.md
  - id: openwiki-source-6bdc7639e06c311468b3d34d
    resource: repo://cmd/harness/contractgolden/contract_golden_test.go
  - id: openwiki-source-7547bacc40580875dcf1f46b
    resource: repo://cmd/harness/daemoncli/daemon_proxy.go
  - id: openwiki-source-91d7e73a4e2c7f91490f9bc6
    resource: repo://cmd/harness/harnessapp/app.go
  - id: openwiki-source-ed2af7422c5c4212bf5849ed
    resource: repo://cmd/harness/harnessapp/cli_facade.go
  - id: openwiki-source-1eb0bc0c0af2651c30aad2a3
    resource: repo://cmd/harness/harnessapp/install_wiring.go
  - id: openwiki-source-5c8b8172ae8279b2d4e8ca03
    resource: repo://cmd/harness/harnessapp/issueops_claim_wiring.go
  - id: openwiki-source-fa482accfe87e99cb5cd2802
    resource: repo://cmd/harness/harnessapp/issueops_publication_wiring.go
  - id: openwiki-source-f35881ba4ad4a7ca820acdf3
    resource: repo://cmd/harness/harnessapp/policy_preflight_wiring.go
  - id: openwiki-source-5c58197e97da783a8d01647b
    resource: repo://cmd/harness/harnessapp/response_contract_golden_test.go
  - id: openwiki-source-9e37307cec8ebb29091064ff
    resource: repo://cmd/harness/harnessapp/root_command_facade.go
  - id: openwiki-source-e0c51879cfe3e0474d8b16fa
    resource: repo://cmd/harness/harnessapp/wire_test.go
  - id: openwiki-source-4d6d4997a2e667fb6e6a7c29
    resource: repo://cmd/harness/main.go
  - id: openwiki-source-41df5b9da259ad094d84bd77
    resource: repo://cmd/harness/testdata/mcp_tools.golden.json
  - id: openwiki-source-01d50c670a87d01a0f199660
    resource: repo://configs/upstream.json
  - id: openwiki-source-7d4f2b30ea7da36e9f3f3139
    resource: repo://internal/adapter/codex/install.go
  - id: openwiki-source-6eb8dd842645fa885b2408e5
    resource: repo://internal/adapter/inbound/issueopslease/claim.go
  - id: openwiki-source-78edd484a6aadd35dec1bce2
    resource: repo://internal/adapter/inbound/issueopspublication/create.go
  - id: openwiki-source-b6ebee518991653bf5cb3f24
    resource: repo://internal/adapter/install_contract_matrix_test.go
  - id: openwiki-source-e5a9db04ba86d33ec5e11a29
    resource: repo://internal/adapter/install/install.go
  - id: openwiki-source-76264c78a3c65d168f1e87e7
    resource: repo://internal/adapter/issueops/adapter_dependencies.go
  - id: openwiki-source-7e568a7db3bbab14b7be042b
    resource: repo://internal/adapter/issueops/gatesgate/gates_gate.go
  - id: openwiki-source-3ca65bdc26f54aad04d682fa
    resource: repo://internal/adapter/issueops/package.go
  - id: openwiki-source-d01cb632c70057b88e6239ec
    resource: repo://internal/adapter/mcp/conformance_probe.go
  - id: openwiki-source-6691e166b95e6624bc89a042
    resource: repo://internal/adapter/outbound/issueopslease/claim_context.go
  - id: openwiki-source-0c917d197e5ce26cea16d91d
    resource: repo://internal/adapter/outbound/issueopslease/sqlite.go
  - id: openwiki-source-1bb7e294c7243e8798131d47
    resource: repo://internal/adapter/outbound/sqlstore/sqlstore.go
  - id: openwiki-source-cd609c5179ac7a91351e5aa8
    resource: repo://internal/adapter/provider/resolve.go
  - id: openwiki-source-33c460260b93fb7c68c37061
    resource: repo://internal/adapter/worker/read_only.go
  - id: openwiki-source-cb27137d6067b15b7df1ad17
    resource: repo://internal/application/issueopslease/claim.go
  - id: openwiki-source-edfa210069f0f94be519966e
    resource: repo://internal/application/issueopslease/ports.go
  - id: openwiki-source-6c72f968c872ab2562dae01b
    resource: repo://internal/application/issueopspublication/ports.go
  - id: openwiki-source-b78b8f957dae0c4e1dae1fcc
    resource: repo://internal/architecture/dependency_test.go
  - id: openwiki-source-c7bccc6b7868c14c0402bbe0
    resource: repo://internal/architecture/documentation_test.go
  - id: openwiki-source-925d89e2a006ae304b835cfc
    resource: repo://internal/architecture/errors_astype_test.go
  - id: openwiki-source-0135852aacfa1a328a852743
    resource: repo://internal/architecture/issueops_base_sync_boundary_test.go
  - id: openwiki-source-adc0ca23f746fdf674324951
    resource: repo://internal/architecture/issueops_record_store_test.go
  - id: openwiki-source-46d14090e031dfe22f951b30
    resource: repo://internal/architecture/orphan_package_test.go
  - id: openwiki-source-97a2e78e6f0ff54daa6c2255
    resource: repo://internal/architecture/ownership_manifest_test.go
  - id: openwiki-source-eb372b3b778b96336f271c53
    resource: repo://internal/contract/issueopslease/stable_v1.go
  - id: openwiki-source-2f5010be48208ed7dc81e552
    resource: repo://internal/domain/auditid/audit_id.go
  - id: openwiki-source-f43a2646d3dfff930a7d4ea4
    resource: repo://internal/domain/cli/usage.go
  - id: openwiki-source-57d4ae7ba28034e13e90c4c3
    resource: repo://internal/domain/issueopslease/claim.go
  - id: openwiki-source-5098bb119460c63daa208f1e
    resource: repo://internal/domain/issueopspublication/decision.go
  - id: openwiki-source-56597a0730aa3ea748051cf7
    resource: repo://internal/domain/mcp/catalog.go
  - id: openwiki-source-a4b853cf3b3dff0668c7171b
    resource: repo://internal/domain/pioneerskill/catalog.go
  - id: openwiki-source-bb19bb9c1aa23ca78ecdb01f
    resource: repo://internal/holdoutdeleak/deleak_test.go
  - id: openwiki-source-c0418c35a633373a6a133212
    resource: repo://internal/port/install.go
  - id: openwiki-source-5b616f73f15968b4475aceb8
    resource: repo://internal/port/orca.go
  - id: openwiki-source-022cca5b7f584ee0482eccfc
    resource: repo://internal/port/provider.go
  - id: openwiki-source-5bd8b36a5de0083ea830b44c
    resource: repo://internal/port/transactional_record_store.go
  - id: openwiki-source-705c9934e76b869b37536118
    resource: repo://internal/testsupport/capture.go
  - id: openwiki-source-a116562c92a178b701505b04
    resource: repo://scripts/install-native.sh
---

# Source Map

This page maps each major source tree to its one-line responsibility, the
dependencies it may use, and the imports it must not take — so a change lands
in the right layer the first time. The rules summarized here are owned by
[`.agent-harness/architecture/hexagonal-core.md`](../../.agent-harness/architecture/hexagonal-core.md)
and enforced mechanically by the zero-baseline import ratchet in
`internal/architecture` (see [Dependency Ratchet](dependency-ratchet.md) for
the enforcement details).

Related pages: [Architecture Overview](overview.md),
[Dependency Ratchet](dependency-ratchet.md),
[Contract Surface](../concepts/contract-surface.md),
[State and SQLStore](../concepts/state-and-sqlstore.md),
[IssueOps Cycle](../workflows/issueops-cycle.md).

## The layer table

| Tree | Responsibility | Must not |
| --- | --- | --- |
| `cmd/harness` | composition root (`harnessapp`) plus per-command CLI packages; flag/parse/dispatch, MCP stdio/JSON-RPC wiring, daemon lifecycle | duplicate host-specific policy or domain judgment |
| `internal/contract/<capability>` | versioned DTOs, `schema_version` markers, and error vocabulary shared by CLI/MCP/state | judgment logic and any I/O |
| `internal/domain/<capability>` | pure rules, reducers, classifiers; the CLI and MCP catalogs | adapters/`cmd`, filesystem/process/DB I/O; clock is injected by default (`internal/domain/auditid` timestamp IDs are the one documented exception) |
| `internal/application/<capability>` | use cases that combine contract, domain, and narrow port interfaces | concrete adapters and transport dependencies |
| `internal/port` | external capability interfaces and typed error contracts | anything outside `internal/contract` (and other port packages) |
| `internal/adapter/inbound/<capability>` | capability request → application call mapping | outbound adapters directly |
| `internal/adapter/outbound/<capability>` | state, SQL, webfetch, and IssueOps persistence implementations | transport policy or domain judgment duplication |
| `internal/adapter/*` (concrete) | host installers, process/git/provider/worker I/O, tooling adapters | imported anywhere except `cmd/harness/harnessapp` (zero legacy edges) |
| `internal/architecture` | test-only fitness boundary over the production import graph | production code never imports it |

Everything below expands a row, names the boundary-relevant packages only, and
finishes with the capability-vertical pattern that most new work should follow.

## `cmd/harness` — the composition root and CLI transport packages

`cmd/harness/main.go` is a three-line shell: it calls
`harnessapp.RunRootCommand(os.Args[1:])` and exits with the returned code.
`cmd/harness/harnessapp` is the **only composition root** in the module — the
ratchet's `isCompositionRoot` allows exactly this path to import concrete
adapters, and any other concrete-adapter edge is a legacy edge, and the legacy
baseline is zero. Wiring is explicit, not `init()`-driven:
`wireDependencies()` runs once under `sync.Once` at startup and assigns
implementations into leaf packages' function-variable slots; `harnessapp`'s
own `TestMain` calls the same function so package tests exercise production
wiring, not stubs.

`harnessapp` is also a facade library, not a command implementation: each
command's flag/parse/render/dispatch lives in a sibling `cmd/harness/*cli`
package (`issueopscli`, `mcpcli`, `daemoncli`, `policycli`, `installcli`,
`workercli`, `gatescli`, `channelcli`, `loopcli`, `hookcli`, `statecli`,
`statuscli`, `updatecli`, `projectcli`, `qualitycli`, `validationcli`,
`contractcli`, `webfetchcli`, `apidoc`, plus `basiccli`, `rootcmd`,
`selfworkflow`, `riskqa`, `commandstep`, `pathutil`, `contractgolden`).
`rootcmd.Command` owns only dispatch, `help`/`version`, and exit-code policy
(e.g. policy/guard denial → 3, gates usage error → 2); the runner map lives in
`harnessapp/root_command_facade.go`.

Two catalogs anchor the human/machine surface so it cannot drift:

- `internal/domain/cli` owns the canonical top-level command list
  (`Commands()`) and the usage text (`Usage(version)`); `harnessapp` renders
  it, and the `issueops` lines are composed solely from the
  `issueOpsUsageCatalog` so a command cannot exist in two hand-written places.
- `internal/domain/mcp` owns the advertised MCP tool catalog and dispatch
  groups; `AdvertisedTools()` and `DispatchMap()` both derive from the single
  ordered `catalogSections()` slice, so registering a tool in one catalog
  function makes it advertised *and* routable. `cmd/harness/mcpcli` owns the
  stdio/JSON-RPC transport and handler wiring over the official MCP Go SDK;
  `internal/adapter/mcp` is limited to the capture-only conformance probe.

`cmd/harness/daemoncli` owns the daemon (identity, admission, socket) and the
MCP stdio proxy (bounded reconnects, `-32002` `daemon_generation_changed` when
the daemon restarts mid-flight). `cmd/harness/workercli` plus
`internal/adapter/worker` provide lifecycle job records and the policy-gated
`run --read-only` executor; there is no long-resident job daemon — the daemon
backs only the MCP proxy.

Golden tests pin the whole surface: `cmd/harness/contractgolden` pins the CLI
usage text and MCP tool/resource catalogs under `cmd/harness/testdata`, and
the `harnessapp` response-contract suite runs the full CLI+MCP surface under a
temporary state dir and goldens the combined snapshot.

## `internal/contract` — shared DTO vocabulary

`internal/contract/<capability>` holds the request/result shapes, schema
versions, and error vocabulary that CLI, MCP, and state transports share.
Contracts must not import implementation layers: the ownership rules fail any
`contract -> domain/application/adapter/cmd/port` edge
(`contract_must_not_import_internal`), while contract-to-contract references
are composition, not coupling. Some contracts exist purely to pin durable
bytes — e.g. `internal/contract/issueopslease/stable_v1.go` re-declares the
persisted v1 record shape without touching any implementation type, so
canonicalization of durable state cannot drift, and vertical-specific rules
forbid the lease contract from importing production IssueOps code.

## `internal/domain` — pure rules and catalogs

`internal/domain/<capability>` owns deterministic rules, reducers, and
classifiers with no filesystem, process, or database I/O. The ratchet bans
`domain -> {application, adapter, cmd, contract, os, os/exec, net, net/http,
database/sql, syscall, sqlite}` outright. Clocks are injected by default;
`internal/domain/auditid`, whose audit IDs embed `time.Now()`, is the recorded
exception. Two catalog packages are special: `internal/domain/cli` and
`internal/domain/mcp` (above) own the command/tool vocabulary, and
`internal/domain/pioneerskill` pins the 12 canonical pioneer skill names.

## `internal/application` — capability use cases

`internal/application/<capability>` composes contract, domain, and narrow
port interfaces into a use case and returns domain-shaped results. It must not
import concrete adapters, `cmd/...`, or transport stdlib
(`application_must_not_import_implementation` includes `path/filepath`).
Applications declare the operations they actually need as small local
interfaces (`ports.go`) rather than consuming fat ports.

## `internal/port` — capability interfaces

`internal/port` declares external capability interfaces and typed error
contracts in contract vocabulary: `port -> contract` and `port -> port` are
allowed, everything else internal is banned
(`port_must_not_import_internal`). Representative surfaces: the root
`port.TransactionalRecordStore`/`RecordCASStore` span+CAS operations,
`IssueProvider*` request/result DTOs for remote issues and PR/MRs,
`port.HostInstaller`, the `port/state` reader/span-store interfaces, and the
IssueOps owner-model defaults in `port/orca.go`. Ports are kept deliberately
narrow — a fitness test restricts `internal/port/issueopsbasesync` to the
exported types `Request`, `Receipt`, and `Inspector` plus a single `context`
import, so a port cannot accumulate behavior.

## `internal/adapter` — concrete boundaries

Two direction classifiers plus capability packages:

- `internal/adapter/inbound/<capability>` maps capability requests to
  application calls and re-projects results/errors for transport. It must not
  import outbound adapters.
- `internal/adapter/outbound/<capability>` implements the external I/O:
  `state` (checkpoint store), `sqlstore` (the shared SQLite engine), `webfetch`,
  `upstream`, `nativeactivation`, and the per-vertical IssueOps stores.
  State/SQL/network implementation imports (`database/sql`, `os`, `os/exec`,
  `net/http`, sqlite) must stay inside outbound adapters for the capabilities
  that own them.
- `internal/adapter/outbound/sqlstore` is the **shared storage engine**: each
  store root owns `harness.db` (records as `bucket`/`id`/JSON rows) and
  `harness.lock.db` (cross-process `BEGIN IMMEDIATE` span locks that die with
  the holder process). It satisfies `port.TransactionalRecordStore` and the
  `port/state` interfaces. The shared-storage exception is the only
  non-composition-root concrete-adapter edge set: any `outbound/*` adapter and
  `internal/adapter/issueops` may import `sqlstore`; only
  `outbound/issueops*` may import the shared `outbound/issueopsrecord`
  codec/lock primitive; `cmd`, inbound, and domain reaching the engine
  directly still fail.

Concrete (non-vertical) adapters each own one external boundary:
`codex`, `claude`, `omo` (host installers implementing `port.HostInstaller`),
`install`/`installutil` (host-neutral install engine + shared install file
primitives), `issueops` (the record-backed IssueOps capability adapter, with
subpackage composition like `loopgate` and `gatesgate`), `orca` (bounded
argv/timeout/envelope projection of the optional Orca CLI — no IssueOps policy
duplication), `policy` (command-policy catalog/evaluate/audit and the
read-only executor), `worker`, `hostprobe` (isolated live host probes),
`toolconformance` (fixture I/O + behavioral replay), `failurecause`,
`operationalhealth` (read-only git/Orca inventory), `channel`, `looprun`,
`gates`, `gitworktree`, `provider` (github/gitlab resolution behind
`port.IssueProvider`, so no core package imports concrete providers), `mcp`
(capture-only conformance probe), `audit`, `preflight`, `docs`, `inspect`,
`projectdocs`, `projectbootstrap`, `lifecycle`, `trace`, `guard`, `hookprompt`,
`doctor`, `repopath`, `commitsuggest`, `lintdiagnose`, and `skillcontract`.

Host adapter scope is contractual: default installs write only user-level
surfaces (skill symlinks, MCP registration, `SessionStart` context hooks) and
nothing into target repositories — repo-local files require explicit
`--project-local`. The host-neutral engine `install.InstallNative` normalizes
inputs and skill lists and delegates to exactly the three first-party
installers passed by `harnessapp`; adding a host means one `port.HostInstaller`
implementation plus composition-root wiring, never duplicated policy.

### Cross-capability injection, not imports

Because cross-capability adapter imports are banned at zero tolerance,
consumers declare their needs as package-level function variables in a
dedicated `adapter_dependencies.go` / `*_dependencies.go` file — e.g.
`internal/adapter/issueops`' `GitCmd`/`GitCmdRaw`/`GitOut`, `gatesgate`'s
`DiscoverGateFiles`/`CheckGateLedger`, the host adapters' install-plan and
activation-evidence functions — and `harnessapp` wiring files
(`configurePolicyAndGitObservers`, `configureAdapterTail`,
`configureStateDatabases`, `configureStateStores`,
`configureIssueOpsCLIRuntime`, …) assign implementations. There are no default
implementations: an un-injected dependency must surface as a structured,
fail-closed error, and tests inject the same concrete implementations that
production wires.

## `internal/architecture` — the test-only fitness boundary

This tree contains only `_test.go` files. It runs `go list -json ./...`,
collects direct production import edges, and enforces: unconditional layer
rules, the zero legacy-adapter baseline, ownership-manifest directions, the
shared-storage exceptions, per-vertical boundary tests (five-layer existence,
no legacy imports, shared record store), the orphan-package guard
(`TestProductionPackagesHaveImporters`, motivated by a wiring deletion that
silently disabled a PR-target guard), the documentation-canonicality test
(canonical docs must name current layer paths and not the removed
`internal/core`, `internal/adapter/cli`, `internal/adapter/fs`), and coding
convention ratchets. Production code never imports it; full rule detail lives
in [Dependency Ratchet](dependency-ratchet.md).

## Support trees

- `configs/` — host configuration templates and the optional provisioning
  catalog. The Codex installer writes `configs/codex/mcp.config.toml` and
  `configs/codex/hooks.json` templates; `configs/claude/` and `configs/omo/`
  hold the analogous surfaces; `configs/upstream.json` declares the optional
  post-activation Claude plugin/skill provisioning that must never become a
  core dependency. The repo-root `.mcp.json` is dogfood-only project-local MCP
  configuration — default installs must not copy it into target repos.
- `skills/` — the single host-neutral source of truth for shared skills
  (34 `SKILL.md` directories: 12 pioneer-namesake + 22 operational). Default
  install symlinks the same sources into each host's user skill directory;
  host-specific copies are forbidden drift.
- `.agent-harness/` — the agent-facing knowledge base and git-tracked
  lifecycle layout: canonical architecture/conventions docs (pinned by the
  documentation test), ADRs, cautions, evidence, and per-issue artifact
  folders (`.agent-harness/issues/<issue-number>/` with `plan.md` and the
  canonical `gates.md` ledger that PR readiness reads). `project bootstrap`
  manages only its marker block inside a target repo's `AGENTS.md`.
- `scripts/` — operational entry points. `scripts/install-native.sh` builds
  `bin/agent-harness` and drives `agent-harness install` as a staged
  activation transaction (begin/commit/abort with SHA-256-verified binary
  candidates); the Python scripts guard skill contracts and shell-skill
  behavior. Install/update paths stay independently runnable and never
  provision external tools on core's behalf.
- `testdata/` (repo root) — cross-cutting fixtures: `issueops/fixtures`,
  `webfetch/live`, and `pioneer-holdouts` inputs. `cmd/harness/testdata`
  holds the contract goldens. `internal/holdoutdeleak` mechanically guarantees
  the pioneer-holdout fixtures contain inputs only, never recorded answers.
- `internal/testsupport` — shared test helpers (stdout/stderr capture).
  `internal/holdoutdeleak` is itself a test-support-only package; intended
  orphan packages are listed, with reasons, in `orphanPackageAllowlist`.

## The capability-vertical pattern

A **capability vertical** is the standard packaging for a migrated capability:

```
internal/contract/<cap>          persisted/request shape + error vocabulary
internal/domain/<cap>            pure decisions, deny codes, reducers
internal/application/<cap>       use case over narrow local interfaces
internal/adapter/inbound/<cap>   transport request → application call
internal/adapter/outbound/<cap>  persistence + side effects
cmd/harness/harnessapp           the only place these are wired together
```

The contract owns the shape, the domain owns the decision, the application
owns the orchestration behind `ports.go` interfaces, the inbound adapter maps
transport, and the outbound adapter touches the world. Fitness tests require
all five layers to exist for migrated capabilities, forbid them from importing
the legacy `internal/adapter/issueops` root, and require the outbound
verticals to share `outbound/issueopsrecord` instead of duplicating codecs.

### Worked example: `issueopslease`

`issueops execution claim` flows through every layer. The composition root
opens the store, builds the outbound pieces, composes the application service,
and hands the inbound handler to the IssueOps dispatcher:

```go
db, _ := sqlstore.Open(stateRoot)
preflight := leaseoutbound.NewClaimContextPreflight(db, readIssueSnapshot)
service := leaseapp.NewClaimService(
    leaseoutbound.NewSQLiteRepository(db),
    leaseoutbound.UTCClock{},
    leaseoutbound.InspectNativeProcess,
    preflight,
)
return leaseinbound.NewClaimHandler(service)(ctx, stateRoot, request, deps)
```

From there:

1. The inbound handler (`adapter/inbound/issueopslease`) translates the
   transport DTO into `leaseapp.ClaimRequest` and maps failures back to public
   errors via domain deny codes — never via string matching.
2. The application `ClaimService` runs preflight → actor resolution →
   repository claim, using only the interfaces in its `ports.go`
   (`ClaimRepository`, `ClaimContextPreflight`, `Clock`, `ProcessInspector`).
3. The domain (`domain/issueopslease`) owns the pure validation — lease
   status/generation/authority/canonical-CWD/token checks returning typed deny
   codes — and constructs the claim outcome from an injected clock.
4. The outbound adapter (`adapter/outbound/issueopslease`) executes the record
   CAS transition inside a `WithSpan` over `port.TransactionalRecordStore`,
   and its claim preflight validates the sealed issue/context digests against
   the stored record and the live remote issue snapshot.

```mermaid
sequenceDiagram
    participant CLI as issueopscli
    participant Root as harnessapp wiring
    participant In as inbound/issueopslease
    participant App as application/issueopslease
    participant Dom as domain/issueopslease
    participant Out as outbound/issueopslease
    participant Store as outbound/sqlstore

    CLI->>Root: issueops execution claim argv
    Root->>In: NewClaimHandler with ClaimService
    In->>App: ClaimService.Claim ClaimRequest
    App->>Out: ClaimContextPreflight.Preflight
    Out->>Store: read issueops_v1 record row
    Out-->>App: RecordValidator
    App->>Dom: resolveActor process receipts
    App->>Out: ClaimRepository.Claim under WithSpan
    Out->>Dom: ValidateClaim status generation token
    Dom-->>Out: deny code or ClaimOutcome
    Out->>Store: BEGIN IMMEDIATE span CAS update
    Out-->>App: RepositoryResult with Execution
    App-->>In: ClaimResult
    In-->>CLI: ExecutionResult or public error
```

*The issueopslease claim request through one vertical: only harnessapp knows
the store and concrete adapters; each arrow crosses exactly one boundary.*

### Worked example: `issueopspublication`

`issueopspublication` (remote PR/MR creation and `remote_pr_create` recovery)
follows the same shape with a different flavor of domain decision:

- `internal/application/issueopspublication` declares narrow `Repository`,
  `Provider`, and `Verifier` interfaces — intent lifecycle
  (Preview/Begin/Load/MarkRetry/RecordFailure/Complete) plus provider create
  and inspect operations.
- `internal/domain/issueopspublication` owns the pure reconcile decision:
  one candidate → adopt; multiple candidates, non-authoritative zero, unknown
  invocation, or exhausted retries → preserve; only a proven `not_invoked`
  outcome is retryable. Ambiguity is fail-closed by construction.
- `harnessapp` composes outbound `NewRepository`/`NewProviderGateway`/
  `NewVerifier` into `CreateService`/`ReconcileService` and wraps them in the
  inbound `NewCreateHandler`/`NewReconcileHandler`; CLI create and CLI/MCP
  reconcile share the same request-scoped handler pair, and a missing handler
  fails closed rather than falling back to a legacy full flow.

## Where a change lands

- New pure rule or classifier → `internal/domain/<cap>`.
- New DTO or persisted shape → `internal/contract/<cap>`.
- New external capability → `internal/port` interface, then the vertical.
- New transport command → a `cmd/harness/<cli>` package plus a
  `harnessapp` facade; the canonical description goes in
  `internal/domain/cli` and the usage golden forces agreement.
- New MCP tool → one `catalogSections()` entry in `internal/domain/mcp`.
- New host → one `port.HostInstaller` implementation plus wiring; the install
  contract matrix golden detects surface drift.
- New IssueOps capability → the five vertical layers, a
  `<cap>_vertical_test.go` in `internal/architecture`, and shared record
  access via `outbound/issueopsrecord`.
- Cross-capability need inside an adapter → a function-variable slot plus
  `harnessapp` wiring — never an import.
