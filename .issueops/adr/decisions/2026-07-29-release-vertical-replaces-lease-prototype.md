# 2026-07-29 — Release vertical replaces the lease prototype

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: GitHub #196
- Decision: Promote the stable v1 contract and release use case into
  `internal/contract`, `internal/domain`, `internal/application`, and
  `internal/adapter` production packages. Production CLI/MCP release invokes
  a typed handler injected by `cmd/issueops/issueopsapp`; it does not silently
  fall back to the legacy implementation. The legacy two-argument facade is
  retained only for source compatibility and byte-differential evidence.
- Rationale: Release needs a narrow transaction, process, path, and clock
  capability without moving unrelated execution actions or changing v1 bytes.
  Keeping composition in the harness app makes two MCP server instances and
  CLI calls own their handler dependency explicitly.
- Rejected alternative: A package-global release service or generic repository
  registry would conceal handler ownership and make dependency capture leak
  across transports.
- Rejected: Importing production `model` from the test contract (couples the
  ratchet subject to the system under comparison), preserving raw source JSON
  bytes (diverges from current typed re-marshal), and calling `clock.Now`
  before `Update` or before domain validation.
- Verification: differential success/denial byte snapshots including rich
  sidecars and `repo: null`, architecture import-ratchet tests, and blocking
  clock tests for valid and rejected transitions.
