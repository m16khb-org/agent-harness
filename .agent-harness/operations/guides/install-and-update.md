---
name: install-and-update
description: Installation, bootstrap, refresh, daily operator commands, and release smoke routing.
---

# Install, Update, and Daily Commands

This guide owns the daily operator command set and the release smoke entrypoint
for the operations family. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Daily Commands

```bash
agent-harness bootstrap --dry-run --json
agent-harness bootstrap
agent-harness project bootstrap --repo /path/to/repo --dry-run --json
agent-harness project bootstrap --repo /path/to/repo --sync --json
agent-harness doctor --repo . --json
agent-harness status --json
agent-harness docs --json
```

`agent-harness doctor` is the daily health gate. The deep preserve-flag and
one-time reconciliation procedure lives in
[troubleshooting.md](troubleshooting.md).

## Release Smoke

```bash
scripts/release-repro-smoke.sh
```

Use `.agent-harness/operations/release-reproducibility.md` before deciding
Homebrew, tarball, or other release packaging.

## Related references

- [operations/install.md](../install.md): first-run install, `ah update`,
  command shims, MCP refresh, and `project bootstrap --sync`.
- [operations/release-reproducibility.md](../release-reproducibility.md):
  release checklist, clean-machine smoke, cross-platform build matrix, and
  rollback criteria.
- [operations/verification.md](../verification.md): self-verify and api-doc
  gates.
