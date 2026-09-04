# ADR decision log rules

← [ADR index](../ADR.md)

This module owns the status model, naming, authoring rules, and archived
history for issueops architecture decision records. The active
dated-decision listing lives in the [root ADR index](../ADR.md); the
implementation roadmap lives in [roadmap.md](roadmap.md).

## Status model

Legacy records under `decisions/` and append-generated records directly under
`adr/` are `accepted` when published. A record becomes `superseded` only when a
later dated record says so explicitly. Current supersessions include:

- 2026-05-29 "upstream companion tools were opt-in dependencies" is superseded
  by the 2026-07-07 standalone issueops policy.
- The `gh issue develop` step in #163's Orca ordering is superseded by the
  2026-07-26 sealed-base-SHA linking decision. The Orca-before-linking ordering
  itself is unchanged.
- The 2026-07-07 standalone policy's blanket prohibition on third-party
  installation is superseded only for the constrained declarative path in the
  2026-08-28 optional-upstream-provisioning decision. Standalone core success
  remains.

Historical host, schema, and command names inside dated records preserve the
rationale at the time of writing. They are not current support contracts. The
current operating surface is set by root `AGENTS.md`, `ARCHITECTURE.md`,
`OPERATIONS.md`, and the most recent explicit superseding decision.

## Naming and authoring

- Keep existing immutable records at `decisions/YYYY-MM-DD-<kebab-slug>.md`; do
  not move them.
- Create new decisions with `project_docs_append(kind=adr)` at
  `YYYY-MM-DD-<kebab-slug>.md` directly under this module.
- Decisions are append-only. Supersede a decision with a later dated record that
  explicitly names what it replaces. Use `project_docs_revise` only to correct a
  factual statement about the current path or name; preserve the rationale and
  record the amendment date.
- Same-day decisions use a distinguishing slug after the date.
- Legacy records link to `../../ADR.md`. A separate SHA-guarded revision of the
  root ADR index links append-generated records.
- Structured records keep their `Kind`, `Source`, `Summary`, `Context`,
  `Decision`, `Consequences`, `Evidence`, and `Alternatives / rejected options`
  fields. Unstructured records keep their original prose and headings.

## Archived decision history

Active decisions live in the [root ADR index](../ADR.md). Detailed dated notes
that are no longer on the hot reading path are preserved in full in
[`archive/adr-history.md`](../archive/adr-history.md). The ledger below
summarizes them so the rationale stays discoverable without re-entering the hot
path.

- LLM Wiki policy.
- 2026-05-29: upstream companion tools were opt-in dependencies; superseded by the 2026-07-07 standalone issueops policy.
- MCP-backed project memory records.
- Adapter separation, compatibility contract, audit log, and worker MVP.
- 2026-05-29: quality gate failures return normal MCP payloads.
- 2026-05-29: public command identity follows project identity.
- 2026-05-29: simple bootstrap/project-bootstrap commands.
- 2026-05-30: static project-doc catalog injection.
- 2026-05-30: host-specific UserPromptSubmit display contract.
- 2026-05-31: shared PreToolUse hook and prompt/tool lifecycle split.
- 2026-05-31: repo-local draft wiki staging before upstream LLM Wiki ingest.
- 2026-06-02: IssueOps split.
- 2026-06-04/05: next-action Stop hook is a trigger/evidence relay, not a judge/scorer/classifier/safety gate. The replacement is not a static scoring heuristic: UserPromptSubmit teaches the main agent the policy, while Stop only detects an explicit next-action review point and relays observed facts back to the main agent for all safety/reversibility/alignment/proceed-or-ask judgement. When re-entered by Stop choices, the main agent must state its rationale either way: why it is auto-proceeding now, or why it is not auto-proceeding and user confirmation is required. Auto-proceed result reports still end with `선택지:` so the next turn has an explicit user-facing action boundary. The only installed relay flag is `--relay-next-action-judgement`.
- 2026-06-10: Stop-hook no-auto-proceed responses must not recreate next-action choices. A real recovery turn showed that the prior phrase "auto-proceed result reports still end with choices" was too easy to over-apply to no-auto-proceed decisions, causing a repeated `선택지:` block after the agent had already stopped for user confirmation. The corrected contract is explicit: auto-proceed result reports still end with choices; no-auto-proceed judgements state the rationale and stop without adding another choices block.
- 2026-06-06: A main-agent `no-auto-proceed` judgement is sticky across automated goal continuation. A real Codex goal continuation resumed implementation immediately after the agent had just said it would not auto-proceed at a Stop-hook next-action boundary, producing contradictory behavior. UserPromptSubmit now injects the explicit rule that the same action must not resume after `no-auto-proceed` unless the user selects a choice or gives a new instruction.
- 2026-06-06: `stop_hook_active` suppresses only missing-choice recovery loops, not valid next-action judgement relay. A real Stop-hook recovery turn produced well-formed `선택지:` choices, but `runHookStop` returned `{}` because the relay path was incorrectly gated by `!stop_hook_active`. That caused the main agent to stop without the required proceed-or-ask judgement. The corrected contract is: missing numbered choices no-op while `stop_hook_active=true`; valid next-action choices still re-enter the main agent once, with duplicate relay suppression preventing loops. A recommended continuation is not permission to continue forever: on every relay, the main agent must state its safety/reversibility/alignment judgement and stop when scope is complete, review risk is too high, or user confirmation is needed.
- 2026-06-05: IssueOps worktree tool-root drift is handled without asking the user to restart the host in a different cwd. CodeGraph remains usable because its CLI supports `--path` and its MCP tools support `projectPath`; IssueOps prepares the worktree CodeGraph index and the worktree PreToolUse guard requires CodeGraph `projectPath` to equal the expected worktree. Source-root-bound filesystem/Serena MCP tools are blocked in IssueOps worktree implementation unless their root can be proven to be the expected worktree. For next-action UX, numbered choices are only for real user-decision points; if the safe next step is continued implementation or verification, the main agent should execute instead of ending with choices.
- 2026-06-05: IssueOps worktree isolation is fail-closed once a code-editing phase begins. The prior guard only blocked edits after an exact `worktree_path` was linked, so an agent could create or switch to the issue branch inside the source checkout and implement there. The PreToolUse guard now blocks `git checkout -b`/`git switch -c` for a known IssueOps branch in the source checkout, blocks mutating source/worktree targets during `implement`, `ai-slop-clean`, `feedback`, and `pr` when the cycle has no linked worktree, and still allows `git worktree add ../<repo>.worktrees/...` so the agent can create the required sibling worktree before running `issueops link-worktree`.
- 2026-06-05: IssueOps lifecycle phase promotion is fail-closed at the same boundaries as the worktree guard. A real CLI walk showed `issueops phase --to ai-slop-clean` and then `--to pr` could advance before any linked worktree existed, leaving the edit hook as the only later barrier. `ai-slop-clean` now requires linked issue, provider-linked branch evidence, linked plan, and an existing worktree before phase entry; `pr` now requires strict PR readiness. The VCS linking parser also treats `--copy-issue-labels` as label evidence while still requiring an explicit assignee flag, including GitLab API-oriented `--assignee-id`/`--assignee-ids`.
- 2026-06-05: IssueOps implementation-state links are fail-closed, not just later phase gates. A follow-up real CLI matrix showed `link-plan`, `link-worktree`, and `done` could still advance too early: plan could move to `implement` without an issue or verified branch, worktree state could point at a nonexistent path, and `done` could bypass PR readiness entirely. `link-plan` and `link-worktree` now require linked issue plus verified provider branch evidence, `link-worktree` requires an existing directory, and `done` requires prior `pr` phase. The CLI also treats `issueops --help` as a successful usage request, and structured MCP-style remote artifact inputs now preserve nested `flags.copy_issue_labels` and `flags.assignee` before VCS linking checks.
- 2026-06-05: IssueOps remote artifact ownership treats GitLab `mr for` as MR creation. The installed `glab` help shows `glab mr for` is deprecated but still creates an MR for an issue, and the Codex glab MCP exposes the same surface as `glab_mr_for`. The VCS linking parser now normalizes `glab mr for`/`new-for`/`create-for` and structured `glab_mr_for` to create actions, counts `--with-labels`/`flags.with_labels` only as label evidence, and still blocks creation unless an explicit assignee is inspectable.
- 2026-06-05: IssueOps start and feedback phases are strict lifecycle gates. A live CLI matrix showed `issueops start --branch main` could create a dead-end cycle that later rejected provider branch preparation, and `issueops phase --to feedback` could skip `ai-slop-clean`. `start` now requires an issue-number-prefixed IssueOps branch from the beginning, feedback items may still be recorded early without advancing the phase, and explicit feedback phase entry requires recorded `ai-slop-clean` evidence.
- 2026-06-05: IssueOps plan links must be real files on the active implementation surface. A live CLI matrix showed `link-plan` could move a cycle to `implement` with a nonexistent plan file, and `ai-slop-clean` could start even when the linked worktree lacked the plan path. `link-plan` now requires the plan file to exist in the source repo at link time, while `ai-slop-clean` and PR readiness require the linked worktree to contain the same plan path before later phases can proceed.
- 2026-06-05: GitLab remote artifact assignee evidence must be concrete, not placeholder-shaped. The installed `glab mr create` help accepts usernames, while deprecated `glab mr for` accepts numeric user IDs. The VCS remote artifact guard now rejects GitLab placeholder assignees such as `@me` and rejects username-shaped assignees for `glab mr for`/structured `glab_mr_for`, so agents must resolve the current username or numeric id first and verify the remote assignee list.
