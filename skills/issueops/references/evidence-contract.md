# IssueOps Evidence Contract Rules

Use these rules when an IssueOps cycle touches domain behavior, endpoint/API contracts, live runtime state, review feedback, or final PR/MR readiness. Keep the content portable: do not embed repository-specific domain names, environment names, credentials, or branch names in this reference.

## Domain Contract Evidence

Before implementation, write the domain contract in the issue or plan evidence section:

- **Invariant**: the user-visible behavior that must stay true.
- **Exact mechanism**: the documented or expected implementation path, with file/line or command evidence when available.
- **Equivalent behavior**: if the exact mechanism is absent, state whether another verified path enforces the same invariant.
- **Source evidence**: cite current files, specs, logs, or command output. Do not rely on memory alone.

When comparing a spec/wiki/doc with code, report two layers separately:

1. Whether the exact documented mechanism exists.
2. Whether the same invariant is enforced elsewhere.

Do not collapse "exact mechanism missing" into "feature missing" unless end-to-end behavior is also unverified or contradicted.

## API Documentation Gate

If endpoint, controller, DTO, schema, OpenAPI, Swagger, or public error behavior changes, the plan and PR/MR draft must include API documentation evidence:

- changed endpoints or an explicit statement that no endpoint contract changed;
- public error responses reachable from service/usecase/error mapping code;
- static API-doc check result when the target repo provides one;
- agent/API-doc review result when business logic can change visible 400/401/403/404/409-style responses;
- targeted verification commands used to avoid blaming unrelated legacy API-doc debt.

Prefer the target repo's documented API-doc command. If none exists, record the absence and use the closest available static/review check.

## Live Evidence Matrix

When the issue involves runtime behavior, environment drift, deployed configuration, external service endpoints, or production/staging differences, build a compact matrix before changing code:

| Environment | Repo/config evidence | Runtime evidence | Failure path | Remediation order |
| --- | --- | --- | --- | --- |

Rules:

- Separate source-code diffs from live DB/config/env/pod/log evidence.
- Separate similar-looking failure paths before proposing one root cause.
- Probe the actual runtime surface when possible. If local network reachability differs from the workload network, say which validation remains local-only and which requires same-network or in-workload proof.
- Record remediation order when multiple fixes are needed.

## Review Feedback Accountability

For each review, QA, CI, or user-feedback item, record:

- **classification**: `contract_change`, `defect`, `question`, `noise`, `valid_review`, `stale_review`, `rollout_evidence_missing`, or `environment_debt`;
- **verification**: file/line, command output, diff evidence, or live evidence used to decide validity;
- **thread reply**: whether the original review thread/discussion was answered with verdict and evidence;
- **resolution**: unresolved, fixed, resolved, obsolete, or split to follow-up.

Apply only verified feedback. If feedback changes acceptance criteria, non-goals, verification, labels, related links, or implementation scope, update the remote issue body before continuing.

## Completion Hygiene

Before reporting an IssueOps cycle ready or done, verify and record:

- final diff summary from the actual worktree;
- target branch and source branch;
- remote issue and PR/MR body freshness against the final implementation;
- linked issue labels copied or explicitly replaced for the PR/MR;
- single-commit policy or the explicit reason multiple commits are acceptable;
- branch divergence and clean working tree;
- worktree cleanup status or numbered cleanup choices.

Also write a **draft issue completion record** before final reporting. This record belongs in the remote issue update, PR/MR-ready notes, or another inspectable draft artifact before the remote write. It must include the final diff summary, verification evidence, selected labels, linked child work items, PR/MR URL, review-agent thread status, cleanup status, and unresolved follow-ups.

Do not treat passing tests alone as completion evidence when the requested outcome includes remote artifact updates, review-thread replies, merge readiness, or cleanup.
