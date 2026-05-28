---
name: project-bootstrap
description: Generate or update a repo-local AGENTS.md routing block and .agent-harness project operating documents from repository evidence. Use when the user asks to bootstrap project docs, create project-specific agent guidance, analyze a repo into ARCHITECTURE/CAUTIONS/COMMIT_POLICY/CONSTITUTION/CONVENTIONS/TECH_STACK/TESTING/OPERATIONS/AGENT_WORKFLOW/OPEN_API_SPEC docs, or install project docs for agent-harness.
---

# Project Bootstrap

## Goal

Create evidence-backed project operating documents for agents without making agent-harness memory or context-injection features. The output is a repo-local `AGENTS.md` routing block plus `.agent-harness/*.md` documents.

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
- `.agent-harness/OPERATIONS.md`
- `.agent-harness/AGENT_WORKFLOW.md`

## Safety rules

- Never overwrite an existing `AGENTS.md` wholesale. The harness may prepend the behavioral guideline block when missing and manages only the `AGENT_HARNESS` marker block after that.
- Start with a dry-run plan and inspect planned create/update actions.
- Write only when the user explicitly requested bootstrapping/updating project docs or when continuing an approved implementation task.
- Treat generated docs as evidence-backed drafts. If the project already has stronger local docs, preserve and reference them.
- After first setup, keep `.agent-harness` documents fresh through MCP: route the task, read the current doc SHA, update one document at a time, or append CAUTIONS/ADR records for concrete false cases and decisions.


## Static vs agent-filled output

`harness project bootstrap` is deterministic and conservative. It creates a safe baseline from repository signals, but it does not deeply understand business logic, architecture, deployment, API style, or historical decisions.

- Static bootstrap fills: document skeletons, detected languages/package managers, candidate commands, routing, generic safety/testing/API guidance, and `AGENTS.md` managed blocks.
- Agent enrichment fills: codebase-specific architecture, operations, conventions, testing examples, OpenAPI style, known risks, and decision rationale from direct evidence.

Use `PROMPT.md` for the enrichment pass. The prompt requires agents to read repo evidence, avoid invented facts, and maintain `.agent-harness` docs through MCP (`project_docs_route` → `project_docs_read` → `project_docs_update` or `project_docs_record`).

## Workflow

1. Run a dry-run plan:

   ```bash
   harness project bootstrap --repo . --json
   ```

2. Inspect the JSON:
   - `signals.files`
   - `signals.languages`
   - `signals.test_commands`
   - planned `files[].action`

3. If the plan is acceptable, write files:

   ```bash
   harness project bootstrap --repo . --write --json
   ```

4. Run the agent enrichment pass using `skills/project-bootstrap/PROMPT.md`. The enrichment pass should read repo evidence and update `.agent-harness` docs through MCP rather than blindly accepting static template text.

5. Ask the MCP route when starting later work. Prefer this over injecting every project doc into context:

   ```bash
   harness project route-docs --repo . --task "commit" --json
   harness project route-docs --repo . --task "test" --json
   harness project route-docs --repo . --task "architecture" --json
   ```

6. When a solved problem or decision should become durable project knowledge, record it through the append-only MCP/CLI contract:

   ```bash
   harness project record --repo . --kind caution --title "<problem>" --summary "<what happened>" --resolution "<fix>" --json
   harness project record --repo . --kind adr --title "<decision>" --summary "<why>" --decision "<choice>" --json
   ```

7. Verify:
   - Confirm `AGENTS.md` contains the behavioral top block and only the managed marker block addition/update.
   - Confirm `.agent-harness/*.md` exists and contains evidence/confidence language where commands were inferred.
   - Confirm future updates can be made through MCP: `project_docs_route`, `project_docs_read`, `project_docs_update`, and `project_docs_record`.
   - Run the target repo's smallest relevant validation command when docs affect workflow claims.
