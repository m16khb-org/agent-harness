# Provider and reviewer reference

`scripts/review_threads.py` wraps every call below. This file is the contract the
script implements, so the same steps can be reproduced with a raw CLI, a GitLab MCP
`glab_api` tool, or any other authenticated surface when the script cannot run
(wrong-profile token, host without Python, MCP-only sandbox).

## Review agents

| Reviewer | Where it appears | How to recognise the first note | Learns from | Mention in reply |
|---|---|---|---|---|
| Kody / Kodus | GitLab (project bot user), GitHub app | login `project_<id>_bot_*` or `kody*`; body `<!-- kody-codereview -->`, `kody-code-review`, `kody-pr-summary` | 👍/👎 award on its note **and** the `@kody` reply | `@kody` |
| CodeRabbit | GitHub `coderabbitai[bot]`, GitLab `coderabbitai` | login; body mentions `coderabbit` | replies addressed to `@coderabbitai` (learnings) | `@coderabbitai` |
| Copilot | GitHub `copilot-pull-request-reviewer[bot]` | login | nothing (no feedback loop) | none |
| Gemini Code Assist | GitHub/GitLab `gemini-code-assist[bot]` | login | `@gemini-code-assist` replies | `@gemini-code-assist` |
| Fagan (this harness) | any | body `<!-- fagan-review head=… -->`, `fagan-finding id=…` | 👎 + reason feeds `rule_candidates` in the next run | none |
| Unknown bot | any | GitHub `author.__typename == Bot` / login ends with `[bot]`; GitLab `author.bot == true` / `*_bot*` login | unknown | none |

Summary notes ("코드 리뷰 완료", CodeRabbit walkthrough, Copilot overview) are not
resolvable threads on GitHub and are `resolvable=false` discussions on GitLab. The
script reports them with `resolvable: false`; reply only when the user asks.

## Normalised thread shape (script output)

```json
{"thread_id": "…", "note_id": 123, "resolvable": true, "resolved": false, "outdated": false,
 "path": "src/a.ts", "line": 42, "author": "coderabbitai[bot]", "author_kind": "bot",
 "reviewer": "coderabbit", "mention": "@coderabbitai", "body": "…", "url": "…",
 "reply_count": 0, "my_reply_count": 0, "already_handled": null}
```

`thread_id` is the GraphQL node id (GitHub) or the discussion id (GitLab); `note_id`
is the REST id of the first note — reactions go on the note, replies and resolution
go on the thread. `already_handled` is the verdict of the newest reply by the current
user that carries the `<!-- review-agent-feedback thread=… verdict=… -->` marker.

## GitHub (`gh`)

Authentication: `gh auth status` (add `--hostname` for GHES). Current user:
`gh api user --jq .login`.

| Step | Call |
|---|---|
| List threads | GraphQL `repository(owner,name){pullRequest(number){headRefOid state reviewThreads(first:100,after){pageInfo{hasNextPage endCursor} nodes{id isResolved isOutdated path line originalLine viewerCanResolve comments(first:100){nodes{databaseId body url author{login __typename}}}}}}}` |
| Reply in thread | `POST repos/{owner}/{repo}/pulls/{n}/comments/{first_comment_databaseId}/replies` `{"body": …}` → reply `id` |
| Reaction | `POST repos/{owner}/{repo}/pulls/comments/{databaseId}/reactions` `{"content": "+1"}` or `"-1"` |
| Verify reaction | `GET repos/{owner}/{repo}/pulls/comments/{databaseId}/reactions` → entry with `user.login == me` |
| Resolve | GraphQL `mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{id isResolved}}}` (`unresolveReviewThread` to undo) |
| Verify | re-run the list query; the thread must show `isResolved: true` and the reply `databaseId` |

PR-conversation comments from a bot (`issues/{n}/comments`) are not review threads:
they cannot be resolved, and a reply is just another conversation comment. Handle
them only when the user points at them.

Pass JSON bodies with `--input file.json` (or `-F`/`-f` for single-line fields).
Multiline Markdown through `-f body=…` is safe on `gh`, but the script uses
`--input` on both CLIs for one code path.

## GitLab (`glab`)

Authentication: `glab auth status --hostname <host>`. Profile-scoped MCP servers
expose the same `glab api` under a project-bound token — when the CLI token does not
match the target host/project, run the calls below through that MCP `glab_api` tool
with identical paths (see the `gitlab-usecase` skill for profile rules). Current
user: `glab api user` → `username`. Project path is URL-encoded (`group%2Fproject`).

| Step | Call |
|---|---|
| MR head | `GET projects/{enc}/merge_requests/{iid}` → `sha`, `state` |
| List threads | `GET projects/{enc}/merge_requests/{iid}/discussions?per_page=100` (paginate). Skip notes with `system: true`; first non-system note carries `resolvable`, `resolved`, `position.new_path/new_line`, `author.username`, `author.bot` |
| Reply in thread | `POST projects/{enc}/merge_requests/{iid}/discussions/{discussion_id}/notes` `{"body": …}` → note `id` |
| Reaction | `POST projects/{enc}/merge_requests/{iid}/notes/{note_id}/award_emoji` `{"name": "thumbsup"}` or `"thumbsdown"` |
| Verify reaction | `GET …/notes/{note_id}/award_emoji` → entry with `user.username == me` |
| Resolve | `PUT projects/{enc}/merge_requests/{iid}/discussions/{discussion_id}` `{"resolved": true}` |
| Verify | `GET …/discussions/{discussion_id}` → `notes[0].resolved == true` and the reply note id present |

`glab api -f body=…` treats a multiline value as a file path in some versions; use
`--input file.json` (what the script does) or `--raw-field`.

## Verdict → action defaults

| Verdict | Meaning | Reaction | Resolve |
|---|---|---|---|
| `valid` | finding correct, fix applied or accepted | 👍 | yes |
| `partial` | risk real but premise/severity/fix wrong; a corrected fix was applied | 👍 (👎 when the suggested action would have misled) | yes |
| `invalid` | premise false against the actual code | 👎 | yes |
| `out_of_scope` | true but pre-existing / not this change's concern; tracked elsewhere | 👎 (the finding is not actionable here) | yes |
| `hold` | waiting on a human decision, uncommitted follow-up in this PR/MR, or user said leave it open | none | **no** — `reason_open` required |

Overrides go in the plan (`reaction`, `resolve`); the defaults only fill gaps.
