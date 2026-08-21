# Bootstrap preserves in-progress repos and transparent plans

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-21
- Status: accepted

## Context

Dogfooding `project bootstrap` against real in-progress repositories
(nextcandle-api, galpi) surfaced three convention-respect defects:

1. On a repo with legacy flat `.agent-harness` family roots and no modular
   manifest, bootstrap created module starters and seeded
   `documentation/manifest.json` around the preserved flat roots. The
   result was a half-migrated layout the project-docs-optimize checker
   rejects (`module_dir_unlinked` on all six families), so bootstrap's own
   verification step failed on the very repos it should serve.
2. `ensureBehavioralGuidelinesAtTop` prepended the full English behavioral
   template over any AGENTS.md that did not start with the exact template
   header. nextcandle-api's Korean "Core behavior" block ended up with a
   competing English block stacked on top.
3. The dry-run plan reported `action: "update"` with no preservation
   signal; `family_docs_preserved` / `sync_available` warnings only
   appeared after a write. Agent consumers planning from the dry-run JSON
   were misled about what would change.

agent-harness is an open-source library applied to many projects; its
defaults must adapt to each repo, never the reverse.

## Decision

1. Legacy flat guard: when family root documents exist without the modular
   manifest, bootstrap writes no module starters and no manifest, keeps
   the flat roots untouched, and reports `legacy_flat_layout_preserved`
   directing restructuring to project-docs-optimize.
2. Curated AGENTS.md header rule: when AGENTS.md already opens with its
   own H1 heading, that content stays authoritative; the generic behavioral
   template is not prepended. Only the managed `AGENT_HARNESS` marker block
   is appended or refreshed in place.
3. Plan transparency: planned files carry a `preserved` boolean (true when
   an update-planned file will not be written), and preservation warnings
   (`family_docs_preserved`, `sync_available`, `legacy_flat_layout_preserved`)
   are emitted on dry-run as well as on write.
4. Skill contracts updated to match: project-docs-bootstrap gains the
   open-source library stance, the legacy-flat routing rule, the
   repo-conventions-and-language precedence rule, and plan-reading guidance
   for `files[].preserved`; the enrichment prompt gains a "Respect existing
   conventions and language" section; the engineering-standards catalog
   gains an explicit precedence rule (repo conventions outrank catalog
   ideals); project-docs-optimize's hard rules keep repo-authored
   conventions, terminology, and document language intact during
   restructure.

## Consequences

- In-progress repositories keep authoritative AGENTS.md rules, doc
  language, and flat layouts; restructuring is a separate explicit
  project-docs-optimize engagement.
- Fresh repositories still receive the full folder-first layout unchanged.
- Plan JSON gains one backward-compatible boolean field
  (`files[].preserved`); CLI and MCP share the same DTO as required.
- Frontmatter normalization on existing docs remains additive-only and
  body-preserving.

## Alternatives rejected

- Migrating legacy flat roots to modular indexes inside bootstrap: mixes
  restructuring into creation, risks summarizing curated content away, and
  duplicates project-docs-optimize's owned stage.
- Prepending the behavioral template with translation: guessing languages
  and duplicating rules is worse than respecting the repo's own header.
