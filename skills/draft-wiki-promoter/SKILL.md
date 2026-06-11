---
name: draft-wiki-promoter
description: Use when judging .agent-harness/draft-wiki candidates, turning claude-mem or agent notes into reviewable draft wiki files, approving/rejecting drafts, or promoting approved drafts into nvk/llm-wiki.
---

# Draft Wiki Promoter

## Purpose

Curate short-term agent memory into long-term LLM Wiki knowledge without letting automation pollute the wiki. Drafts are repo-local review artifacts; only approved drafts may be promoted into `nvk/llm-wiki` raw notes.

## Required checks

Before promotion, read the draft and verify:

- Durable: useful across future sessions, not a transient status update.
- Grounded: contains concrete decisions, commands, paths, or rationale traceable to the repo/session.
- Safe: no secrets, tokens, private personal data, or unreviewed sensitive logs.
- Scoped: `target_wiki` and `target_type` frontmatter are appropriate.
- Non-duplicative: target wiki does not already cover the same point obviously.

Reject instead of promoting if any safety check fails or the value is only temporary.

## Commands

```bash
agent-harness project draft-wiki init --repo . --json
agent-harness project draft-wiki list --repo . --json
agent-harness project draft-wiki suggest --repo . --input PATH --target-wiki dev-fundamentals --dry-run --json
agent-harness project draft-wiki approve --repo . --json PATH
agent-harness project draft-wiki reject --repo . --json PATH
agent-harness project draft-wiki promote --repo . --confirm --json PATH
```

The CLI also provides `queue` and `prune` subcommands: `queue` is the mechanism behind the hook boundary below (hooks enqueue signals; a worker processes them out-of-band), and `prune` trims old queue entries.

`draft-wiki suggest` uses `agy -p`. Antigravity CLI model selection is persistent configuration, not a per-call flag: verify `~/.gemini/antigravity-cli/settings.json` has the desired `model` value. Omit `--agy-model` to accept the currently selected settings model, or pass an exact label to enforce it. Do not pass `--model` to `agy`; current `agy` rejects that flag.

## Workflow

1. `list` drafts and read only relevant files under `.agent-harness/draft-wiki/{draft,approved,rejected}/`.
2. If creating a new candidate, run `suggest --dry-run` first. Run without `--dry-run` only when source scope and model are acceptable.
3. Judge with the required checks above.
4. Move accepted candidates from `draft/` to `approved/`; move failed candidates to `rejected/`. (`reject` accepts any source status — an already-approved draft can still be rejected before promotion.)
5. Promote only approved drafts with `promote --confirm`. This writes a raw note and appends `log.md` in the configured `nvk/llm-wiki` topic; it does not compile/query/index the wiki. Note: `nvk/llm-wiki` is the upstream plugin/repo name, not a CLI — promotion writes hub files directly (the local `llm-wiki` wrapper script only handles lint/archive).
   - Failure boundary when upstream is absent: if `~/.config/llm-wiki/config.json`, the hub path, or the wiki registry is missing, only `promote --confirm` fails (config/registry read error); `init/list/suggest/approve/reject` are repo-local and keep working. Report the exact blocked step instead of treating the whole skill as unavailable.
6. After promotion, report the raw path and suggest running the upstream `wiki` compile workflow if synthesis is needed.

## Boundaries

- Never run `agy -p` inside PostToolUse hooks; hooks may enqueue signals only.
- Never edit LLM Wiki compiled `wiki/` articles from this skill. Promotion writes raw source notes only.
- Never delete rejected drafts unless the user explicitly asks.
