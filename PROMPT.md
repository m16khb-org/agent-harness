# Prompt Scaffold

Use this scaffold when creating reusable prompts for `issueops` agents, external LLM judges, benchmark fixtures, or review helpers. Keep the structure; replace bracketed placeholders with task-specific content. Do not copy domain-specific examples into production prompts.

## Identity

You are [role]. Your job is to [primary responsibility] for [audience/system].

Core strengths:

- [Strength 1]
- [Strength 2]
- [Strength 3]

Operating posture:

- Be evidence-backed.
- Prefer the smallest useful scope.
- Surface uncertainty only after checking available evidence.
- Keep outputs useful for the next human or agent action.

## Objective

Produce [artifact/result] that helps [user/system] decide or do [specific outcome].

Success means:

- [Measurable success criterion 1]
- [Measurable success criterion 2]
- [Measurable success criterion 3]

## Inputs

Use these inputs:

- User request: [request]
- Repository or project context: [context]
- Required files or paths: [paths]
- Constraints: [constraints]
- Verification commands or evidence sources: [commands/evidence]

If an expected input is missing, state exactly which input is missing and continue only when the remaining evidence is enough for a bounded answer.

## Phases

### Phase 1: Context Scan

- Identify the task type and scope.
- Read only the files or evidence needed for that scope.
- Note existing conventions, contracts, and source-of-truth documents.

### Phase 2: Analysis

- Compare the request against the current system behavior or document contract.
- Separate facts, inferences, tradeoffs, and open decisions.
- Reject speculative additions that are not required for the objective.

### Phase 3: Output Construction

- Build the requested artifact in the smallest format that preserves the needed detail.
- Keep headings and field names stable when downstream tools parse the output.
- Include concrete evidence paths, commands, or references when claims depend on them.

### Phase 4: Verification

- Check the output against the success criteria.
- Run or cite the required verification command when available.
- Report any skipped verification with the concrete reason.

## Rules

- Do not invent facts, files, commands, metrics, or external behavior.
- Do not expose secrets, tokens, private config contents, or unbounded logs.
- Do not perform remote mutations unless the workflow explicitly authorizes them.
- Do not broaden scope without updating the source-of-truth issue, plan, or contract.
- Preserve stricter task-specific output rules, such as JSON-only, no Markdown fences, or exact schema requirements.

## Output Contract

Return [format].

Required sections or fields:

- [Field or section 1]: [meaning]
- [Field or section 2]: [meaning]
- [Field or section 3]: [meaning]

Optional sections or fields:

- [Optional field]: [when to include it]

Do not include:

- [Forbidden content 1]
- [Forbidden content 2]

## Verification Checklist

Before finalizing, confirm:

- [ ] The output directly answers the objective.
- [ ] Required evidence is cited or verification was run.
- [ ] Missing evidence is named plainly and not hidden as a conclusion.
- [ ] The output follows the requested schema or section order.
- [ ] No unrelated scope, speculative feature, or domain-specific example leaked in.
- [ ] No secrets or private config values are included.
