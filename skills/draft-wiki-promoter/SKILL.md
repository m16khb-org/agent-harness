---
name: draft-wiki-promoter
description: Use when judging .agent-harness/draft-wiki candidates, turning agent notes into reviewable draft wiki files, approving/rejecting drafts, or promoting approved drafts into the repo-local exported directory.
---

# Draft Wiki Promoter

## Purpose

Curate short-term agent notes into durable, reviewable knowledge without letting automation pollute long-term storage. Drafts are repo-local review artifacts; only approved drafts may be promoted, and promotion is a repo-local export the user moves to their own knowledge base.

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

`draft-wiki suggest` uses Z.AI Coding Plan and defaults to `glm-5-turbo`. Use `--model` only when an explicit Z.AI model override is needed.

## Workflow

1. `list` drafts and read only relevant files under `.agent-harness/draft-wiki/{draft,approved,rejected}/`.
2. If creating a new candidate, run `suggest --dry-run` first. Run without `--dry-run` only when source scope and model are acceptable.
3. Judge with the required checks above.
4. Move accepted candidates from `draft/` to `approved/`; move failed candidates to `rejected/`. (`reject` accepts any source status — an already-approved draft can still be rejected before promotion.)
5. Promote only approved drafts with `promote --confirm`. This moves the draft to `.agent-harness/draft-wiki/exported/` and appends `exported/export.log`; it never writes outside the repo.
6. After promotion, report the exported path so the user can move it into their own repository or knowledge base.

## Boundaries

- Never run external LLM calls inside PostToolUse hooks; hooks may enqueue signals only.
- Never write into external wikis or companion tools from this skill. Promotion writes repo-local exported files only.
- Never delete rejected drafts unless the user explicitly asks.
