# IssueOps Child Issue Graph Design

## Goal

IssueOps should record parent-child work breakdowns as durable provider-neutral state, then expose that state through the same CLI and MCP surfaces. GitHub sub-issues and GitLab child items remain provider-specific remote mechanisms; this change records the shared IssueOps contract without creating remote issues or mutating provider state.

## Current Evidence

- `skills/issueops/references/remote-issue.md` already says parent-child work breakdowns should use GitHub sub-issues on GitHub and GitLab child items on GitLab.
- `internal/core/issueops.go` currently stores one `issue_url`, `plan_path`, `worktree_path`, and feedback list.
- `docs/IDD_IMPLEMENTATION_NEEDS.md` identifies durable issue graph support as a missing IDD capability.

## Scope

Add the first durable graph primitive: a child issue link from the main IssueOps issue to a work item that represents a task/sub-issue. The state stores the child URL, optional title, provider hint, relation type, and timestamp. The command is intentionally narrow: it links existing remote child artifacts after the agent or user has created them through the correct provider-native mechanism.

## API Shape

Core adds `IssueOpsIssueLink` with these JSON fields:

- `type`: currently `child`
- `url`: GitHub or GitLab issue/work-item URL
- `title`: optional short child title
- `provider`: inferred from URL when possible, or left empty for generic URLs
- `created_at`: UTC timestamp

CLI adds:

```bash
issueops link-child --id "$ISSUEOPS_ID" --child-url "$URL" --title "task title" --json
```

MCP adds `issueops_link_child` with the same meaning and response DTO as the CLI JSON output.

## Provider Boundary

This change does not call `gh api` or `glab api`. Remote creation/linking remains an agent workflow step governed by the Korean remote artifact gate and provider-specific hierarchy rules. When provider adapters are added later, they should consume the same state shape rather than inventing a host-specific graph model.

## Validation

- Core tests cover persistence, duplicate rejection, invalid URL rejection, provider inference, and JSON reload.
- CLI tests cover `issueops link-child`.
- MCP tests cover `issueops_link_child`.
- Usage/golden tests cover the new command/tool surface.
