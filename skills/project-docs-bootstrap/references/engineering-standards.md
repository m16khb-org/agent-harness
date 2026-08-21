# Engineering Standards Catalog

Reference catalog of widely accepted development standards for the
project-docs enrichment pass. The enrichment agent uses this catalog as a
**checklist of topics to evaluate against repository evidence**, never as
copy-paste content for target repos.

Usage contract:

1. For each topic below, look for the listed evidence signals in the repo.
2. Only topics confirmed by evidence (source files, config, commands, or
   explicit user instruction) are written into `.agent-harness` docs.
3. Confirmed topics go to the owning document listed in the
   [topic-to-doc map](#topic-to-doc-map). Adoption decisions with real
   trade-offs also get an `adr/` record.
4. Unconfirmed or not-applicable topics are either omitted or explicitly
   marked `Unknown / not confirmed` — never presented as adopted.
5. Keep repo doc content within the manifest line budgets; link to this
   catalog's concepts by name instead of restating whole sections.

---

## Architecture styles

### Layered (n-tier) architecture

Organizes a system into horizontal layers with separate responsibilities —
typically presentation, application/service, domain (business logic), and
persistence/data access — where each layer only depends on the layer beneath
it (Fowler, *Patterns of Enterprise Application Architecture*, catalog:
<https://martinfowler.com/eaaCatalog/>).

- Apply when: the codebase already shows layer-shaped directories or module
  boundaries (`controllers/` + `services/` + `repositories/`, handler →
  usecase → dao, and similar).
- Evidence signals: layered directory names, dependency rules in lint/arch
  tests, request flowing through exactly one layer per hop.
- Good: dependencies point one direction (downward); domain layer has no
  framework/persistence imports; layer responsibilities documented in
  `ARCHITECTURE.md`.
- Bad: skip-layer calls (controller → repository), domain importing UI
  types, "anemic" layers that only forward calls.
- Doc placement: style description and layer diagram → architecture family;
  concrete layer dependency rules → conventions family.

### Hexagonal architecture (ports and adapters)

Application core works without UI, database, or external services; the core
defines **ports** (interfaces) and technology-specific **adapters** connect
them, so business logic never leaks into I/O code (Cockburn, 2005:
<https://alistair.cockburn.us/hexagonal-architecture>).

- Apply when: inbound adapters (HTTP/CLI/MCP/queue consumers) and outbound
  ports (storage, external SDK, network) are explicit interfaces and the
  core is testable without them.
- Evidence signals: `port/`-style interface packages, adapter directories
  per technology, core packages with no infrastructure imports, tests that
  run the core with fakes.
- Good: every external technology reached through a port; symmetric
  treatment of driving (inbound) and driven (outbound) sides.
- Bad: interfaces named after the implementation, core importing adapter
  packages, ports with only one caller and no substitutability.
- Doc placement: port/adapter boundary description → architecture family;
  where new ports are introduced and why → `adr/` records.

### Onion and Clean Architecture

Concentric variants of the same dependency-inversion idea: all dependencies
point inward toward domain/application layers, and infrastructure, UI, and
frameworks sit in outer rings (Palermo, *Onion Architecture*, 2008:
<https://jeffreypalermo.com/2008/07/the-onion-architecture-part-1/>;
Martin, *The Clean Architecture*, 2012:
<https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html>).

- Apply when: entities/use-cases are framework-independent and outer layers
  implement inner-layer interfaces.
- Evidence signals: use-case or entity packages with zero framework imports;
  crossing/ring dependency tests; presenters/ gateways style naming.
- Good: crossing arrows tested or lint-enforced; the dependency rule stated
  in `ARCHITECTURE.md`.
- Bad: "clean architecture" folder names with framework imports in the
  entity layer; use-cases that are pass-throughs with no logic.
- Doc placement: ring/dependency rule → architecture family.

### Domain-Driven Design (DDD)

Strategic design: each **bounded context** has its own model and
**ubiquitous language** shared by developers and domain experts
(Fowler, *BoundedContext*:
<https://martinfowler.com/bliki/BoundedContext.html>; Evans, *Domain-Driven
Design*, 2003). Tactical patterns: entities, value objects, aggregates,
repositories, domain services, domain events. Practical guidance on where
the complexity payoff actually is: <https://vladikk.com/2016/04/05/tackling-complexity-ddd/>.

- Apply when: the domain is genuinely complex (many invariants, evolving
  business rules, multiple teams/models) and naming matches the business
  vocabulary.
- Evidence signals: ubiquitous-language terms in code and tests; explicit
  aggregate boundaries; value objects instead of primitives; context maps.
- Good: ubiquitous language documented in `CONVENTIONS.md`; aggregate
  invariants enforced inside the aggregate; one repository per aggregate
  root.
- Bad: tactical-pattern ceremony (repository + factory + events) on a CRUD
  app; shared "common" model across contexts; anemic entities with all
  logic in transaction scripts.
- Doc placement: bounded-context map → architecture family; naming and
  aggregate rules → conventions family; decision to adopt/skip DDD → `adr/`.

## Object-oriented design

### SOLID

Five principles for managing dependency direction and responsibility
(Martin, *Design Principles and Design Patterns*, 2000):

- **S**ingle responsibility — one reason to change per module (actors, not
  actions).
- **O**pen/closed — extend behavior by adding code, not editing saturated
  branches.
- **L**iskov substitution — subtypes must be usable through the base
  contract without surprise (also: design by contract).
- **I**nterface segregation — small, caller-specific interfaces.
- **D**ependency inversion — high-level policy depends on abstractions, not
  concrete infrastructure.

Apply together with YAGNI/KISS: SOLID clarifies responsibility and
dependency direction only where a real axis of change exists. The bootstrap
static guidance already covers the good/bad case split; enrichment maps
each principle to concrete repo examples (which interface exists because of
which variation point) in the conventions family.

- Good: an interface exists because there are two implementations, an
  external boundary, or a test seam — and that reason is written down.
- Bad: interface-per-class "for DI", registries/factories for single use
  sites, deep inheritance to satisfy LSP on paper.

### OOP fundamentals and composition over inheritance

Encapsulation, abstraction, polymorphism, and the GoF rule: **favor object
composition over class inheritance** (Gamma, Helm, Johnson, Vlissides,
*Design Patterns*, 1994; overview:
<https://en.wikipedia.org/wiki/Composition_over_inheritance>).

- Good: inheritance only for genuine is-a substitutability; behavior reuse
  via composition/delegation; invariants enforced in one place.
- Bad: inheritance for code reuse across unrelated concepts; base classes
  knowing subclasses; protected mutable state shared across a hierarchy.
- Doc placement: composition/inheritance and visibility rules with repo
  examples → conventions family.

## Clean code and readability

Practices from Martin, *Clean Code* (2008): meaningful and searchable
names; small functions that do one thing at one level of abstraction;
functions organized to read top-down; minimal arguments; no hidden side
effects; errors handled rather than silenced; comments explaining why, not
what; tests as readable specifications; the boy-scout rule (leave code
cleaner than found) applied without unrelated refactors.

- Good: a new contributor can follow naming to intent without tribal
  knowledge; dead code and commented-out blocks are absent.
- Bad: encoding types in Hungarian-style prefixes, `data2`/`util`/`manager`
  as names, 200-line functions with nested conditionals, silent `catch {}`.
- Doc placement: naming and function-size conventions with repo examples →
  conventions family; readability regressions worth remembering →
  `cautions/` records after they happen.

## Error and exception handling

Cross-language guidance (verify which style the target repo actually uses):

- **Errors as values** (Go `error`, Rust `Result<T,E>`): return errors,
  check them at each boundary, wrap with context (`fmt.Errorf`/`anyhow`
  style), reserve panics/`unwrap` for programmer bugs.
- **Exceptions**: throw for exceptional conditions, not control flow; catch
  the narrowest type; never swallow (`catch {}` / `except: pass`); convert
  at boundaries into typed error contracts.
- **Fail fast**: validate at system boundaries; do not carry invalid state
  deeper; make illegal states unrepresentable where practical.
- **HTTP error contracts**: map internal failures to documented statuses
  (validation → 400, auth → 401, permission → 403, missing → 404, conflict
  → 409); prefer a consistent machine-readable envelope such as RFC 9457
  Problem Details (<https://www.rfc-editor.org/info/rfc9457/>) when the API
  has no existing envelope.

- Doc placement: language/repo error style with examples → conventions
  family; HTTP status mapping rules → `OPEN_API_SPEC.md` (it is the
  single-owner for OpenAPI requirements, including error responses).

## API documentation (Swagger / OpenAPI)

Spec-first practice for HTTP APIs (OpenAPI Specification:
<https://spec.openapis.org/oas/v3.1.0>; practical error docs:
<https://swagger.io/blog/problem-details-rfc9457-api-error-handling/>):

- Document every operation with a client-oriented summary and sectioned
  description; every parameter with requiredness/format/example; every
  client-handled error status with schema — success-only docs are a defect.
- Error responses must mirror the business logic the endpoint calls (the
  static `api-doc` gate + agent review enforce this in agent-harness
  repos).
- Keep a single source of truth for the spec; generated docs drift silently
  when hand-edited copies exist.
- Doc placement: `OPEN_API_SPEC.md` owns this topic. If a repo has multiple
  API surfaces, list them there and keep per-surface detail as modules
  linked from it.

## Testing best practices

- **Test pyramid / test sizes**: many small fast unit tests, fewer
  integration tests, few end-to-end tests (Fowler, *The Practical Test
  Pyramid*: <https://martinfowler.com/articles/practical-test-pyramid.html>;
  Google Testing Blog, *Test Sizes*:
  <https://testing.googleblog.com/2010/12/test-sizes.html>;
  *Just Say No to More End-to-End Tests*:
  <https://testing.googleblog.com/2015/04/just-say-no-to-more-end-to-end-tests.html>).
- **Test doubles**: pick dummy/fake/stub/spy/mock deliberately; mocks verify
  interactions, stubs supply answers (Fowler, *Mocks Aren't Stubs*:
  <https://martinfowler.com/articles/mocksArentStubs.html>). Mock only the
  boundaries you own or fake; over-mocking locks implementation structure.
- **Structure**: Arrange-Act-Assert / given-when-then; one behavior per
  test; F.I.R.S.T. (fast, isolated, repeatable, self-validating, timely).
- **Determinism**: no reliance on wall-clock, ordering, real network, or
  machine state; flaky tests are bugs, not retries (also the harness-wide
  rule in TESTING.md templates).
- **Coverage discipline**: test observable behavior through public
  contracts; assertion per real bug or requirement; broad snapshot/golden
  updates must state intent and scope.
- Doc placement: repo-specific commands, good/bad examples, known flaky
  areas → testing family (`TESTING.md` root + `testing/` modules).

## Adjacent topics ("etc.")

Evaluate briefly; document only what the repo uses:

- **KISS / YAGNI / DRY**: simplest working design; no speculative
  features; DRY against duplicated *knowledge*, not duplicated text.
- **CQRS / event-driven**: separate read/write models only when shapes
  genuinely diverge (Fowler, CQRS:
  <https://martinfowler.com/bliki/CQRS.html>).
- **12-factor / configuration**: config from environment, secrets never in
  code or docs (12factor.net).
- **Versioning**: API and persisted-state versioning strategy, backward
  compatibility rules.
- **Security basics**: input validation at boundaries, least privilege,
  secret redaction in logs/docs, dependency hygiene (OWASP Top 10 as an
  awareness list, applied to real surfaces).
- **Observability**: structured logs with consumer-routed levels, metrics
  for real SLOs, traces where a chain exists to follow.

---

## Topic-to-doc map

| Topic | Owner (single) | Where detail lives |
|---|---|---|
| Layered / hexagonal / onion / clean style actually used | `ARCHITECTURE.md` | `architecture/` modules |
| Layer & dependency rules, ports placement | conventions family | `conventions/` modules |
| DDD bounded contexts & context map | `ARCHITECTURE.md` | `architecture/` modules |
| DDD naming / aggregate / value-object rules | conventions family | `conventions/` modules |
| SOLID, OOP, composition-over-inheritance, clean-code conventions | conventions family | `conventions/` modules |
| Error/exception style (language level) | conventions family | `conventions/` modules |
| HTTP error contract & OpenAPI/Swagger rules | `OPEN_API_SPEC.md` | root doc (single owner) |
| Testing strategy, doubles, flakiness | `TESTING.md` | `testing/` modules |
| Adoption/skipping decisions with trade-offs | `ADR.md` | `adr/` records (dated) |
| Recurring false cases and incident lessons | `CAUTIONS.md` | `cautions/` records (dated) |

The map follows the same single-owner rule the optimize checker enforces:
each topic has exactly one normative owner; other documents link to it.
