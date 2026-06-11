---
name: project-bootstrap
description: Generate or update a repo-local AGENTS.md routing block and .agent-harness project operating documents from repository evidence. Use when the user asks to bootstrap project docs, create project-specific agent guidance, analyze a repo into ARCHITECTURE/CAUTIONS/COMMIT_POLICY/CONSTITUTION/CONVENTIONS/TECH_STACK/TESTING/OPERATIONS/AGENT_WORKFLOW/OPEN_API_SPEC/ADR docs, or install project docs for agent-harness.
---

# Project Bootstrap

## Goal

Create evidence-backed project operating documents and repo profile metadata for agents. The output is a repo-local `AGENTS.md` routing block plus `.agent-harness/*.md` documents, with VCS/language/project-type metadata persisted in agent-harness user state for later hook context injection.

## Generated documents

- `AGENTS.md` behavioral top block + managed `AGENT_HARNESS` marker block: routes agents to project docs.
- `.agent-harness/ARCHITECTURE.md`
- `.agent-harness/CAUTIONS.md` — false cases, recurring failures, and risk notes.
- `.agent-harness/COMMIT_POLICY.md`
- `.agent-harness/CONSTITUTION.md` — SessionStart baseline and source-of-truth policy.
- `.agent-harness/CONVENTIONS.md`
- `.agent-harness/TECH_STACK.md`
- `.agent-harness/TESTING.md` — good/bad test criteria and likely verification commands for test work.
- `.agent-harness/OPEN_API_SPEC.md` — static plus agent documentation gate prompt for endpoint/DTO/OpenAPI changes.
- `.agent-harness/ADR.md` — architecture decision rationale and rejected alternatives.
- `.agent-harness/draft-wiki/` — README plus `draft/`, `approved/`, `rejected/` staging tree for the draft-wiki-promoter workflow (created by bootstrap; operated via `project draft-wiki` subcommands).
- `.agent-harness/OPERATIONS.md`
- `.agent-harness/AGENT_WORKFLOW.md`

## Safety rules

- Never overwrite an existing `AGENTS.md` wholesale. The harness may prepend the behavioral guideline block when missing and manages only the `AGENT_HARNESS` marker block after that.
- Use `--dry-run` when the user only wants a plan.
- Write when the user explicitly requested bootstrapping/updating project docs or when continuing an approved implementation task; use `--sync` before replacing existing generated docs from current templates/evidence.
- Treat generated docs as evidence-backed drafts. If the project already has stronger local docs, preserve and reference them.
- After first setup, keep `.agent-harness` documents fresh through MCP: route the task, read the current doc SHA, update one document at a time, or append CAUTIONS/ADR records for concrete false cases and decisions. The MCP catalog also exposes `project_docs_bootstrap_plan` for the dry-run plan shape.


## Static vs agent-filled output

`agent-harness project bootstrap` is deterministic and conservative. It creates a safe baseline from repository signals, but it does not deeply understand business logic, architecture, deployment, API style, or historical decisions.

- Static bootstrap fills: document skeletons, detected languages/package managers, candidate commands, routing, generic safety/testing/API guidance, `AGENTS.md` managed blocks, and repo profile metadata.
- Agent enrichment fills: codebase-specific architecture, operations, conventions, testing examples, OpenAPI style, known risks, and decision rationale from direct evidence.

Use `PROMPT.md` for the enrichment pass. The prompt requires agents to read repo evidence, avoid invented facts, and maintain `.agent-harness` docs through MCP (`project_docs_route` → `project_docs_read` → `project_docs_update` or `project_docs_record`).

## Workflow

1. Run a dry-run plan when you need to inspect before writing:

   ```bash
   agent-harness project bootstrap --repo . --dry-run --json
   ```

2. Inspect the JSON:
   - `signals.files`
   - `signals.languages`
   - `signals.test_commands`
   - `signals.profile`
   - planned `files[].action`

3. If the plan is acceptable, write files and user-state profile metadata:

   ```bash
   agent-harness project bootstrap --repo . --json
   ```

4. Run the agent enrichment pass using `skills/project-bootstrap/PROMPT.md`. The enrichment pass should read repo evidence and update `.agent-harness` docs through MCP rather than blindly accepting static template text.

5. Ask the MCP route when starting later work. Prefer this over injecting every project doc into context:

   ```bash
   agent-harness project route-docs --repo . --task "commit" --json
   agent-harness project route-docs --repo . --task "test" --json
   agent-harness project route-docs --repo . --task "architecture" --json
   ```

6. When a solved problem or decision should become durable project knowledge, record it through the append-only MCP/CLI contract:

   ```bash
   agent-harness project record --repo . --kind caution --title "<problem>" --summary "<what happened>" --resolution "<fix>" --json
   agent-harness project record --repo . --kind adr --title "<decision>" --summary "<why>" --decision "<choice>" --json
   ```

7. Verify:
   - Confirm `AGENTS.md` contains the behavioral top block and only the managed marker block addition/update.
   - Confirm `.agent-harness/*.md` exists and contains evidence/confidence language where commands were inferred.
   - Confirm future updates can be made through MCP: `project_docs_route`, `project_docs_read`, `project_docs_update`, and `project_docs_record`.
   - Run the target repo's smallest relevant validation command when docs affect workflow claims.
