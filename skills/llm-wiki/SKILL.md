---
name: llm-wiki
description: Query, read, cite, and curate the local LLM Wiki at ~/workspace/knowledge-base/llm-wiki. Use when the user asks to use llm-wiki, search durable knowledge, recall previous decisions, cite local source cards, capture reusable findings, ingest/update wiki pages, lint/audit the vault, or persist project/session knowledge outside the current repository.
---

# LLM Wiki

Use the local Obsidian-style LLM Wiki as durable, cross-session knowledge. The canonical vault root is:

```text
~/workspace/knowledge-base/llm-wiki
```

Prefer this wiki only when durable/project knowledge, prior decisions, source-card citations, or curated capture would materially improve the answer. Do not query it reflexively for every prompt.

## Operating rules

- Read `00-meta/AGENTS.md` before any wiki write, ingest, archive, or schema-affecting edit.
- Treat `00-meta/index.md` and `00-meta/log.md` as the canonical catalog and activity log.
- Treat `10-sources/` as read-only evidence. Do not modify source bodies; only allowed metadata URL maintenance may touch them, and only under `00-meta/AGENTS.md` rules.
- Write durable synthesized pages under `20-wiki/` and session captures under `30-sessions/` unless the schema says otherwise.
- Never edit `.obsidian/`; archive obsolete pages under `_archive/` instead of deleting.
- Cite wiki evidence with wikilinks such as `[[page-name]]` or `[[page-name#section]]`. Mark synthesis, human notes, and unverified claims explicitly.

## Query workflow

1. Decide whether wiki evidence is needed. Use it for previous decisions, project knowledge, local source cards, or citation-backed context.
2. If `agent_harness` MCP is available, first orient with `llm_wiki_session_context` or resource `harness://llm-wiki/session-context`. This loads current inventory and the safe operating rules without reading the whole vault.
3. Search narrowly. Prefer `agent_harness` MCP tools `llm_wiki_search` and `llm_wiki_read`; fall back to app tools such as `wiki_search`/`wiki_read`, then to focused shell search:

   ```bash
   rg -n "<focused terms>" ~/workspace/knowledge-base/llm-wiki/00-meta ~/workspace/knowledge-base/llm-wiki/20-wiki ~/workspace/knowledge-base/llm-wiki/10-sources
   ```

4. Read only the most relevant pages plus `00-meta/index.md` when orientation is needed.
5. Distinguish evidence from inference in the answer:
   - source-backed fact: cite `[[source-or-page]]`
   - agent synthesis: use `> 💡 Synthesis:` when writing wiki content, or state “추론” in the response
   - uncertain claim: label as unverified and say what would verify it
6. If the answer creates reusable knowledge, use `llm_wiki_capture` or perform a curated capture when the user requested persistence.

## Write / capture workflow

1. Read `~/workspace/knowledge-base/llm-wiki/00-meta/AGENTS.md` first. The `llm_wiki_capture` tool verifies that the schema exists before writing, but it does not replace judgment about where durable knowledge belongs.
2. Choose the destination:
   - `20-wiki/concepts/`, `20-wiki/entities/`, or `20-wiki/summaries/` for evergreen knowledge
   - `30-sessions/YYYY/` for session notes and work logs
   - `00-meta/reports/` for lint/audit reports
3. Use YAML frontmatter with at least: `title`, `type`, `status`, `created`, `updated`, `tags`, and when applicable `sources`/`related` as quoted wikilink lists.
4. Use kebab-case filenames. Keep pages concise; split pages that grow too broad.
5. On updates, change `updated:`, append a `## Changelog` entry, and append `00-meta/log.md` for meaningful changes.
6. Preserve source fidelity. Do not copy long copyrighted upstream text into the vault; use source cards, links, short excerpts, or raw snapshots only when allowed.

## Lint / audit workflow

For wiki hygiene requests, inspect for:

- broken wikilinks
- orphan or duplicate pages
- pages missing required frontmatter
- stale claims with old `updated:` dates
- generated/runtime artifacts that should be archived
- uncited facts or source-card claims needing upstream verification

Write reports to `00-meta/reports/*-YYYY-MM-DD.md` and log meaningful maintenance in `00-meta/log.md`.
