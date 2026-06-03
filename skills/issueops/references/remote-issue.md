# IssueOps Remote Issue And Artifact Rules

## Remote Issue First

When the user explicitly invokes `$issueops` and the repo remote, credentials, target project, branch target, and issue ownership are discoverable, create the remote GitHub/GitLab issue before planning or implementation. Then immediately link it with `agent-harness issueops link-issue`.

Assign the created remote issue to the currently authenticated user. For GitHub, resolve the login with `gh api user --jq .login` and pass it to `gh issue create --assignee "$login"` or apply it immediately with `gh issue edit "$ISSUE_URL" --add-assignee "$login"` before linking the issue. For GitLab, use the equivalent current-user assignee field supported by the target project's CLI/API.

Before creating or editing the remote issue, proactively score related issues and labels. Gather candidate issues and labels from the target provider, build an `issueops remote score` request, and apply only selected candidates whose score is at or above the threshold. The count is not fixed; the threshold decides the set.

Provider candidate gathering:

- GitHub: use `gh issue list --state all --limit N --json number,title,body,labels,url,state` and `gh label list --json name,description,color`.
- GitLab: use the equivalent `glab issue list`/GitLab API issue fields and project label list.

Always inspect the current issue list and label list before deciding whether to create a new issue, link an existing issue, or create/update labels. If an existing issue already matches the requested work, link or update that issue instead of creating a duplicate. If the scoring gate selects a label that does not exist on the provider, create the missing label before issue creation or issue edit, then apply it. Do not create labels that were not selected by the scoring gate.

Run the scoring gate with the external LLM judge when available:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge agy --json
```

Use deterministic fallback only when the external LLM is unavailable or intentionally disabled:

```bash
agent-harness issueops remote score --input issueops-remote-score.json --judge none --json
```

Default threshold is `0.70` unless the repo or user sets a stronger threshold. Include selected related issue references/URLs in the issue body, include a compact scoring summary when it helps future reviewers understand why those links and labels were chosen, and apply selected labels with provider CLI/API commands. Do not apply rejected labels, create rejected labels, or link rejected issues.

The agent must propose the operational choice instead of leaving the user to invent it. Example:

```text
관련 이슈/라벨 후보를 점수화하고 threshold 이상만 이슈 본문과 라벨에 반영하겠습니다. 기본은 agy judge, 실패 시 deterministic fallback으로 진행합니다.
```

Only prepare a local issue draft instead of creating a remote issue when credentials, target provider, ownership, or branch target are unclear, or when the user explicitly asks not to create a remote issue.

If the agent realizes it implemented before creating or linking the issue, it must stop implementation, create or link the issue if possible, record corrective feedback in IssueOps state, and resume from the issue-linked plan.

## Issue Template

Use this structure unless the target project already has a stronger issue template:

```markdown
## Problem

## Current Evidence

## Acceptance Criteria

## Non-goals

## Verification

## Feedback Log
```

## Korean Remote Artifact Gate

IssueOps가 원격에 생성하거나 수정하는 issue와 PR/MR 제목·본문은 한글 중심이어야 한다. 명령어, 코드 식별자, 파일 경로, URL, upstream/project 이름은 영어 원문을 유지할 수 있다.

IssueOps cycle에서 `gh issue create`, `gh issue edit`, `gh pr create`, `gh pr edit` 또는 GitLab equivalent를 실행하기 전에는 매번 다음 gate를 통과해야 한다.

1. 제목과 본문을 임시 파일 또는 heredoc으로 준비한다.
2. bundled language gate를 실행한다.

```bash
python3 skills/issueops/scripts/remote_artifact_gate.py --kind issue --title "$TITLE" --body-file "$BODY_FILE"
python3 skills/issueops/scripts/remote_artifact_gate.py --kind pr --title "$TITLE" --body-file "$BODY_FILE"
```

3. gate가 실패하면 원격 artifact를 생성하거나 수정하지 말고 한글 중심으로 다시 작성한다.

이 gate는 issue/PR/MR에 영어 section label, command output, code identifier, URL, 외부 project 이름이 포함되어도 반드시 실행한다.

원격 issue 본문에는 repo-local plan path를 넣지 않는다. plan 파일은 ignored/untracked일 수 있으므로 `agent-harness issueops link-plan` state와 PR/MR 본문에서 필요한 경우에만 추적한다.
