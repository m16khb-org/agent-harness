---
name: project-docs-update
description: "Evidence-bound incremental refresh of existing .agent-harness project documents during ongoing work through the project_docs MCP contract: route, read with SHA-256, then append cautions or ADRs via project_docs_append, or revise one document at a time via project_docs_revise. Use when a completed change, solved failure, new command, or durable decision should be reflected in project docs. For initial creation use project-docs-bootstrap; for structural modularization use project-docs-optimize."
---

# Project Docs Update

## Goal

Keep existing `.agent-harness` documents current while work happens, without
losing user consensus and without restructuring. Knowledge lands as
append-only records by default; full-document replacement is the exception.

## Lifecycle Position

`project-docs-bootstrap` (create) → **this skill** (incremental refresh) →
`project-docs-optimize` (restructure). If `.agent-harness` documents are
missing, route to `project-docs-bootstrap` first. If documents are oversized
or ownership is duplicated, route to `project-docs-optimize`; do not
restructure here.

## When to Trigger

Route the current task first to avoid injecting every doc into context:

```bash
agent-harness project route-docs --repo . --task "<task kind>" --json
```

| Work outcome | Action |
|---|---|
| Solved false case, recurring failure, operational risk | `project_docs_append(kind=caution)` |
| Durable decision with rationale or rejected alternatives | `project_docs_append(kind=adr)` |
| Changed command, convention, architecture fact, or stale doc section | `project_docs_revise` on that one document |
| Only needs doc context for the task | route/read only; no write |

Trigger this skill when a completed unit of work produced one of the left-side
outcomes. Do not batch updates speculatively for hypothetical future work.

## Decision Table: append vs revise

| | `project_docs_append` (default) | `project_docs_revise` (exception) |
|---|---|---|
| Shape | Append-only dated record in CAUTIONS/ADR | Full document replacement |
| Use when | New knowledge is a discrete event/decision | An existing section is stale or wrong |
| Consensus risk | None (additive) | Requires preserving stronger existing guidance |
| Locking | Not required | `expected_sha256` from `project_docs_read` |
| Confirm | Not required | `confirm=true` only after the replacement preserves stronger guidance |

One document per update. Never rewrite multiple documents in one pass.

## MCP Contract

1. `project_docs_route` — map the task to the owning documents.
2. `project_docs_read` — read the target document, keep its `sha256`.
3. `project_docs_revise` with `expected_sha256`, `summary` (why), `evidence`
   (user instruction, files, commands), `confirm=true` — or
   `project_docs_append(kind=caution|adr)` for append-only knowledge.

CLI fallback: `agent-harness project append --kind caution|adr` covers
records. There is no CLI for full-document revision; when MCP is unavailable,
edit the file directly with the same evidence discipline and state that MCP
was bypassed in the report.

## Safety Rules

1. Every statement written must be backed by a file, command output, or
   explicit user instruction. Mark anything else `Unknown / not confirmed`.
2. Preserve user consensus: `project_docs_revise` replacements must keep all
   stronger existing guidance; when in doubt, append rather than replace.
3. Never edit `AGENTS.md` outside the behavioral top block and managed marker
   block unless explicitly requested.
4. Never restructure documents (moving sections, splitting files) — that
   belongs to `project-docs-optimize` and its checker contract.
5. Do not update CAUTIONS or ADR for hypothetical issues; only concrete
   solved cases and real decisions.
6. Honor `expected_sha256` failures: re-read, reconcile the intervening
   change, then retry — never force an overwrite.

## Verify

- Re-read the updated document and confirm the change and only the intended
  change landed.
- Run the smallest relevant repo check when the update claims workflow or
  command facts.
- Report revised/appended documents, evidence, commands run, and remaining
  unknowns.

## Completion Evidence

List: documents revised or entries appended, the evidence for each, the
route/read/update sequence used, verification results, and anything left as
`Unknown / not confirmed`.
