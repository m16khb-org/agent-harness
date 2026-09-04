# Issue #261 Turing Report

Lifecycle `io-14a09ebb1b15` generation 4 implements explicit managed regular-file adoption for the canonical native command path. The default path remains fail-closed, while `--adopt-command-file` adds static Go build identity validation of the actual staged/canonical candidate, immutable file identity checks, a private same-directory backup, atomic exchange with displaced-object verification, pre-Seal rollback, and post-Seal recovery evidence.

Native activation now assigns an opaque transition ID at Begin, deletes any prior receipt in that transaction, and fences Seal/Abort by the exact transition and candidate identity. Direct install owns rollback plus Abort; explicit Seal rolls back the command path and returns `abort_required=true`, leaving the install script as the single pending-cleanup owner. The script chooses one exact candidate binary by the Begin digest, attempts Abort once, preserves the original exit status, and disarms only after a structured committed Seal receipt.

The wrapper completes path preflight before Begin and omits empty-array expansion for zero-flag Begin/Seal calls, preserving the same flow under macOS Bash 3.2 with `set -u`.

The canonical acceptance ledger is [issueops-v1-e80de0a23208d5ea.json](../verified-execution/issueops-v1-e80de0a23208d5ea.json). All targeted, contract-golden, full, race, vet, and build gates passed before the sealed-diff review. The live #248 exact-head native-host receipt remains the issue-defined post-review/post-merge lane; this change adds and passes the disposable regular-command round-trip evidence needed before that lane.
