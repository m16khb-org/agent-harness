---
name: release-dogfood-notes.md
description: Codex and Claude release dogfood transcripts for the shared inspect/docs/state workflow.
---

# Release Dogfood Notes

This note captures the Phase 7 dogfood evidence for the first tarball/manual archive release path. The goal is to verify that Codex and Claude Code both point at the same `agent-harness` binary/MCP surface before a package-manager release is considered.

Measured on 2026-06-13 KST from `/Users/m16khb/Workspace/agent-harness`.

## Codex MCP transcript

Command:

```bash
codex mcp get agent_harness
```

Observed result:

```text
agent_harness
  enabled: true
  transport: stdio
  command: /Users/m16khb/Workspace/agent-harness/bin/agent-harness
  args: mcp
  env: HARNESS_ROOT=*****
```

Interpretation: Codex resolves `agent_harness` through the rebuilt repository binary and the shared `mcp` entrypoint. The `HARNESS_ROOT` value is redacted by the host output.

## Claude MCP transcript

Command:

```bash
claude mcp list
```

Observed result:

```text
agent_harness: /Users/m16khb/Workspace/agent-harness/bin/agent-harness mcp - connected
agent_harness_project: ./bin/agent-harness mcp - connected
```

Interpretation: Claude Code has both user-scope and project-scope `agent_harness` entries connected to the same repository binary shape. Unrelated Google Drive, Gmail, and Calendar MCP entries reported authentication needs; those are not part of the `agent-harness` release path.

## inspect/docs/state workflow

Command:

```bash
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
tmp_state="$(mktemp -d)"
HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state maintain --json
rm -rf "$tmp_state"
```

Observed result:

- `inspect --json`: `ok=true`, `harness_root=/Users/m16khb/Workspace/agent-harness`, `codex_mcp_configured=true`, `claude_skill_installed=true`, `project_claude_mcp_config=true`.
- `docs --json`: `ok=true`; docs index includes `.agent-harness/operations/release-reproducibility.md`.
- `state maintain --json`: `ok=true` on an empty temp state root (this line replaced the retired schema-promotion command of the original 2026-06-13 transcript; schema promotion no longer exists).

## UX findings

- The shared binary/MCP model is working for both hosts, so first release risk is concentrated in installation and packaging rather than host-specific command drift.
- Claude Code currently exposes both `agent_harness` and `agent_harness_project`. This is acceptable for project dogfood, but release notes should tell users whether they are using user-scope or project-scope MCP to avoid duplicate-server confusion.
- Tarball/manual archive remains the better first release path because both hosts already resolve the repository binary directly, while Homebrew would add tap maintenance before more external dogfood exists.

## Next release check

Before promoting Homebrew or another package-manager path, rerun this note after:

```bash
agent-harness update
scripts/release-repro-smoke.sh
scripts/release-build-matrix.sh
./bin/agent-harness self-verify --seed=100 --target-score=95 --json
```
