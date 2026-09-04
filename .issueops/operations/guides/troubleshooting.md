---
name: troubleshooting
description: Health gate, diagnosis, and one-time reconciliation procedures.
---

# Troubleshooting and Reconciliation

This guide owns the cross-system health gate and the one-time reconciliation
procedure. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Operational Health and One-Time Reconciliation

`issueops doctor` is the sole public cross-system health gate for canonical Git state, all IssueOps records/bindings, optional Orca inventory, and unexpected user-state artifacts. Invocation-only preservation never writes state:

```bash
issueops doctor --repo . --preserve-cycle EXACT_CYCLE_ID --preserve-terminal EXACT_HANDLE --json
```

- An active IssueOps execution is live only with a complete generation, native process receipt, canonical worktree, and mode-specific resource identity. Process absence alone never authorizes interrupt, deletion, or lease replacement; use the previewed generation-CAS replacement sequence and prove quiescence.
- Preserve flags are repeatable exact values for one doctor invocation. They do not create persistent exceptions or cure incomplete/duplicate identity.
- Orca remains optional. Absence is healthy only when no durable cycle claims Orca resources; otherwise inventory is unknown and doctor fails closed.
- The stability audit builds the binary, then delegates operational judgement to `doctor`. `--preserve-terminal EXACT_HANDLE` is a singular explicit assertion and takes precedence over the inherited environment; only when it is absent does a non-empty `ORCA_TERMINAL_HANDLE` remain the fallback. Sealed reconciliation passes its resolved `manifest.current_terminal.handle` explicitly and does not overwrite the environment variable.

The approved one-time full reconciliation uses an external mode-`0700` bundle at `~/.local/state/issueops-backups/<repo-fingerprint>/<UTC-timestamp>/`, not a product cleanup command. Git and SQLite backups are restore-tested; Orca snapshots are archival evidence only because the installed CLI exposes global reset but no conditional reset/import/restore. Stop before reset if the final full digest drifts. After a reset or crash seam, resume the sealed append-only journal and complete idempotently forward; do not infer a partial rollback.

## Related references

- [operations/stability-baseline.md](../stability-baseline.md): stability audit
  baseline that builds the binary then delegates to `doctor`.
- [issueops-execution.md](issueops-execution.md): generation-CAS replacement and
  recovery for failed execution holders.
