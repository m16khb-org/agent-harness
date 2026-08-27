---
name: issueops-providers
description: IssueOps Orca preparation, provider branch linkage, and issue-snapshot contracts.
---

# IssueOps Provider Publication and Branch Contracts

This guide owns cycle preparation, provider branch linkage, and issue-snapshot
contracts for IssueOps Orca execution. The execution state machine, recovery,
and owner workflow live in
[issueops-execution.md](issueops-execution.md). Canonical index:
[../../OPERATIONS.md](../../OPERATIONS.md).

## Preparation and Provider Publication Contracts

### Durable parent issue creation and recovery

`create-issue --confirm` requires a started IssueOps record and a canonical
`origin` remote. The command stores a sealed intent before invoking `gh` or
`glab`, appends an operation marker to the issue body, performs live
label/assignee verification, then records the canonical issue URL and completed
intent atomically.

```bash
agent-harness issueops remote create-issue \
  --id ID --provider github --title "Title" --body "Body" \
  --label bug --assignee USER --confirm --json
```

If the provider process did not start, rerun the exact request; changed title,
body, labels, assignees, provider, or project authority are rejected. If the
process started but the result is unknown, do not rerun create. Search and
adopt through the durable marker:

```bash
agent-harness issueops remote reconcile-issue --id ID --json
agent-harness issueops remote reconcile-issue --id ID --confirm --json
```

Zero or multiple marker candidates remain blocked. One candidate is accepted
only when its title and full body digest match the sealed request and live
labels/assignees verify. `issueops list` shows
`[issue-create:<status>]`; `doctor` reports ambiguous, verification-failed, and
receipt-failed states. Normal short-lived `pending` intent is not unhealthy.

Orca is user-installed and optional. Preview
`agent-harness issueops execution prepare --id ID --mode auto ... --json` preview 후 반환된 `next_command`의 동일 입력과 `--expected-readiness-fingerprint`로 confirm한다. `--mode direct`는 예외 경로이며 정규화된 `--direct-reason` 없이는 거부된다. 완료 전 status의 `execution.selection`에서 requested/resolved mode, probe booleans/code, fallback, fingerprint, selected_at, explicit-direct reason을 다시 읽는다.
review the mode, branch, base SHA, canonical worktree, and owner model, then
repeat the identical request with `--confirm`. `auto` selects Orca only when
readiness succeeds before mutation; otherwise it selects direct. The only
first-party owner hosts are Codex and Claude.

Orca-capable prepare 전에 승인된 child plan을 source checkout 밖의 coordinator
임시 파일에 작성하고 `issueops artifact stage --id ID --name plan --file PATH --json`로
stage한다. 이 command에는 actor flag가 없다. 새 Orca prepare는 worktree receipt
직후 레코드의 `execution.workspace.artifact_dir` 아래에 권한이 `0600`인 artifact를
materialize한다. 이슈에 연결된 cycle의 canonical 경로는
`.agent-harness/issues/<provider-issue-number>/artifact/plan.md`이며,
`artifact_dir`가 비어 있는 legacy 레코드만
`.agent-harness/artifact/plan.md`를 fallback 경로로 사용한다. Prepare는 이 경로를
durable `plan_path`로 같은 CAS에 기록하고, 동일 digest를 sealed packet에 넣은
뒤에만 owner를 띄운다. `parent_plan_path`만으로는 readiness를 충족하지 못한다.

엄브렐라 자식 cycle은 `issueops branch prepare`에 canonical
`--parent-worktree`를 기록하고 preview의 `workspace.parent_worktree`가 부모
통합 worktree를 가리키는지 확인한다. 내부 delegation이 없는 provider-native
자식도 같은 계약을 사용한다. confirm은 Orca
`worktree create --parent-worktree path:<부모>`를 사용하고 생성 lineage의
`explicit-cli-flag`/`explicit` 영수증이 없으면 fail-closed한다. 독립 cycle은
기존처럼 top-level worktree로 생성된다.

GitLab-linked cycle은 host가 관찰한 `issue_snapshot` 또는 provider adapter의 일반
`glab api` read로 issue 본문을 봉인한 뒤 Orca를 사용할 수 있다. 특정 MCP server
이름이나 개인 wrapper를 하네스에 고정하지 않는다. host agent는 semantic leaf
`glab_api`와 실제 schema로 compatible tool을 찾고, 개인 wrapper도 같은
capability를 노출하면 사용할 수 있다. MCP snapshot이 있으면 source는
`glab_mcp`, 없으면 generic CLI fallback 결과는 `glab_cli`로 응답에 남는다.
`/issues/:iid`와 `/work_items/:iid`는 exact authority(명시 port 포함), project
path, IID가 모두 같을 때만 equivalent identity다.

**GitHub + Orca 모드는 브랜치 생성 순서를 뒤집는다.** IssueOps 정식 순서는 linked branch를
먼저 만들지만, Orca `worktree create`는 언제나 새 브랜치를 만들므로 이름이 겹쳐
`orca_branch_name_taken`으로 막힌다(#149·#152·#154).

`createLinkedBranch`는 `oid`에서 새 브랜치를 만들기 때문에 **원격에** 같은 이름이 있으면
실패하지만 **로컬에만 있으면 성공한다**(#163 실측). Orca는 로컬 워크트리와 로컬 브랜치만 만들고
push하지 않으므로 이 순서가 성립한다. 실제 값은 앞 단계 출력에서 그대로 옮겨 적는다 — 아래
꺾쇠 자리는 셸 변수가 아니다.

#176의 전체 계약은 `branch prepare` (base SHA only) → `artifact stage --name plan` → `execution prepare --mode orca` → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified` 순서다.

```bash
agent-harness issueops branch prepare --id ID --base-sha <BASE_HEAD> ...    # 기록만, 브랜치 없음
agent-harness issueops artifact stage --id ID --name plan --file <TEMP_PLAN_OUTSIDE_SOURCE> --json
agent-harness issueops execution prepare --id ID --mode orca ... --confirm  # Orca가 로컬 브랜치 생성
gh api repos/<OWNER>/<REPO>/issues/<NUMBER> --jq .node_id                   # 다음 단계의 issueId
gh api graphql -f 'query=mutation($issueId:ID!,$oid:GitObjectID!,$name:String!){createLinkedBranch(input:{issueId:$issueId,oid:$oid,name:$name}){linkedBranch{ref{name target{oid}}}}}' -F issueId=<NODE_ID> -F oid=<BASE_HEAD> -F name=<BRANCH>
agent-harness issueops branch prepare --id ID ... --link-verified           # 추적 확인 후 갱신
```

`branch prepare`는 브랜치 존재를 검증하지 않고 이름 규칙과 레코드 일치만 보므로 1단계가
성립한다. 이 순서로 **Orca 모드와 이슈-브랜치 추적을 둘 다 얻는다.**

**`gh issue develop`을 쓰지 않는 이유는 base가 갈리기 때문이다(#176).** 그 CLI의 `--base`는
브랜치 이름만 받고 GitHub이 **그 시점** 브랜치 HEAD를 `oid`로 쓴다. Orca는 봉인된 base SHA에서
로컬 브랜치를 만들므로, 그 사이 base 브랜치가 진행하면 push가 `non-fast-forward`로 거부되고
봉인 가드가 `merge`를, 안전 훅이 force push를, `sync-base`가 completion 이전 실행을 막아 발행
경로가 사라진다. `oid`는 `createLinkedBranch`의 필수 필드이므로 봉인 SHA를 직접 넘기면 그
갈림이 원리적으로 생기지 않는다.

두 가지 형태 제약이 있다:

- **GraphQL 변수는 단일 인용해야 한다.** `$issueId` 같은 토큰이 인용 밖에 있으면 lifecycle
  가드가 셸 파라미터 확장으로 판정해 명령을 거부한다. 같은 이유로 `oid`·`name`·`issueId` 값도
  셸 변수로 넘기지 않고 리터럴로 적는다.
- **node ID 조회는 별도 단계다.** 한 명령에 `$(...)`를 넣으면 가드가 정적으로 분류할 수 없다.
  출력값을 그대로 옮겨 적는다.

`branch prepare`가 이 두 명령을 실행 가능한 형태로 `Steps`에 렌더하므로 `--base-sha`를 준
사이클은 그 출력을 그대로 따르면 된다. 문서와 `Steps`가 어긋나면 `Steps`가 원본이다.

GitLab은 브랜치 이름 규칙이 연결 수단이라 GitHub linked-branch 순서 문제는 없다.
그래도 base 못박기는 같은 계약이다. GitLab의 `ref`는 commit SHA를 받으므로
`--base-sha`를 주면 `Steps`가 그 값을 `ref`로 안내한다(`#180`).

GitLab issue snapshot은 다음 순서로 준비한다.

1. canonical worktree의 선택 문서 `.agent-harness/VCS.md`를 읽는다.
2. 현재 host의 trusted tool 중 semantic leaf `glab_api`와 실제 input schema가
   맞는 후보를 찾는다. server namespace는 capability identity가 아니다.
3. `projects/<URL-escaped-project>/issues/<iid>`를 읽고, schema가 지원하면
   `flags.hostname`으로 target host를 명시한다. 응답의 `web_url`,
   `description`, `state`를 exact linked identity와 대조한다.
4. MCP 호출이면 `issueops_execution.issue_snapshot`에 다섯 필드
   (`provider=gitlab`, `source=glab_mcp`, `web_url`, `body`, `state`)만 넘긴다.
   IssueOps CLI 호출이면 같은 객체를 mode `0600` private file에 쓰고
   `--issue-snapshot-file`로 넘긴다.
5. 후보 부재나 auth/permission/transport/schema 호출 실패 뒤에도 successful
   exact-identity MCP evidence를 얻지 못했을 때만 snapshot을 생략한다. 이 경우
   provider adapter가 일반 `glab api`를 사용한다. 이미 공급한 invalid evidence는
   CLI fallback하지 않고 fail-closed한다.

성공한 portable recipe는 canonical worktree에서 `project_docs_read` 후
`project_docs_revise` SHA-CAS로 `.agent-harness/VCS.md`에 기록한다. GitLab과
GitHub가 함께 들어갈 수 있는 provider-neutral 문서이며, GitHub CLI recipe는
검증된 `gh issue view <url> --json url,body,state`를 사용한다. 실제로 관찰하지
않은 MCP 이름, 개인 wrapper 경로, token/profile/server namespace는 기록하지
않고 OpenWiki 자동 update도 실행하지 않는다.
