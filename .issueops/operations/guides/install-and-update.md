---
name: install-and-update
description: Installation, bootstrap, refresh, daily operator commands, and release smoke routing.
---

# Install, Update, and Daily Commands

This guide owns the daily operator command set and the release smoke entrypoint
for the operations family. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Daily Commands

```bash
issueops bootstrap --dry-run --json
issueops bootstrap
issueops project bootstrap --repo /path/to/repo --dry-run --json
issueops project bootstrap --repo /path/to/repo --sync --json
issueops doctor --repo . --json
issueops system-status --json
issueops docs --json
```

`issueops doctor` is the daily health gate. The deep preserve-flag and
one-time reconciliation procedure lives in
[troubleshooting.md](troubleshooting.md).

## Release Smoke

```bash
scripts/release-repro-smoke.sh
```

Use `.issueops/operations/release-reproducibility.md` before deciding
Homebrew, tarball, or other release packaging.

## Related references

- [operations/install.md](../install.md): first-run install, `io update`,
  command shims, MCP refresh, and `project bootstrap --sync`.
- [operations/release-reproducibility.md](../release-reproducibility.md):
  release checklist, clean-machine smoke, cross-platform build matrix, and
  rollback criteria.
- [operations/verification.md](../verification.md): self-verify and api-doc
  gates.
