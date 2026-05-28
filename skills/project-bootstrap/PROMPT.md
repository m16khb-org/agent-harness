# Project Bootstrap Agent Enrichment Prompt

Use this prompt after the static `harness project bootstrap --write` pass, or when refreshing existing `.agent-harness` docs.

## Objective

Turn the static bootstrap drafts into evidence-backed, repo-specific operating docs that future agents can maintain through MCP. Keep user consensus current without inventing facts.

## Static vs agent-filled boundary

Static harness bootstrap fills only:

- `AGENTS.md` behavioral top block and managed `AGENT_HARNESS` routing block.
- `.agent-harness/*.md` baseline documents from deterministic templates.
- Detected signals such as languages, package managers, candidate test/build/lint commands, GitHub workflows, and existing agent docs.
- Generic safety, testing, OpenAPI, commit, and workflow rules.

The agent enrichment pass must fill from codebase evidence:

- Actual architecture: entrypoints, module boundaries, data flow, persistence/network surfaces, auth/error boundaries.
- Actual operations: local setup, env vars, build/test/lint/run/deploy commands, CI behavior, generated artifacts.
- Actual conventions: naming, layering, dependency rules, response shapes, OpenAPI/Swagger style, SOLID/YAGNI/KISS/design-pattern examples.
- Actual testing guidance: good tests and bad tests for this repo, fixture style, mock boundaries, known flaky areas.
- Actual cautions/ADR: only concrete solved false cases, risks, decisions, rejected alternatives, and consequences.

## Required workflow

1. Read `AGENTS.md`, existing `.agent-harness/*.md`, README, package/build config, CI files, and the smallest relevant source files.
2. Run or inspect `project_docs_route` for the task type when MCP is available.
3. For each document you plan to update, call `project_docs_read` first and keep its `sha256`.
4. Edit one `.agent-harness` document at a time through `project_docs_update` with:
   - `expected_sha256` from `project_docs_read`
   - `summary` explaining the consensus-preserving change
   - `evidence` listing user instruction, files, or commands
   - `confirm=true` only after the replacement content preserves stronger existing guidance.
5. Use `project_docs_record` instead of full-document updates when recording:
   - a solved problem, false case, or recurring risk → `kind=caution`
   - an architecture/process/API decision with rationale or rejected alternatives → `kind=adr`
6. Verify the changed docs by rerunning the smallest relevant checks and reading tool output.

## Do not invent

- If a fact is not confirmed by source files, commands, or explicit user instruction, mark it as `Unknown / not confirmed` and add how to verify.
- Do not copy generic framework advice into project docs unless this repo actually uses it.
- Do not overwrite stronger local project docs with template text.
- Do not update CAUTIONS or ADR for hypothetical issues; use them only for concrete false cases or decisions.
- Do not edit `AGENTS.md` outside the behavioral top block and managed marker block unless explicitly requested.

## Document-specific fill targets

- `CONSTITUTION.md`: project-specific priority, safety, source-of-truth, session-start baseline.
- `ARCHITECTURE.md`: current structure, boundaries, adopted architectural decisions, diagrams only if helpful.
- `CONVENTIONS.md`: concrete coding conventions, SOLID/YAGNI/KISS/design-pattern good/bad cases observed in this repo.
- `TESTING.md`: commands and examples of well-structured vs poorly-structured tests for this repo.
- `OPEN_API_SPEC.md`: only if API surfaces exist; otherwise keep framework-agnostic gate and mark API style as not confirmed.
- `OPERATIONS.md`: install/run/build/deploy/env/local-dev details and smoke checks.
- `AGENT_WORKFLOW.md`: when to route docs, when to call MCP, when to update docs, and completion evidence.
- `CAUTIONS.md`: durable false cases after they happen and are fixed.
- `ADR.md`: durable decisions after consensus or implementation makes the decision real.
- `COMMIT_POLICY.md`: commit style and verification evidence requirements.
- `TECH_STACK.md`: confirmed language/runtime/package manager/toolchain versions and confidence.

## Completion criteria

- Every updated statement is backed by a file, command output, or user instruction.
- MCP metadata remains sufficient for future agents: route → read → update/record → verify.
- Legacy or unconfirmed areas are not silently treated as facts.
- Final report lists updated docs, evidence, verification commands, and remaining unknowns.
