---
name: project-docs-bootstrap
description: "Initial creation or template refresh of repo-local .agent-harness operating documents and the AGENTS.md routing block from repository evidence, through the deterministic agent-harness project bootstrap pass followed by an agent enrichment pass. Use when asked to bootstrap, initialize, or regenerate project docs for a repository, or to set up AGENTS.md agent guidance. For incremental refresh during ongoing work use project-docs-update; for restructuring oversized docs use project-docs-optimize."
---

# Project Docs Bootstrap

## Goal

Create or refresh the repo-local `.agent-harness` document family and the
`AGENTS.md` routing block from repository evidence. Two passes run in order:
the deterministic static pass (CLI engine), then the agent enrichment pass
(`PROMPT.md` in this skill). This skill is the docs dimension of the
`project-bootstrap` orchestration and is also usable standalone.

## Use and Boundaries

Use for first-time creation of `.agent-harness` docs, or an explicit template
refresh of existing generated docs. Route instead:

- incremental refresh while work is happening → `project-docs-update`;
- oversized or badly structured docs → `project-docs-optimize`.

If `.agent-harness` documents already exist and the user did not ask for a
refresh, do not run the write pass; route to `project-docs-update`.

## Inputs

- repository root (defaults to the current directory);
- `--dry-run` for a plan without writing;
- `--sync` to replace existing generated docs from current templates and
  evidence — only on explicit request.

## Safety Rules

1. Never overwrite an existing `AGENTS.md` wholesale. The engine may prepend
   the behavioral block when missing and manages only the `AGENT_HARNESS`
   marker block after that.
2. Run the dry-run plan first when any doubt exists about what will change.
3. Without `--sync`, existing generated docs are preserved; report the
   `sync_available` warning instead of forcing a refresh.
4. Treat static output as an evidence-backed draft. If the project already
   has stronger local docs, preserve and reference them.
5. The enrichment pass must not invent facts: anything not confirmed by
   source files, command output, or explicit user instruction is marked
   `Unknown / not confirmed` with a way to verify.

## Static vs Agent-Filled Boundary

The static pass fills only: document skeletons, detected
languages/package managers, candidate test/build/lint commands, routing,
generic safety/testing/API guidance, `AGENTS.md` managed blocks, and repo
profile metadata.

The agent enrichment pass fills from codebase evidence: actual architecture
(entrypoints, boundaries, data flow, auth/error surfaces), actual operations
(setup, env, build/test/lint/deploy, CI), actual conventions and testing
guidance, and concrete cautions/ADRs. Standard engineering topics (DDD, clean
code, layered/hexagonal architecture, SOLID, OOP, testing, Swagger, exception
handling, ...) are evaluated against that evidence using
`references/engineering-standards.md`; only confirmed topics reach the docs.
See `PROMPT.md` for the full fill targets per document.

## Workflow

1. Plan without writing:

   ```bash
   agent-harness project bootstrap --repo . --dry-run --json
   ```

   MCP alternative: `project_docs_bootstrap_plan`.

2. Inspect the plan JSON: `signals.files`, `signals.languages`,
   `signals.test_commands`, `signals.profile`, and each planned
   `files[].action`.

3. Write files and repo profile metadata when the plan is acceptable:

   ```bash
   agent-harness project bootstrap --repo . --json
   ```

   Add `--sync` only for an explicitly requested refresh of existing
   generated docs.

4. Run the agent enrichment pass using `PROMPT.md` in this skill directory.
   It reads repository evidence and updates `.agent-harness` docs through the
   MCP contract (`project_docs_route` → `project_docs_read` →
   `project_docs_revise` / `project_docs_append`) rather than blindly
   accepting static template text. It also evaluates the engineering
   standards catalog at `references/engineering-standards.md` in this skill
   directory (layered/hexagonal/onion/clean architecture, DDD, SOLID, OOP,
   clean code, error/exception handling, OpenAPI/Swagger, testing best
   practices, and adjacent topics) and maps every evidence-confirmed topic
   to its single owning document per the catalog's topic-to-doc map.

5. Verify:

   - `AGENTS.md` contains the behavioral top block and only the managed
     marker block was added or updated;
   - every expected root index, module starter, single-owner root, and
     `documentation/manifest.json` exists;
   - each family root links into its module directory and each module
     starter back-links its root;
   - the MCP maintenance path works: `project_docs_route`,
     `project_docs_read`, `project_docs_revise`, `project_docs_append`;
   - the project-docs-optimize checker reports zero violations on the fresh
     layout when its script is available:

     ```bash
     uv run --directory <agent-harness>/skills/project-docs-optimize \
       python -m scripts.check --root "$PWD" --mode check --json
     ```

   - the smallest relevant repo validation command still passes when docs
     make claims about workflow or commands.

## Generated Documents

`AGENTS.md` (behavioral block + managed marker block) and the
`.agent-harness` family: ARCHITECTURE, CAUTIONS, COMMIT_POLICY,
CONSTITUTION, CONVENTIONS, TECH_STACK, TESTING, OPEN_API_SPEC, ADR,
OPERATIONS, AGENT_WORKFLOW.

## Completion Evidence

Report: plan summary, files created vs preserved vs synced, enrichment
updates with their evidence, verification command results, and remaining
unknowns explicitly marked.
