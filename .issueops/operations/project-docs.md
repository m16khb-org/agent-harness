---
name: project-docs.md
description: Project bootstrap, project-doc routing, MCP document updates, and standalone docs policy.
---

# Project Docs Operations

## Project Bootstrap

`issueops project bootstrap` analyzes a target repo and creates agent-facing operating documents plus user-state repo profile metadata. By default, it writes missing files. Use `--dry-run` for a plan only and `--sync` to refresh existing docs from current templates and repo evidence.

```bash
issueops project bootstrap --repo /path/to/repo --json
issueops project bootstrap --repo /path/to/repo --sync --json
issueops project route-docs --repo /path/to/repo --task "commit" --json
```

Bootstrap behavior:

- `AGENTS.md` is not overwritten wholesale. The behavioral top block may be prepended, and only the `ISSUEOPS` marker block is added or updated.
- `.issueops/*.md` frontmatter `description` uses canonical concise English metadata.
- Runtime state is stored in user-state `projects/<repo-id>/project.json`, not target repo source.
- Profile metadata records VCS provider/hosting, language, package manager, app classification, framework evidence, and lifecycle namespace data for hook context.
- Static bootstrap is a baseline; improve docs from repo evidence when needed.

Generated project-doc set:

- `AGENTS.md` routing block
- `.issueops/ARCHITECTURE.md`
- `.issueops/CAUTIONS.md`
- `.issueops/COMMIT_POLICY.md`
- `.issueops/CONSTITUTION.md`
- `.issueops/CONVENTIONS.md`
- `.issueops/TECH_STACK.md`
- `.issueops/TESTING.md`
- `.issueops/OPEN_API_SPEC.md`
- `.issueops/ADR.md`
- `.issueops/OPERATIONS.md`
- `.issueops/AGENT_WORKFLOW.md`
- `.issueops/DESIGN.md` (client repositories only: `frontend` or `desktop-client` project type detected; points at a curated root `DESIGN.md` when one exists)

## MCP Project Docs

Tools/resources:

- `project_docs_bootstrap_plan`: write-free bootstrap plan.
- `project_docs_route`: recommend `AGENTS.md`/`.issueops` docs for a task.
- `project_docs_read`: read a document and return its hash.
- `project_docs_revise`: full-document replacement; requires `expected_sha256` and `confirm=true`.
- `project_docs_append`: append-only ADR/caution record.
- `issueops://project-docs`: current workspace project-doc routing.
- `issueops://project-doc-upkeep`: pending lifecycle doc-upkeep state.

Effective MCP sequence:

1. Use `project_docs_route` to choose relevant docs.
2. If a doc needs an update, call `project_docs_read` first and keep `expected_sha256`.
3. Use `project_docs_revise` for intentional full-document replacements.
4. Use `project_docs_append(kind=caution)` for solved recurring mistakes.
5. Use `project_docs_append(kind=adr)` for decisions with rationale and rejected alternatives.
6. Use `command_policy_check` before commands whose cwd/workspace/write/network boundaries matter.

Dry-run/default-write rules:

- `project_docs_bootstrap_plan` is dry-run only.
- `project_docs_revise` is dry-run without `confirm=true`.
- `project_docs_append` is append-only and narrow.
- `state_prune` and `self_verify_promote` are dry-run unless confirmed.

## Standalone Docs Policy

Project docs, docs routing, and draft-wiki staging must not depend on an external wiki, memory provider, graph index, or companion MCP server. Do not make external-tool-specific harness CLI commands, MCP tools, resources, or SessionStart hooks prerequisites for project documentation workflows.

Repo-local draft-wiki staging is separate from source-of-truth `.issueops/*.md`. Candidates live under `.issueops/draft-wiki/`, and `issueops docs`/MCP `docs_index` must not index them as canonical project docs. Hooks never decide or queue this material. The public CLI does not currently have a working `project draft-wiki queue` surface. Use `project_docs_append` or SHA-guarded `project_docs_revise` for canonical updates; do not prescribe a nonexistent queue command.
