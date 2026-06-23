---
name: codex-system-skill-quick-validate.md
description: Evidence for the Codex system skill quick_validate.py upstream source path and PyYAML dependency.
---

# Codex system skill quick_validate.py source path

## Summary

The durable upstream source for Codex's built-in `skill-creator` sample is in
the official `openai/codex` repository, not in this `agent-harness` repository
and not in the local materialized `~/.codex/skills/.system` cache.

Confirmed source path:

```text
openai/codex
codex-rs/skills/src/assets/samples/skill-creator/scripts/quick_validate.py
```

## Evidence

- Local `codex --version` reports `codex-cli 0.142.0`.
- The pnpm shim at `$HOME/Library/pnpm/codex` invokes
  `@openai/codex@0.142.0`.
- That npm package lists only `bin/codex.js` in its `files` field, so the
  embedded system skill sources are not shipped as plain package files.
- GitHub code search for `quick_validate.py repo:openai/codex` returns:
  <https://github.com/openai/codex/blob/fbd575ab4a8f786a535223856e4b8fd8beb29458/codex-rs/skills/src/assets/samples/skill-creator/scripts/quick_validate.py>
- GitHub code search for `skill-creator repo:openai/codex` also returns
  `codex-rs/skills/src/lib.rs`.
- `codex-rs/skills/src/lib.rs` embeds `$CARGO_MANIFEST_DIR/src/assets/samples`
  as `SYSTEM_SKILLS_DIR`, writes it to `CODEX_HOME/skills/.system`, and uses
  `.codex-system-skills.marker` as the materialization fingerprint.
- The current upstream `quick_validate.py` imports external `yaml` and calls
  `yaml.safe_load`.

## Reproduction

The local materialized Codex copy currently passes with isolated Python because
it has been patched locally:

```bash
python3 -S $HOME/.codex/skills/.system/skill-creator/scripts/quick_validate.py skills/issueops
# Skill is valid!
```

The unpatched Claude marketplace copies still fail without PyYAML:

```bash
python3 -S $HOME/.claude/plugins/marketplaces/claude-plugins-official/plugins/skill-creator/skills/skill-creator/scripts/quick_validate.py skills/issueops
# ModuleNotFoundError: No module named 'yaml'
```

This confirms that local user-site PyYAML and local `.system` edits can mask the
failure. The durable fix target is the embedded upstream sample path above.

## Recommended upstream fix

Patch `openai/codex` at:

```text
codex-rs/skills/src/assets/samples/skill-creator/scripts/quick_validate.py
```

Use a small stdlib fallback for simple scalar frontmatter when `import yaml`
fails, while preserving `yaml.safe_load` behavior when PyYAML is available. Add
a regression test or fixture that runs the validator in an isolated Python mode
without site packages.

## Boundary

`agent-harness` should not treat edits under `~/.codex/skills/.system` as a
complete fix. That directory is generated from Codex's embedded system skill
payload and can be replaced whenever the marker fingerprint changes.
