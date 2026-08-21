---
name: project-docs-optimize
description: Audit and modularize oversized agent-harness operating documents into canonical root indexes and focused folders without losing constraints, breaking project-doc contracts, or duplicating normative ownership. Use when asked to optimize, split, reorganize, or validate `.agent-harness` documentation, ADRs, testing guides, cautions, conventions, operations, or architecture docs.
---

# Project Docs Optimize

## Goal

Turn oversized `.agent-harness` operating documents into a navigable,
single-owner documentation system while preserving every actionable constraint
and every required project-doc entrypoint.

## Inputs

- repository root;
- `.agent-harness/documentation/manifest.json`;
- required root documents under `.agent-harness/`;
- the nearest `AGENTS.md` and project documentation rules.

Resolve script paths relative to this skill directory.

## Lifecycle Position

```
create              refresh during work            restructure
project-docs-bootstrap  ->  project-docs-update  ->  project-docs-optimize
```

This skill owns the restructure stage only. It reorganizes layout; it does
not refresh content. Route content changes (new cautions, ADRs, stale
sections) to `project-docs-update`, and missing-document setup to
`project-docs-bootstrap`. Deterministic triggers for this stage:

- `--mode report` shows one or more violations (line budget, single-owner,
  link integrity, required entrypoint);
- a root document exceeds its manifest budget;
- the user explicitly asks to reorganize, split, or restructure.

## Hard rules

1. Required root filenames remain canonical entrypoints. Do not replace them
   with compatibility copies or redirects.
2. Move detail; do not summarize away commands, constraints, decisions,
   failure modes, or evidence. Preserve the repo's own conventions,
   terminology, and document language while restructuring — this skill
   reorganizes layout, it does not normalize a repo's authored style.
3. Give every topic one normative owner. Other documents link to that owner.
   For standard engineering topics (architecture styles, DDD, SOLID, clean
   code, error handling, OpenAPI/Swagger, testing practice), the
   project-docs-bootstrap skill's `references/engineering-standards.md`
   topic-to-doc map is the canonical ownership reference: preserve those
   topics in their owning family modules and their cross-links instead of
   dropping or genericizing them.
4. Keep root indexes and detailed modules within the manifest line budgets.
5. Preserve relative-link integrity in both directions.
6. Do not edit generated OpenWiki pages.
7. Use `apply_patch` for every repository file change.
8. Validate machine contracts and real discovery surfaces after the move.

## Workflow

### Audit

Run the deterministic report before editing:

```bash
uv run --directory skills/project-docs-optimize python -m scripts.check \
  --root "$PWD" \
  --mode report \
  --json
```

Read every violating document. Classify each section by responsibility,
normative owner, historical status, and consumer. Record the measured inventory
in `.agent-harness/documentation/AUDIT.md`.

### Design

Update `.agent-harness/documentation/manifest.json` and
`.agent-harness/documentation/README.md` before moving content.

For each family:

- the required root owns the universal summary and navigation;
- the module directory owns focused procedures and rationale;
- dated ADRs and incident lessons use one file per record;
- cross-family rules link to their canonical owner.

### Modularize

Move one document family at a time:

1. create focused modules with the original detail intact;
2. replace the root monolith with a concise index;
3. add root-to-module and module-to-root links;
4. remove duplicated text from non-owning documents;
5. run the checker before moving to the next family.

Do not mix unrelated prose cleanup into the move.

### Verify

Run the strict check:

```bash
uv run --directory skills/project-docs-optimize python -m scripts.check \
  --root "$PWD" \
  --mode check \
  --json
```

Then verify the real project surfaces:

```bash
python3 scripts/validate-skill.py skills/project-docs-optimize
./bin/agent-harness docs --json
go test ./internal/adapter/projectdocs ./internal/domain/projectdoc -count=1
git diff --check
```

Read command output and inspect every reported broken link, missing owner,
oversized file, or undiscoverable required entrypoint. Do not weaken the
manifest to make a failure disappear.

## Completion evidence

Report:

- before and after line counts for each modularized root;
- the canonical module map;
- checker output with zero violations;
- project-doc discovery output;
- focused test and command exit codes;
- any retained document that exceeds budget and the explicit reason.
