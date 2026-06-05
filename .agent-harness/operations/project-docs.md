---
name: project-docs.md
description: Project bootstrap, project-doc routing, MCP document updates, and LLM Wiki policy.
---

# Project Docs Operations

## Project Bootstrap

`agent-harness project bootstrap` analyzes a target repo and creates agent-facing operating documents plus user-state repo profile metadata. It writes missing files by default; use `--dry-run` for a plan only. Use `--sync` to refresh existing docs from current templates and repo evidence.

```bash
agent-harness project bootstrap --repo /path/to/repo --json
agent-harness project bootstrap --repo /path/to/repo --sync --json
agent-harness project route-docs --repo /path/to/repo --task "commit" --json
```

Bootstrap behavior:

- `AGENTS.md` is not overwritten wholesale. The behavioral top block may be prepended, and only the `AGENT_HARNESS` marker block is added or updated.
- `.agent-harness/*.md` frontmatter `description` uses canonical concise English metadata.
- Runtime state is stored in user-state `projects/<repo-id>/project.json`, not target repo source.
- Profile metadata records VCS provider/hosting, language, package manager, app classification, framework evidence, and lifecycle namespace data for hook context.
- Static bootstrap is a baseline; agents should improve docs from repo evidence when needed.

Generated project-doc set:

- `AGENTS.md` routing block
- `.agent-harness/ARCHITECTURE.md`
- `.agent-harness/CAUTIONS.md`
- `.agent-harness/COMMIT_POLICY.md`
- `.agent-harness/CONSTITUTION.md`
- `.agent-harness/CONVENTIONS.md`
- `.agent-harness/TECH_STACK.md`
- `.agent-harness/TESTING.md`
- `.agent-harness/OPEN_API_SPEC.md`
- `.agent-harness/ADR.md`
- `.agent-harness/OPERATIONS.md`
- `.agent-harness/AGENT_WORKFLOW.md`

## MCP Project Docs

Tools/resources:

- `project_docs_bootstrap_plan`: write-free bootstrap plan.
- `project_docs_route`: recommend `AGENTS.md`/`.agent-harness` docs for a task.
- `project_docs_read`: read a document and return its hash.
- `project_docs_update`: full-document replacement; requires `expected_sha256` and `confirm=true`.
- `project_docs_record`: append-only ADR/caution record.
- `harness://project-docs`: current workspace project-doc routing.
- `harness://project-doc-upkeep`: pending lifecycle doc-upkeep state.

Effective MCP sequence:

1. Use `project_docs_route` to choose relevant docs.
2. If a doc needs an update, call `project_docs_read` first and keep `expected_sha256`.
3. Use `project_docs_update` for intentional full-document replacements.
4. Use `project_docs_record(kind=caution)` for solved recurring mistakes.
5. Use `project_docs_record(kind=adr)` for decisions with rationale and rejected alternatives.
6. Use `command_policy_check` before commands whose cwd/workspace/write/network boundaries matter.

Dry-run/default-write rules:

- `project_docs_bootstrap_plan` is dry-run only.
- `project_docs_update` is dry-run without `confirm=true`.
- `project_docs_record` is append-only and narrow.
- `state_prune`, `state_migrate`, and `self_verify_promote` are dry-run unless confirmed.

## LLM Wiki Policy

LLM Wiki functionality is not implemented by `agent-harness`. Use upstream `nvk/llm-wiki` Codex/Claude plugin or portable `AGENTS.md` workflows. Do not add llm-wiki-specific harness CLI commands, MCP tools, resources, or SessionStart hooks.

Repo-local draft wiki staging remains separate from source-of-truth `.agent-harness/*.md`: draft candidates live under `.agent-harness/draft-wiki/`, and `agent-harness docs`/MCP `docs_index` must not index draft candidates as canonical project docs. Hooks do not decide whether material is worth remembering and do not auto-queue draft-wiki work; the main agent must judge reuse value and explicitly queue material with `agent-harness project draft-wiki queue --stdin` or `--input`.
