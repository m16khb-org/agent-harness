---
name: release-dogfood-notes.md
description: Codex and Claude release dogfood transcripts for the shared inspect/docs/state workflow.
---

# Release Dogfood Notes

This note captures the Phase 7 dogfood evidence for the first tarball/manual archive release path. The goal is to verify that Codex and Claude Code both point at the same `issueops` binary/MCP surface before a package-manager release is considered.

Measured on 2026-06-13 KST from `/Users/m16khb/Workspace/issueops`.

## Codex MCP transcript

Command:

```bash
codex mcp get issueops
```

Observed result:

```text
issueops
  enabled: true
  transport: stdio
  command: /Users/m16khb/Workspace/issueops/bin/issueops
  args: mcp
  env: ISSUEOPS_ROOT=*****
```

Interpretation: Codex resolves `issueops` through the rebuilt repository binary and the shared `mcp` entrypoint. The `ISSUEOPS_ROOT` value is redacted by the host output.

## Claude MCP transcript

Command:

```bash
claude mcp list
```

Observed result:

```text
issueops: /Users/m16khb/Workspace/issueops/bin/issueops mcp - connected
issueops_project: ./bin/issueops mcp - connected
```

Interpretation: Claude Code has both user-scope and project-scope `issueops` entries connected to the same repository binary shape. Unrelated Google Drive, Gmail, and Calendar MCP entries reported authentication needs; those are not part of the `issueops` release path.

## inspect/docs/state workflow

Command:

```bash
./bin/issueops inspect --json
./bin/issueops docs --json
tmp_state="$(mktemp -d)"
ISSUEOPS_STATE_DIR="$tmp_state" ./bin/issueops state maintain --json
rm -rf "$tmp_state"
```

Observed result:

- `inspect --json`: `ok=true`, `harness_root=/Users/m16khb/Workspace/issueops`, `codex_mcp_configured=true`, `claude_skill_installed=true`, `project_claude_mcp_config=true`.
- `docs --json`: `ok=true`; docs index includes `.issueops/operations/release-reproducibility.md`.
- `state maintain --json`: `ok=true` on an empty temp state root (this line replaced the retired schema-promotion command of the original 2026-06-13 transcript; schema promotion no longer exists).

## UX findings

- The shared binary/MCP model is working for both hosts, so first release risk is concentrated in installation and packaging rather than host-specific command drift.
- Claude Code currently exposes both `issueops` and `issueops_project`. This is acceptable for project dogfood, but release notes should tell users whether they are using user-scope or project-scope MCP to avoid duplicate-server confusion.
- Tarball/manual archive remains the better first release path because both hosts already resolve the repository binary directly, while Homebrew would add tap maintenance before more external dogfood exists.

## Next release check

Before promoting Homebrew or another package-manager path, rerun this note after:

```bash
issueops update
scripts/release-repro-smoke.sh
scripts/release-build-matrix.sh
./bin/issueops self-verify --seed=100 --target-score=95 --json
```
