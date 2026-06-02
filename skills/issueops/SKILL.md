---
name: issueops
description: Run an issue-driven work cycle from problem discovery through domain grilling, issue creation, planning, TDD/subagent implementation, feedback loops, and PR/MR drafting.
---

# IssueOps

Use this skill when the user wants a repeatable cycle from a vague problem to a GitHub/GitLab issue, implementation plan, tested change, feedback loop, and PR/MR.

## Contract

The workflow is advisory and agent-driven. Hooks may suggest this skill, but hooks must not create issues, edit files, run tests, or open PRs/MRs by themselves.

The cycle has one durable state record. Use `agent-harness issueops ... --json` or the matching MCP tools when the cycle needs to survive compaction, handoff, or another host.

Required phases:

1. Problem intake: use `superpowers:brainstorming` to clarify the actual problem, constraints, success criteria, and ambiguity.
2. Domain grill: use `grill-with-docs` to challenge terminology, existing domain model fit, and documentation updates before committing to an issue.
3. Issue contract: create or prepare a GitHub/GitLab issue that states the problem, acceptance criteria, non-goals, verification, and open decisions.
4. Plan: use `superpowers:writing-plans` to produce an issue-based implementation plan under the target repo's planning convention.
5. Implementation: use `superpowers:test-driven-development` for behavior changes and `superpowers:subagent-driven-development` when independent tasks can be split safely.
6. Feedback loop: collect user, review, QA, and CI feedback; classify each item; update the issue/plan when the contract changes; then continue implementation.
7. PR/MR: draft the PR/MR only after the issue URL and plan path are linked and the relevant verification has been run.

## State Commands

Start:

```bash
agent-harness issueops start --repo "$PWD" --branch "$(git branch --show-current)" --json
```

Link the issue:

```bash
agent-harness issueops link-issue --id "$ISSUEOPS_ID" --issue-url "$ISSUE_URL" --json
```

Link the plan:

```bash
agent-harness issueops link-plan --id "$ISSUEOPS_ID" --plan-path "$PLAN_PATH" --json
```

Record feedback:

```bash
agent-harness issueops feedback add --id "$ISSUEOPS_ID" --source user --body "$FEEDBACK" --json
```

Check PR/MR readiness:

```bash
agent-harness issueops pr-readiness --id "$ISSUEOPS_ID" --json
```

## Issue Template

Use this structure unless the target project already has a stronger issue template:

```markdown
## Problem

## Current Evidence

## Acceptance Criteria

## Non-goals

## Plan Link

## Verification

## Feedback Log
```

## Stop Conditions

Stop and ask the user before creating or updating remote issues, PRs, or MRs if credentials, target project, branch target, or issue ownership are unclear.

Stop before implementation if brainstorming or grilling exposes materially different interpretations. Present the interpretations and ask for the intended one.

Do not move to PR/MR drafting when `issueops pr-readiness` reports missing `issue_url` or `plan_path`.
