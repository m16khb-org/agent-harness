# Engineering standards catalog for project-docs bootstrap and optimize

> Family index: [`../../ADR.md`](../../ADR.md)

- Date: 2026-08-21
- Status: accepted

## Context

project-docs-bootstrap generated `.agent-harness` docs whose enrichment
guidance listed only SOLID/YAGNI/KISS/design patterns. Standard development
topics users expect a fresh project doc set to evaluate — DDD, clean code,
layered/hexagonal/onion/clean architecture, OOP, testing best practices,
OpenAPI/Swagger, error/exception handling — had no catalog, so enrichment
coverage depended on what the running agent happened to know.

Static Go templates also had no topic checklist, so a static-only write
produced drafts that never surfaced these evaluation areas.

## Decision

1. Add `skills/project-docs-bootstrap/references/engineering-standards.md`:
   a research-backed catalog of the standard topics (with canonical
   sources), repo evidence signals, good/bad case splits, and a
   topic-to-doc map assigning each topic a single normative owner in the
   `.agent-harness` layout.
2. Wire it into the enrichment contract (`PROMPT.md`): the catalog is a
   checklist to evaluate against repository evidence, never copy-paste
   content; only evidence-confirmed topics are written; adoption or
   rejection decisions go to `adr/` records; unconfirmed topics are marked
   `Unknown / not confirmed`.
3. Wire it into `SKILL.md` (bootstrap) workflow step 4 and the
   static-vs-agent-filled boundary, and into project-docs-optimize's
   single-owner rule: the catalog's topic-to-doc map is the canonical
   ownership reference when restructuring, so standards modules are not
   dropped or genericized.
4. Extend the static pass (`internal/domain/projectdoc` constants +
   `internal/adapter/projectdocs` renderers) with a compact
   `Engineering standards checklist` in the conventions module overview and
   an architecture-style naming bullet in the architecture overview, so the
   topics are visible in fresh bootstraps before enrichment.

## Consequences

- Fresh bootstraps and refreshes consistently evaluate the same standard
  topic set, host-independently, through the skill source of truth in
  `skills/`.
- The catalog keeps the no-invention rule: generic catalog prose still
  cannot enter target-repo docs without repo evidence.
- The checklist grows the static conventions module overview; module line
  budget (250) still holds.
- Catalog maintenance is a skill-directory edit; no Go release cycle is
  needed for catalog content updates, only for static checklist wording.

## Alternatives rejected

- Expanding the Go static templates with the full catalog text: bloats
  every generated repo with generic advice the repo may not use and
  couples research updates to Go releases.
- Relying on the enrichment prompt alone without a catalog file: keeps
  coverage dependent on the running model's recall.
