---
title: "Draft wiki hook memory"
source: "claude-mem"
target_wiki: "dev-fundamentals"
target_type: "notes"
summary: "Decouple issueops draft-wiki suggestions by enqueuing them in user state and processing them asynchronously via a worker calling agy -p."
suggester: "agy -p"
model: "Gemini 3.5 Flash (Medium)"
---

# Agent-Harness Draft-Wiki Suggestion Architecture

To ensure reliable performance during agent execution and avoid blocking critical paths, draft-wiki suggestions are processed asynchronously.

## Design and Flow

1. **Enqueueing**: The `issueops` `PostToolUse` hooks enqueue draft-wiki suggestions into the user state.
2. **Worker Processing**: The background worker retrieves enqueued suggestions and invokes the generator (`real agy -p`) outside of the hook's critical execution path.
3. **Draft Output**: The worker writes the final, reviewable Markdown draft to the following directory:
   ```
   .issueops/draft-wiki/draft
   ```
