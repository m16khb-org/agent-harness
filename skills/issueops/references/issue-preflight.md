# IssueOps Issue Preflight

Run this before creating, editing, or preparing a remote IssueOps issue. The goal is to turn the raw user request into a bounded issue contract prompt with less ambiguity. Do not create or edit remote artifacts until this preflight is complete or explicitly waived by the user.

## Deep-Interview Gate

Use a deep-interview style pass, preferably `omo:ulw-plan` or the nearest available LazyCodex/OMO deep-interview workflow, to identify:

- the user-visible problem and why it matters now
- success criteria that can be verified with tests and a real surface
- non-goals and scope boundaries
- domain terms that need source evidence
- required files, APIs, commands, or runtime surfaces
- open decisions that would materially change implementation

Ask only the questions that block a bounded issue. If the remaining ambiguity can be captured as an open decision without risking the wrong implementation, continue and put it in the issue contract.

## Ideal Issue Prompt Rewrite

Use repo-root `PROMPT.md` as the scaffold for an ideal issue prompt. Preserve its structure conceptually; do not paste placeholders into the issue. Rewrite the raw user request into these fields before remote issue creation:

- Identity: the role the implementing agent should take
- Objective: the concrete artifact or behavior to deliver
- Inputs: raw user request, repo context, required paths, constraints, and evidence sources
- Phases: context scan, analysis, output construction, verification
- Rules: no invented facts, no secrets, no unauthorized remote mutation, no scope broadening without issue updates
- Output contract: issue body sections, plan expectations, and verification evidence
- Verification checklist: tests, manual QA surface, remote artifact gates, and unresolved decisions

Record an ambiguity ledger with `resolved`, `deferred`, and `blocking` entries. Blocking entries stop remote issue creation until the user answers; deferred entries become explicit open decisions in the issue.
