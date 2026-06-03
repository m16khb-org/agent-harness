# IssueOps Review And Feedback Rules

## Worker And Review Gates

Every implementation, TDD, review, QA, or subagent worker prompt must require the worker to begin by reporting and verifying:

- `pwd`
- `git branch --show-current`
- `git rev-parse --short HEAD`
- the expected isolated worktree path

If any value does not match the IssueOps branch/worktree contract, the worker must stop and report the mismatch instead of reviewing or editing.

For short or narrow reviews, prefer `verifier` or direct bounded review over `code-reviewer`. When `code-reviewer` is necessary, the prompt must set a clear time budget, forbid nested subagent fan-out, and require the reviewer to verify `pwd`, branch, `HEAD`, and worktree path before inspecting the diff.

Subagent reviews must be bounded work items, not broad unscoped diff sweeps. Before spawning a review worker, classify the review surface:

- `small_direct`: a few files and no large generated fixtures; review directly or with one verifier.
- `bounded_subagent`: specific files or areas; include a time budget, excluded paths, and concise expected output shape.
- `split_required`: golden files, generated fixtures, vendored data, response snapshots, or broad docs/code diffs; split into separate review prompts or verify generated files with tests instead of asking a subagent to read them.

Every review subagent prompt must name the expected worktree path, branch, and `HEAD`; forbid edits; list included paths or concerns; list excluded large/generated paths such as `cmd/harness/testdata/*.golden.*`; and state the fallback if the reviewer does not respond in time. If a review subagent does not return by the chosen wait budget, do not treat that as approval or failure. Record the timeout, continue with direct verification evidence, and either retry with a narrower prompt or close the subagent before final reporting.

## Remote Review Feedback

When creating or editing a PR/MR, assign it to the currently authenticated user before reporting that the PR/MR is ready. For GitHub, resolve the login with `gh api user --jq .login`, edit the PR assignee, and verify with `gh pr view "$PR_URL" --json assignees`. For GitLab, use the equivalent current-user assignee field and verify the assignee list.

When handling remote PR/MR review feedback, first verify each reviewer claim against the diff, code, and commands before changing files. Apply only confirmed fixes, then reply in the original review thread with the commit and verification evidence.

The remote issue is the source of truth for IssueOps scope. If user feedback, review feedback, QA, CI evidence, or agent analysis changes the problem statement, acceptance criteria, non-goals, verification, implementation scope, related issue links, or labels, update the issue body before continuing. A thread/comment may record discussion, but it is not enough; the issue body must match the implementation contract. Run the Korean Remote Artifact Gate before every remote issue body edit.

When the user asks only for review-validity verification, verify each remote review claim against the diff, code, and commands, then reply in the original review thread with the verdict before reporting back to the user. Each thread reply must say whether the review is `타당` or `타당하지 않음`, cite concrete evidence, and state the next action.

Use this thread reply shape:

```text
타당성: 타당

근거:
- <파일:라인 또는 명령 결과 근거>
- <계약/테스트 근거>

다음 조치: <수정 진행|별도 PR 분리|보류 사유>
```

After posting thread replies, report the evidence-based verdict and present numbered next actions:

```text
선택지:
1. 진행: 테스트를 먼저 추가하고 결함을 수정합니다. (추천)
2. 축소 진행: 일부 검증만 먼저 수정하고 나머지는 별도 PR로 분리합니다.
3. 보류: 현재 PR에는 수정하지 않고 리뷰 스레드에 검증 결과만 답변합니다.
```

## Provider Thread Handling

For GitHub inline review comments, reply to the original review comment:

```bash
gh api "repos/$OWNER/$REPO/pulls/$PR_NUMBER/comments"
gh api "repos/$OWNER/$REPO/pulls/comments/$COMMENT_ID/replies" -f body="$BODY"
```

For GitLab merge request discussions, reply to the original discussion thread:

```bash
glab api "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions"
glab api "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions/$DISCUSSION_ID/notes" -f body="$BODY"
```

Provider-specific thread discovery is mandatory before claiming there is no thread to reply to:

- GitHub: query both `gh pr view --json reviews,latestReviews` and `gh api "repos/$OWNER/$REPO/pulls/$PR_NUMBER/comments"`. Use the REST comment `id` for replies and GraphQL `reviewThreads` for resolution.
- GitLab: query both MR notes/reviews if needed and `glab api "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions"`. Use the discussion `id` for replies and resolution.

Replying to a review thread is not the same as resolving it. After a valid review is fixed, verified, committed, pushed, and answered with evidence, resolve the addressed conversation/discussion on the provider before reporting that review feedback is cleared.

For GitHub review thread resolution:

```bash
gh api graphql -f owner="$OWNER" -f repo="$REPO" -F number="$PR_NUMBER" -f query='query($owner:String!, $repo:String!, $number:Int!) { repository(owner:$owner, name:$repo) { pullRequest(number:$number) { reviewThreads(first:100) { nodes { id isResolved isOutdated path } } } } }'
gh api graphql -f threadId="$THREAD_ID" -f query='mutation($threadId:ID!) { resolveReviewThread(input:{threadId:$threadId}) { thread { id isResolved } } }'
```

For GitLab discussion resolution:

```bash
glab api "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions"
glab api --method PUT "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions/$DISCUSSION_ID" -f resolved=true
glab api "projects/$PROJECT_ID/merge_requests/$MR_IID/discussions/$DISCUSSION_ID"
```

Resolve only threads/discussions whose feedback has actually been addressed or is obsolete for a verified reason. Re-check review threads/discussions and PR/MR readiness before claiming merge blockage is cleared.
