# Reversible Child Host Smoke

`scripts/verify-child-host-smoke.sh` verifies one pushed child-worktree commit against fresh Codex and Claude sessions. It is intentionally fail-closed: the command refuses to mutate user integrations until the caller supplies the literal `--confirm-user-activation` flag and the clean local HEAD exactly matches the named remote branch ref.

The runner builds and dry-runs the child binary, records bounded digests plus a private byte-for-byte snapshot of the currently installed source surfaces, activates the child checkout, verifies unchanged host versions, and runs one `empty_object` MCP episode per host. Before either episode, it validates the activated managed `SessionStart`/`PreToolUse` handler command, type, timeout, and key set exactly; a missing enforcement flag or appended shell suffix fails closed. Claude's temporary child commands then receive the private marker path explicitly. Codex instead derives its two projected commands from those validated activated handlers and runs with a private per-episode `CODEX_HOME`; the source auth file is referenced by a private symlink, while user config, plugins, and co-resident hooks are not loaded. The Codex episode uses its invocation-scoped `--dangerously-bypass-hook-trust` automation flag because the exact pushed child commands intentionally differ from persisted trust hashes; the runner does not edit or persist trust state, and the bypass can reach only the projected two-hook document. The child hook processes append only those event names because Codex does not project hook lifecycle notifications into `exec --json`; the host probes combine the markers with native MCP results. The receipt exposes only hook booleans, MCP call count, response SHA-256, exit code, and duration, and never persists native event transcripts. Its top-level `validation_lane` is always `native_host`; this receipt validates Codex/Claude adapters and is not Orca Run/task/dispatch/claim evidence.

Every post-activation exit path invokes the source checkout installer exactly once, atomically restores the four original configuration byte streams and modes from the private snapshot, and compares the full restored activation digest with the pre-activation digest. A mode-`0600` receipt is atomically replaced inside a caller-provided mode-`0700` directory. A pass requires both host observations, exactly one MCP result per host, activated child binary identity, and exact source restoration.

Example after the child branch has been pushed:

```bash
scripts/verify-child-host-smoke.sh \
  --issue 230 \
  --source-root /absolute/path/to/source-checkout \
  --child-root /absolute/path/to/clean-child-worktree \
  --head 0123456789abcdef0123456789abcdef01234567 \
  --remote-ref refs/heads/230-child-smoke \
  --json-out /absolute/private-directory/child-host-smoke.json \
  --confirm-user-activation
```

The command does not merge, delete a worktree, or clean a branch. Those remain separate IssueOps approval boundaries.
