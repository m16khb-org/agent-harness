---
name: issueops-prepare
description: Give an IssueOps cycle its branch identity. Resolve and pin the exact base SHA, record it with "issueops branch prepare", create and verify the provider-side branch link, and move the cycle to the plan phase. Creating the worktree is not this skill's job — "issueops execution prepare" owns it and picks git or Orca. Use when the user says "준비", "브랜치 만들어줘", "prepare the issue branch", or when "issueops next" reports stage prepare.
---

# IssueOps Prepare

이 스킬의 일은 **브랜치 정체성 하나를 봉인하는 것**이다. 어느 커밋에서 시작하는지,
어떤 이름의 브랜치인지, 그 브랜치가 이슈에 연결됐는지를 기록하고 계획 단계로 넘긴다.
워크트리는 만들지 않는다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 다음 단계: [`issueops-plan`](../issueops-plan/SKILL.md)
- lease와 워크트리 provisioning 계약: [`execution.md`](../issueops/references/execution.md)

## 이 스킬이 맞는지 확인

```bash
agent-harness issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `prepare`일 때 이 스킬이다. `issue`면 [`issueops-create-issue`](../issueops-create-issue/SKILL.md)가,
`plan.*`이면 [`issueops-plan`](../issueops-plan/SKILL.md)이 맞다. 다른 값이면 라우터로
돌아간다. `next_command`가 이미 다음 명령을 렌더하므로 그것을 먼저 읽는다.

## 안전 규칙

- **base는 움직이는 ref가 아니라 정확한 SHA로 봉인한다.** `origin/main`은 다음 fetch에
  움직인다. 이후 모든 단계가 이 SHA를 기준으로 판정하므로 이름이 아니라 값을 기록한다.
- **워크트리를 만들지 않는다.** 워크트리 provisioning은 `execution prepare`가 단독으로
  소유한다. direct 모드는 git으로 만들고, Orca 모드는 `orca worktree create --name
  <branch>`로 자기 브랜치와 워크트리를 함께 만든다. 여기서 로컬 브랜치나 워크트리를
  미리 만들면 Orca 경로가 이름 충돌로 깨진다. `git worktree add`를 실행하지 않고,
  `link-worktree`도 호출하지 않는다.
- **provider 링크는 외부 write다.** 무엇이 원격에 쓰이는지 말하고 사용자 확인을 받은 뒤
  첫 mutation을 실행한다. 확인을 받은 뒤에는 나머지 등록 단계를 다시 묻지 않는다.
- 이미 있는 로컬·원격 브랜치 이름을 재사용하지 않는다. 충돌이면 멈추고 보고한다.
  자리를 비우려고 지우지 않는다.
- source checkout은 시작할 때 깨끗해야 하고 끝까지 그대로여야 한다. 브랜치도 바뀌지
  않는다.
- force 연산, 기본 브랜치 변경, `gh issue develop`을 쓰지 않는다. `gh issue develop`은
  링크 시점의 base branch HEAD를 쓰므로 봉인한 SHA와 갈라진다.

## 절차

```bash
COMMON_GIT_DIR=$(git rev-parse --path-format=absolute --git-common-dir)
SOURCE_ROOT=$(dirname "$COMMON_GIT_DIR")
BRANCH="<issue-number>-<kebab-slug>"

git -C "$SOURCE_ROOT" fetch origin "$BASE_BRANCH"
BASE_SHA=$(git -C "$SOURCE_ROOT" rev-parse "origin/$BASE_BRANCH")

agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" \
  --issue-url "$ISSUE_URL" --branch "$BRANCH" --base-branch "$BASE_BRANCH" \
  --base-sha "$BASE_SHA" $RECORD_ACTOR_FLAGS --json
```

브랜치 이름은 `<issue-number>-<kebab-slug>` 형식이 강제된다. 번호가 앞에 오는 것은
규약이 아니라 요구다. GitLab은 `<iid>-` 접두 브랜치를 이슈에 자동으로 연결하고,
GitHub 경로도 같은 이름 규칙을 요구한다.

**provider 링크**(외부 write, 사용자 확인 뒤):

```bash
# GitHub — 봉인한 SHA에서 linked branch를 만든다.
ISSUE_ID=$(gh issue view <number> --json id -q .id)
gh api graphql -f query='
  mutation($issueId: ID!, $oid: GitObjectID!, $name: String!) {
    createLinkedBranch(input: {issueId: $issueId, oid: $oid, name: $name}) {
      linkedBranch { ref { name } }
    }
  }' -f issueId="$ISSUE_ID" -f oid="$BASE_SHA" -f name="$BRANCH"

# GitLab — 봉인한 SHA에서 원격 브랜치를 만든다. `<iid>-` 접두가 연결을 만든다.
glab api --method POST "projects/:id/repository/branches" \
  -f branch="$BRANCH" -f ref="$BASE_SHA"
```

```bash
agent-harness issueops branch prepare --id "$ISSUEOPS_ID" --provider "$PROVIDER" \
  --issue-url "$ISSUE_URL" --branch "$BRANCH" --base-branch "$BASE_BRANCH" \
  --base-sha "$BASE_SHA" --link-verified $RECORD_ACTOR_FLAGS --json

agent-harness issueops phase --id "$ISSUEOPS_ID" --to plan $RECORD_ACTOR_FLAGS --json
```

**GitHub + Orca 순서.** Orca가 준비된 환경의 GitHub 사이클은 원격 브랜치가 먼저 있으면
Orca prepare가 실패한다. 그래서 첫 `branch prepare`는 base SHA만 기록하고
`--link-verified`와 `createLinkedBranch`를 `execution prepare` 뒤로 미룬다. 그 순서는
[`execution.md`](../issueops/references/execution.md)의 "GitHub Orca Branch Ordering"이
소유한다. 어느 순서를 쓸지는 그 문서를 읽고 정한다.

**issue와 code가 다른 프로젝트에 있을 때.** `branch prepare`는 체크아웃의 `origin`을
관측해 이슈의 프로젝트와 다르면 `code_project_key`를 봉인한다. `origin`이 없거나 다른
곳을 가리키거나 값을 명시해야 하면 `--code-project-key HOST/GROUP/PROJECT`를 넘긴다.
이 값이 틀리면 `remote create-pr`, `remote verify-artifact`, `execution complete`가 모두
아티팩트를 거부한다.

**delegated child 사이클**(`issueops child start`로 만든 것)은 분기가 다르다. base
branch는 부모 사이클의 브랜치이고, base SHA는 부모 워크트리의 HEAD이며,
`--parent-worktree "${SOURCE_ROOT}.worktrees/<부모 브랜치>"`를 반드시 넘긴다. 그 값이
봉인되고 이후 대조된다. `origin/$BASE_BRANCH` fetch는 독립 사이클 전용이므로 child에서는
쓰지 않는다.

## 검증

- `agent-harness issueops status --id "$ISSUEOPS_ID" --json`의 `branch_prepare`에
  `provider`, `issue_url`, `branch`, `base_branch`, `base_sha`가 있고 `phase`가 `plan`이다.
- provider 링크가 실제로 보인다. GitHub은 이슈의 `linkedBranches` GraphQL 조회에 브랜치가
  있고, GitLab은 `glab api "projects/:id/repository/branches/<branch>"`가 원격 브랜치를
  돌려준다(`<iid>-` 접두가 관계를 보장한다).
- `git -C "$SOURCE_ROOT" status --short`가 비어 있고 브랜치가 바뀌지 않았다.
- **`git worktree list`에 새 항목이 없다.** 있으면 이 단계가 하지 말아야 할 일을 한 것이다.

## 출구

다음은 [`issueops-plan`](../issueops-plan/SKILL.md)이다. 계획은 워크트리가 아직 없으므로
source checkout 밖 임시 파일에 쓰고 `artifact stage --name plan`으로 올린다. 워크트리와
구현 세션은 3단계 끝의 `execution prepare --mode auto`가 만든다. 그 명령이 Orca 준비
여부를 probe해서 모드를 고르므로, 여기서 모드를 미리 정하지 않는다.

## 실패 처리

- **브랜치 이름 충돌**: 멈추고 충돌하는 ref를 나열한 뒤 재사용할지 이름을 바꿀지 묻는다.
- **`createLinkedBranch` 거부**(이름 선점, 권한 부족): API 오류 원문을 보고한다.
  `gh issue develop`으로 조용히 대체하지 않는다.
- **부분 완료**(record는 남고 provider 링크가 실패): 기록을 지우지 말고 어느 단계가
  남았는지 보고한다. 재시도는 멱등해야 하므로 만들기 전에 존재를 다시 확인한다.
- **base branch가 fetch되지 않음**: 원격 접근 문제이므로 추정한 SHA로 진행하지 않는다.
  봉인할 값을 관측하지 못하면 이 단계는 완료될 수 없다.
