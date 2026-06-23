# IssueOps Issue Preflight

Run this before creating, editing, or preparing a remote IssueOps issue. The goal is to turn the raw user request into a bounded issue contract prompt with less ambiguity. Do not create or edit remote artifacts until this preflight is complete or explicitly waived by the user.

## Deep-Interview Gate

Use a deep-interview style pass, preferably **`von-neumann`** Phase 2 (Interview + Clearance Checklist), to identify:

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

## Remote Template Gate

Before a remote issue, child task, PR, or MR body is created or accepted, render or validate it through the shared IssueOps remote template contract:

```bash
agent-harness issueops remote render-template --kind issue --template implementation_task --provider github --title "$TITLE" --field problem="$PROBLEM" --json
agent-harness issueops remote create-issue --id "$ISSUEOPS_ID" --template implementation_task --field problem="$PROBLEM" --label "$LABEL" --assignee "$ASSIGNEE" --json
```

Use `--body-file` for manually drafted bodies. Do not combine `--body` and `--body-file`. If a manual body is used with `--template`, it must still satisfy the canonical required sections. Confirmed writes fail closed on critical validation failures, missing labels, missing assignees, Korean artifact gate failures, and PR/MR target branch mismatch.

## Plan-Prep Evidence Gate

Before entering the IssueOps `plan` phase, record three pre-plan evidence items with `agent-harness issueops plan-prep record`. Each item takes either concrete evidence or a mutually-exclusive waive reason:

- **prior-decision lookup** (`--decisions-evidence` | `--decisions-waive`): consult `.agent-harness/ADR.md` and prior IssueOps decisions for choices that constrain this work. Evidence is the relevant ADR/decision link; waive when no recorded decision touches this area.
- **related-issue scoring** (`--related-score-ref` | `--related-waive`): run `issueops remote score` against existing issues/labels and record the selected/rejected summary with the threshold. Waive when no comparable issues exist.
- **web research** (`--web-research-evidence` | `--web-research-waive`): when external API semantics, library behavior, or competitive context matter, capture a `berners-lee` research file or source. Waive for purely internal changes with no external semantics.

This gate is fail-closed for non-trivial intent classes: `plan`-phase entry is blocked until all three items carry evidence or a waive reason. A `trivial` intent class (set via `intent record --intent-class trivial`) skips the gate. Design review does not require plan-prep — it runs inside the plan phase, where the gate is already satisfied.
