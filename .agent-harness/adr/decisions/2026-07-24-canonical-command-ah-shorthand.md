# 2026-07-24 — Canonical command with managed `ah` shorthand

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: user directive
- Summary: Keep `agent-harness` as the canonical command identity and install a collision-safe `ah` command symlink for concise use from any directory.
- Context: Native install already exposed `~/.local/bin/agent-harness`, but users wanted `ah update` everywhere. Adding only a second link was insufficient because executable-root discovery did not resolve the installed symlink back to the checkout outside the repository.
- Decision:
  - Install `~/.local/bin/ah -> ~/.local/bin/agent-harness` in every PATH mode; `manual` and `skip` continue to control shell rc changes only.
  - Resolve the executable symlink chain when locating the harness root, while preserving explicit `HARNESS_ROOT`, current-directory, and unresolved-executable fallbacks.
  - Treat an exact existing `ah` link as a no-op. Refuse to overwrite an existing regular file, directory, or unrelated symlink.
- Rationale: A managed command symlink works consistently in interactive shells, non-interactive commands, and scripts while reusing the canonical shim's checkout refresh. The strict collision policy protects a short, commonly claimed command name.
- Consequences: Install/update dry-run results expose both command links, and `ah update` can locate the checkout from outside it. `agent-harness` remains the name used by CLI output, MCP configuration, and host adapters.
- Rejected:
  - Shell alias — shell-specific and unavailable to many non-interactive callers.
  - Wrapper script — duplicates root and argument forwarding behavior instead of fixing canonical executable discovery.
- Verification: pathutil symlink-chain regression, install path-mode and collision tests, update resolved-root boundary test, installer help contract, and an isolated external-CWD `ah update --dry-run` smoke.
