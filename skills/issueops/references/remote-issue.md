# IssueOps Remote Issue And Artifact Rules

## Remote Issue First

When the user explicitly invokes `$issueops` and the repo remote, credentials,
target project, branch target, and issue ownership are discoverable, route the
issue phase to [`issueops-create-issue`](../../issueops-create-issue/SKILL.md).
The lifecycle may enter `plan` only after that skill records the canonical
remote issue and its verification evidence.

The focused creation skills resolve the concrete current user and pass it through
the provider-neutral `agent-harness issueops remote` commands. Do not put raw
provider mutation commands or placeholders such as `@me` in a skill workflow.

Before creating or editing the remote issue, proactively score related issues and labels. Gather candidate issues and labels from the target provider, build an `issueops remote score` request, and apply only selected candidates whose score is at or above the threshold. The count is not fixed; the threshold decides the set.

Provider candidate gathering:

- GitHub: use `gh issue list --state all --limit N --json number,title,body,labels,url,state` and `gh label list --json name,description,color`.
- GitLab: use the equivalent `glab issue list`/GitLab API issue fields and project label list.

Always inspect the current issue list and label list before deciding whether to create a new issue, link an existing issue, or create/update labels. If an existing issue already matches the requested work, link or update that issue instead of creating a duplicate. If the scoring gate selects a label that does not exist on the provider, create the missing label before issue creation or issue edit, then apply it. Do not create labels that were not selected by the scoring gate.

Run the deterministic score first:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge none --json
```

For semantic judgment, render the read-only `background_join` prompt:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge prompt --json
```

Give the returned `prompt` field to a fresh independent host agent. Save only
that agent's result JSON, then strict-decode it before any remote artifact write:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge file --judge-file issueops-remote-judge.json --json
```

If an independent host agent is unavailable or intentionally disabled, use the
deterministic result as the final scoring evidence and record that choice.

Default threshold is `0.70` unless the repo or user sets a stronger threshold. Attach selected related issues with the provider-native mechanism described in "Provider-Specific Linking And Hierarchy" below (GitHub body references vs GitLab linked items) — do not reuse one provider's style for the other. Include a compact scoring summary when it helps future reviewers understand why those links and labels were chosen, and apply selected labels with provider CLI/API commands. Do not apply rejected labels, create rejected labels, or link rejected issues. If label candidates existed but none met threshold, do not create an unlabeled remote artifact; stop before remote writes and either rerun scoring with corrected candidates or choose an explicit manual label with the reason recorded in IssueOps feedback.

The scoring summary is the **threshold-based label decision**. It must name selected labels, rejected labels, and manual override reason if the agent chooses or applies a label outside the scorer's selected set. A manual override is allowed only when the reason is evidence-backed, recorded in the issue draft or IssueOps feedback, and still passes the Korean Remote Artifact Gate before the remote write.

The agent must propose the operational choice instead of leaving the user to invent it. Example:

```text
관련 이슈/라벨 후보를 먼저 deterministic scorer로 점검하고, read-only prompt를 fresh host agent가 독립 평가한 결과만 `file` backend로 검증해 threshold 이상을 반영하겠습니다.
```

Only prepare a local issue draft instead of creating a remote issue when credentials, target provider, ownership, or branch target are unclear, or when the user explicitly asks not to create a remote issue.

If the agent realizes it implemented before creating or linking the issue, it must stop implementation, create or link the issue if possible, record corrective feedback in IssueOps state, and resume from the issue-linked plan.

## Publication Ownership

User-facing body examples and publication sequences live in the focused skills:

- [`issueops-create-issue`](../../issueops-create-issue/SKILL.md): parent Issue,
  provider-native child, readable Korean examples, score, and reconcile.
- [`issueops-create-pr`](../../issueops-create-pr/SKILL.md): linked PR/MR body,
  execution fence, publication, and live readback.

This reference keeps only shared provider rules. The CLI contract is:

- `--body` and `--body-file` are mutually exclusive.
- `--template` without a body renders a body; with a body, it validates the
  canonical sections.
- confirmed writes fail closed on critical validation, missing label/assignee,
  Korean artifact, or PR/MR target/base mismatch.
- `--field` aliases are defined by the renderer code and its tests; do not
  copy a second alias table into a skill.

Never add `## Plan Link`, `## Plan`, or a `TBD` placeholder to an Issue body.
Plan tracking belongs in IssueOps state and, when needed, the PR/MR body.

## Provider-Specific Linking And Hierarchy

For GitLab remote work, use the `gitlab-usecase` skill before creating or mutating issues, linked items, child items, MRs, review discussions, or cleanup state.

GitHub and GitLab expose similar concepts through different mechanisms. Never apply one provider's mechanism to the other; detect the provider first, then use its native feature. Use the authenticated provider CLI first (`gh api` for GitHub, `glab api` for GitLab); if auth, token scope, or permission errors block the CLI path in a host that provides a VCS MCP fallback, use that MCP fallback and verify the same remote fields afterward.

| Concept | GitHub mechanism | GitLab mechanism |
| --- | --- | --- |
| Related / non-hierarchical link | Cross-reference in the issue body (`#123` or full URL). GitHub has no native "linked items" relation, so body references are correct. | Native **linked items** (relation), not a body section. Create with `glab api projects/:id/issues/:iid/links -X POST -f target_project_id=<id> -f target_issue_iid=<iid> -f link_type=relates_to` (`relates_to` \| `blocks` \| `is_blocked_by`). |
| Parent → child work breakdown (tasks) | `agent-harness issueops remote create-child` creates and verifies the native hierarchy. | The same provider-neutral command owns GitLab child Task creation through its adapter. |
| Labels | Pass selected labels through `remote create-issue/create-child/create-pr`; verify live values. | Same command and verification contract; provider syntax stays inside the adapter. |
| Assignee | Pass the concrete username through the remote command; verify live value. | Same command; never use `@me` as a provider username. |

Rules:

- When the scoring gate selects related issues, attach them as **GitLab linked items** on GitLab and as **body cross-references** on GitHub. Do not put a `## Related Issues` body section on GitLab when a linked item is the correct home; do not invent a linked-items relation on GitHub where none exists.
- When breaking work into tasks/subtasks on the remote, match the provider's
  native parent/child relation. `remote create-child` creates, attaches,
  verifies, and records the child link; `link-child` is only for an already
  verified native child. GitLab child URLs may be `/-/work_items/:iid` Task
  URLs, not only `/-/issues/:iid`. Never flatten a supported hierarchy into
  ordinary sibling issues or body-only bullet lists.
- GitLab child Task creation is not a trial-and-error REST issue flow.
  `remote create-child --confirm` owns work-item type resolution, GraphQL
  creation, hierarchy attachment, and verification of hierarchy, labels, and
  assignees. A REST response may verify an already-existing child URL only;
  a failed native hierarchy operation blocks completion.
- Before creating child tasks, design the child execution graph to be parallelizable by default. Every child title, child task body, and parent child-task section must carry a mandatory `[p]` or `[s]` prefix: prefer `[p] parallelizable` for every child whose scope allows an independent start and independent verification, and reserve `[s] sequential` only for a child with a genuinely unavoidable cross-child dependency (one child's code, schema, migration, remote state, fixture, or decision output is a hard input to another and no contract/interface decoupling can remove that ordering). For each `[s]` child, state the specific unavoidable dependency that blocks parallelization; if no concrete hard dependency can be named, the child must be `[p]`. List prerequisites/dependencies (`none` for `[p]`) and place children in execution waves (parallelizable children first, then sequential children if any). If the dependency graph is unclear, stop for a user or owner decision before remote writes.
- When child work is merged into its parent work branch, close only the linked child task with `issueops cleanup close-children --merged --confirm`; leave the parent issue open as the umbrella until the full parent scope is merged to the mainstream target.
- After creating child tasks, verify the provider-native hierarchy, labels, assignee, and parent body update before reporting `parent body updated`. GitHub verification should inspect the parent sub-issues list and each child issue's labels/assignees. GitLab verification should inspect the parent work-item children/Tasks list and each child work item's labels/assignees.
- Run the **Large Issue Breakdown Gate** in
  [`issueops-create-issue`](../../issueops-create-issue/SKILL.md). This
  provider reference supplies hierarchy semantics; it does not duplicate the
  split decision or child-body rubric.
- When creating a PR/MR, copy labels from the linked issue into the provider create command. If the linked issue is unlabeled, apply an explicit manual label to the issue first or stop and record why no label can be chosen; do not create an unlabeled PR/MR. Label-copy flags such as `--copy-issue-labels` or GitLab issue-based MR flags such as `--with-labels` satisfy only the label requirement; the create command must still include an assignee flag for the current user.
- If a provider mechanism is unavailable (API/permission/feature flag), say so explicitly, fall back to the closest documented mechanism, and record the limitation in IssueOps feedback rather than silently using the other provider's style.

## Language And Writing Protocol

원격 아티팩트의 한국어 게이트, `fluent-korean` 호출, preview→confirm→readback 절차는
[`issueops-remote-write`](../../issueops-remote-write/SKILL.md)가 소유한다. 이 문서는
provider별 링크와 계층 규칙만 소유한다. 게이트 스크립트도 그 스킬이 번들한다.

원격 write 없이 남는 링크 규칙은 다음과 같다.

- Issue bodies must not contain `Plan Link`; GitLab relations belong in native
  linked items, not an invented `Related Issues` section.
- Every `remote create-*` request needs selected labels and a concrete assignee.
  Label-copy flags do not imply assignment, and `@me` is not a username.
- When `branch_prepare.base_branch` exists, PR/MR publication must send the
  same target branch. Policy returns `pr_target_branch_required` or
  `pr_target_branch_mismatch` before a write, with the expected branch in the
  warning. Cycles without a prepared branch are not judged.
- Attach GitLab relations through the native issue-links capability. Do not
  revive deprecated provider-specific creation aliases.

원격 issue 본문에는 repo-local plan path를 넣지 않는다. plan 파일은 ignored/untracked일 수 있으므로 `agent-harness issueops link-plan` state와 PR/MR 본문에서 필요한 경우에만 추적한다.
