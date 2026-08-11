# 2026-08-04 — Completed reseed requires stamped current completion provenance

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: GitHub #304 and durable incidents `io-14a09ebb1b15`, `io-a3818bd20165`
- Decision: Completion receipts stamp their lease generation. A
  completion-bearing reseed archives that stamped generation, not the current
  lease generation. A current completion whose generation is absent or zero is
  invalid v1 state and cannot be repaired by request input. Missing or
  conflicting generation selections fail before artifact preparation and the
  raw-record CAS.
- Rationale: #261 retained a generation-4 completion in an active generation-5
  lease, while #237 retained a generation-1 completion in generation 2.
  `completed_at < replaced_at` can show that a receipt is stale but cannot prove
  its origin after multiple reseeds. Silent `current_generation-1` or timestamp
  inference would therefore corrupt append-only audit history.
- Consequences: preview and reseed trust only the stamped current completion.
  History remains append-only evidence and never becomes current authority.
  State JSON is never edited to backfill provenance.
- Rejected: request-selected fallback, current lease generation, timestamp
  heuristics, legacy wording, aliases, and silent compatibility paths.
