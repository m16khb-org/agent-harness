# Operating Documentation Responsibility Audit

Measured on 2026-08-11 from the current worktree.

## Scope and threshold

This audit covers `AGENTS.md` and the required operating documents under
`.agent-harness/`. A document is a modularization candidate when it exceeds 250
lines and owns more than one independently navigable responsibility.

The required root filenames remain stable because
`internal/domain/projectdoc/constants.go` exposes them as the project-doc
contract. Those files should become concise canonical indexes, not compatibility
copies. Detailed content belongs in responsibility folders.

## Inventory

| Document | Lines | Words | H2 sections | Assessment |
|---|---:|---:|---:|---|
| `.agent-harness/ADR.md` | 956 | 11,280 | 43 | Split required |
| `.agent-harness/CAUTIONS.md` | 942 | 15,267 | 78 | Split required |
| `.agent-harness/TESTING.md` | 489 | 3,931 | 15 | Split required |
| `.agent-harness/ARCHITECTURE.md` | 415 | 4,130 | 15 | Split required |
| `.agent-harness/OPERATIONS.md` | 408 | 3,623 | 13 | Split required |
| `.agent-harness/CONVENTIONS.md` | 324 | 3,698 | 22 | Split required |
| `AGENTS.md` | 221 | 1,701 | 12 | Keep as routing document |
| `.agent-harness/TECH_STACK.md` | 206 | 1,245 | 9 | Keep focused |
| `.agent-harness/CONSTITUTION.md` | 190 | 1,286 | 11 | Keep authoritative |
| `.agent-harness/AGENT_WORKFLOW.md` | 131 | 1,472 | 10 | Keep focused |
| `.agent-harness/COMMIT_POLICY.md` | 92 | 389 | 4 | Keep focused |
| `.agent-harness/OPEN_API_SPEC.md` | 64 | 374 | 6 | Keep focused |

## Responsibility findings

### ADR

`ADR.md` combines the original architecture proposal, roadmap, risk register,
decision index, and 34 dated decisions. Dated decisions are independent records
and should not share one append-only file with the current architecture summary.

Target ownership:

- `ADR.md`: decision-system index and current accepted baseline
- `adr/README.md`: status, naming, and authoring rules
- `adr/roadmap.md`: retained implementation roadmap
- `adr/decisions/*.md`: one immutable decision record per subject

### Cautions

`CAUTIONS.md` combines baseline risk categories, subsystem-specific operational
rules, and 15 dated incident lessons. The file is both a policy manual and an
incident ledger, which makes targeted retrieval noisy.

Target ownership:

- `CAUTIONS.md`: risk navigation and mandatory universal cautions
- `cautions/runtime.md`: process, daemon, worker, lock, and state risks
- `cautions/security.md`: secrets, command policy, and boundary risks
- `cautions/issueops.md`: IssueOps lifecycle and orchestration risks
- `cautions/integrations.md`: host, MCP, hook, GitHub/GitLab, and external-tool risks
- `cautions/lessons/*.md`: dated incident lessons

### Testing

`TESTING.md` mixes test philosophy, command gates, fixture rules, concurrency,
goldens, API documentation review, host parity, and self-verification behavior.
These are distinct workflows with different consumers.

Target ownership:

- `TESTING.md`: test strategy index and minimum completion gate
- `testing/unit-and-contract.md`: unit, integration, fixture, and golden guidance
- `testing/concurrency-and-race.md`: race, process, lock, and nondeterminism rules
- `testing/cli-mcp-and-hosts.md`: CLI, MCP, Codex, and Claude parity checks
- `testing/self-verification.md`: single-pass self-verification contract
- `testing/api-documentation.md`: OpenAPI static and agent review gates
- `testing/issueops-execution.md`: IssueOps and Orca execution verification

### Architecture

`ARCHITECTURE.md` combines target topology, current implementation inventory,
component boundaries, native host integration, state isolation, and future worker
constraints. Current architecture and integration detail should remain linked
but independently readable.

Target ownership:

- `ARCHITECTURE.md`: architecture index, dependency direction, and invariants
- `architecture/hexagonal-core.md`: domain, application, ports, and adapters
- `architecture/runtime.md`: daemon, MCP proxy, state, and process boundaries
- `architecture/host-integration.md`: Codex and Claude thin adapters
- `architecture/issueops.md`: IssueOps vertical ownership

### Operations

`OPERATIONS.md` combines installation, upgrades, CLI usage, MCP registration,
daemon lifecycle, skills, project-local behavior, and troubleshooting.

Target ownership:

- `OPERATIONS.md`: operator index and critical recovery entrypoints
- `operations/guides/install-and-update.md`: install, bootstrap, upgrade, rollback
- `operations/guides/cli-and-state.md`: CLI discovery and state lifecycle
- `operations/guides/skills-and-hosts.md`: native skills and host-specific activation
- `operations/guides/troubleshooting.md`: diagnosis and recovery procedures
- `operations/guides/issueops-providers.md`: IssueOps preparation and provider contracts
- `operations/guides/issueops-execution.md`: IssueOps execution and recovery

### Conventions

`CONVENTIONS.md` combines Go package rules, contracts, adapters, CLI/MCP output,
skills, commits, API documentation, hooks, guards, policy tiers, state machines,
and external adapters. Commit policy and OpenAPI already have dedicated root
documents, so this file should route rather than repeat those rules.

Target ownership:

- `CONVENTIONS.md`: universal coding rules and responsibility index
- `conventions/go-and-packages.md`: Go, package, contract, and adapter rules
- `conventions/cli-mcp-and-output.md`: CLI/MCP schemas and response conventions
- `conventions/state-policy-and-hooks.md`: state, guards, policy, hooks, lifecycle
- dedicated `COMMIT_POLICY.md` and `OPEN_API_SPEC.md`: sole normative owners

## Cross-document duplication

- API documentation gates appear in `AGENTS.md`, `AGENT_WORKFLOW.md`,
  `CONVENTIONS.md`, `TESTING.md`, and `OPEN_API_SPEC.md`.
- Dependency fitness rules appear in both `ARCHITECTURE.md` and
  `CONVENTIONS.md`.
- Language-selection rationale appears in both `ADR.md` and `TECH_STACK.md`.

Each duplicated topic should have one normative owner. Other documents should
carry a one-line routing link and only the context specific to their workflow.

## Modularization outcome

Applied with `skills/project-docs-optimize` on 2026-08-11:

| Family | Root before | Root after | Detailed modules | Largest module |
|---|---:|---:|---:|---:|
| ADR | 956 | 79 | 46 | 238 |
| Cautions | 942 | 86 | 23 | 160 |
| Testing | 489 | 71 | 6 | 149 |
| Architecture | 415 | 105 | 4 | 146 |
| Operations | 408 | 86 | 6 | 174 |
| Conventions | 324 | 102 | 3 | 155 |

The strict checker reported 307 inspected documents, six validated families,
and zero violations. The runtime docs index reported all six required roots and
88 documents under the six module families. Untracked canonical root and
manifest-declared module documents are intentionally discoverable during
authoring; unrelated untracked research remains excluded.

## Acceptance criteria

The modularization is complete when:

1. every candidate root document is at most 250 lines;
2. every moved section has one canonical owner;
3. required root filenames remain valid project-doc entrypoints;
4. all relative links and documented commands validate;
5. `agent-harness docs --json` still reports every required root document;
6. a reusable documentation-optimization skill enforces these checks.
