# 2026-07-28 — Lease differential contract owns stable v1 canonicalization

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: #191 decision gates `msg_09208c28b563` and `msg_bb413022b7ae`
- Decision: The test-only leasevertical contract owns a stable v1 DTO that
  reproduces the current durable JSON type shape and decode/re-marshal
  canonicalization without importing `internal/core/issueops/model`.
  `internal/architecture` rejects every leasevertical contract import of a
  production IssueOps package. During release, application validates the
  domain request inside repository `Update`, reads its clock immediately after
  that validation, and then applies the transition.
- Rationale: A differential prototype must compare the current persistence
  contract without becoming coupled to its production DTO. Reading the clock
  before the repository span makes rejected transitions observe time and lets
  a blocked clock delay entry to the atomic update boundary.
- Consequences: The prototype intentionally duplicates the stable v1 JSON
  shape but remains test-only; rich sidecars and `null` normalization are
  compared against current persisted bytes. The domain keeps semantic release
  validation, while application owns the ordering of repository scope and its
  injected clock.
