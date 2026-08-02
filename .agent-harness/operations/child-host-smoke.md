# Reversible Child Host Smoke

`scripts/verify-child-host-smoke.sh` verifies one pushed child-worktree commit against fresh Codex and Claude sessions. It is intentionally fail-closed: the command refuses to mutate user integrations until the caller supplies the literal `--confirm-user-activation` flag and the clean local HEAD exactly matches the named remote branch ref.

The runner builds and dry-runs the child binary, records bounded digests of the currently installed source surfaces, activates the child checkout, verifies unchanged host versions, and runs one `empty_object` MCP episode per host. In the explicit smoke environment, the child hook processes append only `SessionStart`/`PreToolUse` names to a private marker because Codex does not project hook lifecycle notifications into `exec --json`; the host probes combine those markers with native MCP results. The receipt exposes only hook booleans, MCP call count, response SHA-256, exit code, and duration, and never persists native event transcripts.

Every post-activation exit path invokes the source checkout installer exactly once and compares the full restored activation digest with the pre-activation digest. A mode-`0600` receipt is atomically replaced inside a caller-provided mode-`0700` directory. A pass requires both host observations, exactly one MCP result per host, activated child binary identity, and exact source restoration.

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
